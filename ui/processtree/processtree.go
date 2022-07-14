// SPDX-License-Identifier: BSD-3-Clause
//
// Authors: Alexander Jung <alex@unikraft.io>
//
// Copyright (c) 2022, Unikraft GmbH. All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions
// are met:
//
// 1. Redistributions of source code must retain the above copyright
//    notice, this list of conditions and the following disclaimer.
// 2. Redistributions in binary form must reproduce the above copyright
//    notice, this list of conditions and the following disclaimer in the
//    documentation and/or other materials provided with the distribution.
// 3. Neither the name of the copyright holder nor the names of its
//    contributors may be used to endorse or promote products derived from
//    this software without specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
// AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
// IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
// ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE
// LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR
// CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF
// SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS
// INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN
// CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE)
// ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE
// POSSIBILITY OF SUCH DAMAGE.

package processtree

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/stopwatch"
	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"

	"go.unikraft.io/kit/pkg/log"
)

type (
	SpinnerProcess func(l log.Logger) error
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
	log       log.Logger
}

type ProcessTree struct {
	verb     string
	channel  chan *ProcessTreeItem
	tree     []*ProcessTreeItem
	quitting bool
	log      log.Logger
	timer    stopwatch.Model
	width    int
	rightPad int
	parallel bool
	norender bool
	finished int
	total    int
}

func NewProcessTree(opts []ProcessTreeOption, tree ...*ProcessTreeItem) (*ProcessTree, error) {
	total := 0

	TraverseTreeAndCall(tree, func(item *ProcessTreeItem) error {
		total += 1
		return nil
	})

	pt := &ProcessTree{
		tree:     tree,
		timer:    stopwatch.NewWithInterval(time.Millisecond * 100),
		channel:  make(chan *ProcessTreeItem),
		total:    total,
		finished: 0,
	}

	for _, opt := range opts {
		if err := opt(pt); err != nil {
			return nil, err
		}
	}

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

func (pt ProcessTree) Start() error {
	var teaOpts []tea.ProgramOption

	if pt.norender {
		teaOpts = []tea.ProgramOption{tea.WithoutRenderer()}
	} else {
		// Set this super early (even before bubbletea), as fast exiting processes
		// may not have received the window size update and therefore pt.width is
		// set to zero.
		pt.width, _, _ = term.GetSize(int(os.Stdout.Fd()))
	}

	return tea.NewProgram(pt, teaOpts...).Start()
}

func (pt ProcessTree) Init() tea.Cmd {
	cmds := []tea.Cmd{
		waitForProcessExit(pt.channel),
		spinner.Tick,
		pt.timer.Init(),
	}

	// Initialize all timers
	TraverseTreeAndCall(pt.tree, func(pti *ProcessTreeItem) error {
		pti.log = pt.log

		// Clone the logger for this process if we are in fancy render mode
		if !pt.norender {
			pti.log = pt.log.Clone()
			pti.log.SetOutput(pti)
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
					failed += 1
				} else if child.status == StatusSuccess {
					completed += 1
				}
			}
		}

		// Only start the parent process if all children have succeeded or if there
		// no children and the status is pending
		if len(subprocesses) == 0 &&
			failed == 0 &&
			completed == len(item.children) &&
			(item.status == StatusPending || item.status == StatusRunningChild) {
			items = append(items, item)
		}
	}

	return items
}

func TraverseTreeAndCall(items []*ProcessTreeItem, callback func(*ProcessTreeItem) error) error {
	for _, item := range items {
		item := item

		if len(item.children) > 0 {
			if err := TraverseTreeAndCall(item.children, callback); err != nil {
				return err
			}
		}

		// Call the callback on the leaf node first
		if err := callback(item); err != nil {
			return err
		}
	}

	return nil
}

func formatTimer(dur time.Duration) string {
	ns := dur.Nanoseconds()
	ms := ns / 1000000
	sec := ms / 1000
	min := sec / 60
	hr := min / 60

	// Get only the excess amt of each component
	ns %= 1000000
	ms %= 1000
	sec %= 60
	hr %= 60

	// Express ns as ms to 3 significant digits
	ns /= 1000

	// Express ms to 1 significant digit
	ms /= 100

	if hr >= 1 {
		return fmt.Sprintf("%dh %2dm %2ds", hr, min, sec)
	} else if min >= 10 {
		return fmt.Sprintf("%2dm %2ds", min, sec)
	} else if min >= 1 && sec < 10 {
		return fmt.Sprintf("%dm %ds", min, sec)
	} else if min >= 1 {
		return fmt.Sprintf("%dm %2ds", min, sec)
	}

	return fmt.Sprintf("%d.%ds", sec, ms)
}

func (pt *ProcessTree) waitForProcessCmd(item *ProcessTreeItem) tea.Cmd {
	return func() tea.Msg {
		// Start the process
		item.status = StatusRunning

		if err := item.process(item.log); err != nil {
			item.log.Error(err)
			item.status = StatusFailed
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
