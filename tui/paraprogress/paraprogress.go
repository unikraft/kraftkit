// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package paraprogress

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/LastPossum/kamino"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"golang.org/x/term"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"
	"kraftkit.sh/tui"
)

var tprog *tea.Program

type ParaProgress struct {
	processes     []*Process
	quitting      bool
	width         int
	timerWidth    int
	parallel      bool
	ctx           context.Context
	norender      bool
	maxConcurrent int
	curr          int
	err           error
	errChan       chan error
	failFast      bool
	nameWidth     int
	timeout       time.Duration
	oldOut        iostreams.FileWriter
}

func NewParaProgress(ctx context.Context, processes []*Process, opts ...ParaProgressOption) (*ParaProgress, error) {
	if len(processes) == 0 {
		return nil, fmt.Errorf("no processes to perform")
	}

	md := &ParaProgress{
		processes: processes,
		errChan:   make(chan error),
		curr:      0,
		ctx:       ctx,
		// -1 represents the default mode where ecah individual
		// process's name's width is checked and the maximum of all
		// is used.
		nameWidth: -1,
		oldOut:    iostreams.G(ctx).Out,
	}

	for _, opt := range opts {
		if err := opt(md); err != nil {
			return nil, err
		}
	}

	maxNameLen := md.nameWidth
	if maxNameLen <= 0 {
		for _, process := range processes {
			if nameLen := len(process.Name); nameLen > maxNameLen {
				maxNameLen = nameLen
			}
		}
	}

	for _, process := range processes {
		process.norender = md.norender
		process.NameWidth = maxNameLen
		process.timeout = md.timeout

		if md.norender {
			process.ctx = ctx
		} else {
			pctx := ctx

			logger, err := kamino.Clone(log.G(pctx),
				kamino.WithZeroUnexported(),
			)
			if err != nil {
				return nil, err
			}

			// Update formatter when using KraftKit's TextFormatter.  The
			// TextFormatter recognises that this is a non-standard terminal and
			// changes the output to a more machine readable format.  Instead we want
			// to force the formatting so that the output looks seamless with the
			// style of the TUI.
			if formatter, ok := logger.Formatter.(*log.TextFormatter); ok {
				formatter.ForceColors = termenv.DefaultOutput().ColorProfile() != termenv.Ascii
				formatter.ForceFormatting = true
				logger.Formatter = formatter
			}

			logger.Out = process
			pctx = log.WithLogger(pctx, logger)

			ios, err := kamino.Clone(iostreams.G(pctx),
				kamino.WithZeroUnexported(),
			)
			if err != nil {
				return nil, err
			}

			ios.Out = iostreams.NewNoTTYWriter(process, iostreams.G(ctx).Out.Fd())
			ios.ErrOut = process
			ios.In = iostreams.G(ctx).In

			pctx = iostreams.WithIOStreams(pctx, ios)

			process.ctx = pctx
		}
	}

	return md, nil
}

func (pd *ParaProgress) Start() error {
	teaOpts := []tea.ProgramOption{
		tea.WithContext(pd.ctx),
	}

	if iostreams.G(pd.ctx).IsStdinTTY() {
		teaOpts = append(teaOpts, tea.WithInput(iostreams.G(pd.ctx).In))
	} else {
		teaOpts = append(teaOpts, tea.WithInput(nil))
	}

	if pd.norender {
		teaOpts = append(teaOpts, tea.WithoutRenderer())
	} else {
		// Set this super early (even before bubbletea), as fast exiting processes
		// may not have received the window size update and therefore pd.width is
		// set to zero.
		pd.width, pd.maxConcurrent, _ = term.GetSize(int(os.Stdout.Fd()))
	}

	tprog = tea.NewProgram(pd, teaOpts...)

	// Restore the old output for the IOStreams which is manipulated per process.
	defer func() {
		iostreams.G(pd.ctx).Out = pd.oldOut
		log.G(pd.ctx).Out = pd.oldOut
	}()

	if _, err := tprog.Run(); err != nil {
		return err
	}

	return pd.err
}

func (md ParaProgress) Init() tea.Cmd {
	var cmds []tea.Cmd

	for _, process := range md.processes {
		cmds = append(cmds, process.Init())

		if md.parallel {
			cmds = append(cmds, process.Start())
		}
	}

	// Start the first download if not in parallel mode
	if !md.parallel {
		cmds = append(cmds, md.processes[0].Start())
	}

	return tea.Batch(cmds...)
}

func (md *ParaProgress) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	// tea.KeyMsg is sent whenever there is a keyboard interrupt
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			md.quitting = true
			md.err = fmt.Errorf("force quit")
			return md, tea.Quit
		}
	case StatusMsg:
		if msg.err != nil {
			md.err = msg.err
		}
		if msg.err != nil && md.failFast {
			md.quitting = true
			cmds = append(cmds, tea.Quit)
		} else if (!md.failFast || msg.status == StatusSuccess) && !md.parallel {
			md.curr++
			if len(md.processes) > md.curr {
				cmds = append(cmds, md.processes[md.curr].Start())
			}
		}
	}

	var cmd tea.Cmd
	complete := 0
	for i := range md.processes {
		md.processes[i], cmd = md.processes[i].Update(msg)
		cmds = append(cmds, cmd)

		if md.processes[i].timerWidth > md.timerWidth {
			md.timerWidth = md.processes[i].timerWidth
		}

		if md.processes[i].Status == StatusFailed ||
			md.processes[i].Status == StatusSuccess ||
			md.processes[i].percent == 1 {
			complete++
		}
	}

	// Update each process to have the same width
	for i := range md.processes {
		md.processes[i].timerMax = md.timerWidth
	}

	if complete == len(md.processes) {
		md.quitting = true
		batch := tea.Batch(cmds...)
		if batch == nil {
			return md, tea.Quit
		}
		return md, tea.Sequence(batch, tea.Quit)
	}

	return md, tea.Batch(cmds...)
}

func (md ParaProgress) View() string {
	if md.norender {
		return ""
	}

	var content []string

	for _, process := range md.processes {
		content = append(content, process.View())
	}

	if md.quitting {
		content = append(content, "")
	} else {
		content = append(content, tui.TextLightGray("ctrl+c to cancel"))
	}

	return lipgloss.JoinVertical(lipgloss.Left, content...)
}
