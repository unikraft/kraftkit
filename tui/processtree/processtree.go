// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package processtree

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/stopwatch"
	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"

	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"
)

type (
	SpinnerProcess func(ctx context.Context) error
	processExitMsg *ProcessTreeItem
)

type SpinnerProcessStatus uint

const (
	StatusPending SpinnerProcessStatus = iota
	StatusRunning
	StatusRunningChild
	StatusRunningButAChildHasFailed
	StatusFailed
	StatusFailedChild
	StatusSuccess
)

const (
	INDENTS = 4
	LOGLEN  = 5
)

var tprog *tea.Program

type ProcessTreeItem struct {
	textLeft  string
	textRight string
	status    SpinnerProcessStatus
	spinner   spinner.Model
	children  []*ProcessTreeItem
	logs      []string
	logChan   chan *ProcessTreeItem
	process   SpinnerProcess
	timer     stopwatch.Model
	norender  bool
	ctx       context.Context
}

type ProcessTree struct {
	verb     string
	channel  chan *ProcessTreeItem
	tree     []*ProcessTreeItem
	quitting bool
	ctx      context.Context
	timer    stopwatch.Model
	width    int
	rightPad int
	parallel bool
	norender bool
	finished int
	total    int
	err      error
	errChan  chan error
	failFast bool
}

func NewProcessTree(ctx context.Context, opts []ProcessTreeOption, tree ...*ProcessTreeItem) (*ProcessTree, error) {
	pt := &ProcessTree{
		tree:     tree,
		ctx:      ctx,
		timer:    stopwatch.NewWithInterval(time.Millisecond * 100),
		channel:  make(chan *ProcessTreeItem),
		errChan:  make(chan error),
		finished: 0,
	}

	for _, opt := range opts {
		if err := opt(pt); err != nil {
			return nil, err
		}
	}

	total := 0

	pt.traverseTreeAndCall(tree, func(item *ProcessTreeItem) error {
		total++
		item.norender = pt.norender
		return nil
	})

	pt.total = total

	return pt, nil
}

func NewProcessTreeItem(textLeft, textRight string, process SpinnerProcess, children ...*ProcessTreeItem) *ProcessTreeItem {
	return &ProcessTreeItem{
		textLeft:  textLeft,
		textRight: textRight,
		process:   process,
		status:    StatusPending,
		children:  children,
		timer:     stopwatch.NewWithInterval(time.Millisecond * 100),
		logChan:   make(chan *ProcessTreeItem),
		spinner:   spinner.New(),
	}
}

// Write implements `io.Writer` so we can correctly direct the output from tree
// process to an inline fancy logger
func (pti *ProcessTreeItem) Write(p []byte) (int, error) {
	// Remove the last line which is usually appended by a logger
	line := strings.TrimSuffix(string(p), "\n")

	// Split all lines up so we can individually append them
	lines := strings.Split(strings.ReplaceAll(line, "\r\n", "\n"), "\n")

	pti.logs = append(pti.logs, lines...)

	return len(p), nil
}

func (pt *ProcessTree) Start() error {
	var teaOpts []tea.ProgramOption

	if pt.norender {
		teaOpts = []tea.ProgramOption{tea.WithoutRenderer()}
	} else {
		// Set this super early (even before bubbletea), as fast exiting processes
		// may not have received the window size update and therefore pt.width is
		// set to zero.
		pt.width, _, _ = term.GetSize(int(os.Stdout.Fd()))
	}

	tprog = tea.NewProgram(pt, teaOpts...)

	go func() {
		err := tprog.Start()
		if err == nil {
			pt.errChan <- pt.err
		} else {
			pt.errChan <- err
		}
	}()

	err := <-pt.errChan
	return err
}

func (pt *ProcessTree) Init() tea.Cmd {
	cmds := []tea.Cmd{
		waitForProcessExit(pt.channel),
		spinner.Tick,
		pt.timer.Init(),
	}

	// Initialize all timers
	pt.traverseTreeAndCall(pt.tree, func(pti *ProcessTreeItem) error {
		pti.ctx = pt.ctx

		// Clone the logger for this process if we are in fancy render mode
		if !pt.norender {
			logger := log.G(pti.ctx)
			logger.Logger.Out = pti
			pti.ctx = log.WithLogger(pt.ctx, logger)
		}

		return nil
	})

	// Start all child processes
	children := pt.getNextReadyChildren(pt.tree)
	for _, pti := range children {
		pti := pti
		cmds = append(cmds, pt.waitForProcessCmd(pti))
		cmds = append(cmds, pti.timer.Init())
	}

	return tea.Batch(cmds...)
}

func (pt ProcessTree) getNextReadyChildren(tree []*ProcessTreeItem) []*ProcessTreeItem {
	var items []*ProcessTreeItem

	for _, item := range tree {
		var subprocesses []*ProcessTreeItem
		completed := 0
		failed := 0

		if len(item.children) > 0 {
			subprocesses = pt.getNextReadyChildren(item.children)

			// Add all subprocesses if in parallel mode
			if pt.parallel {
				items = append(items, subprocesses...)

				// We can only add 1 item if non-parallel and there are actual
			} else if len(subprocesses) > 0 {
				items = append(items, subprocesses[0])
			}

			// Determine the status of immediate children
			for _, child := range item.children {
				if child.status == StatusFailed ||
					child.status == StatusFailedChild {
					failed++
				} else if child.status == StatusSuccess {
					completed++
				}
			}
		}

		// Only start the parent process if all children have succeeded or if there
		// no children and the status is pending
		if len(subprocesses) == 0 &&
			failed == 0 &&
			(pt.parallel || (len(items) == 0 && !pt.parallel)) &&
			completed == len(item.children) &&
			(item.status == StatusPending || item.status == StatusRunningChild) {
			items = append(items, item)
		}
	}

	return items
}

func (pt *ProcessTree) traverseTreeAndCall(items []*ProcessTreeItem, callback func(*ProcessTreeItem) error) error {
	for i, item := range items {
		item := item

		if len(item.children) > 0 {
			if err := pt.traverseTreeAndCall(item.children, callback); err != nil {
				return err
			}
		}

		// Call the callback on the leaf node first
		if err := callback(item); err != nil {
			return err
		}

		items[i] = item
	}

	return nil
}

func (pt *ProcessTree) waitForProcessCmd(item *ProcessTreeItem) tea.Cmd {
	return func() tea.Msg {
		item := item // golang closures

		// Clone the context to be used individually by each process
		ctx := pt.ctx

		if pt.norender {
			log.G(ctx).Info(item.textLeft)
		} else {
			// Set the output to the process Writer such that we can hijack logs and
			// print them in a per-process isolated view.
			entry := log.G(ctx).Dup()
			logger := *entry.Logger //nolint:govet
			logger.SetOutput(item)
			entry.Logger = &logger
			ctx = log.WithLogger(ctx, entry)

			// Set the output of the iostreams to the per-process isolated view.
			io := *iostreams.G(ctx)
			io.Out = item
			io.SetStdoutTTY(false)
			io.SetStderrTTY(false)
			io.SetStdinTTY(false)
			ctx = iostreams.WithIOStreams(ctx, &io)
		}

		// Set the new context for the individual process.
		item.ctx = ctx

		// Set the process to running
		item.status = StatusRunning

		if err := item.process(item.ctx); err != nil {
			log.G(item.ctx).Error(err)
			item.status = StatusFailed
			pt.err = err
			if pt.failFast {
				pt.quitting = true
			}
		} else {
			item.status = StatusSuccess
		}

		pt.channel <- item

		return item.timer.Stop()
	}
}

func waitForProcessExit(sub chan *ProcessTreeItem) tea.Cmd {
	return func() tea.Msg {
		return processExitMsg(<-sub)
	}
}
