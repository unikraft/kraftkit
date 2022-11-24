// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package paraprogress

import (
	"context"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
	"kraftkit.sh/log"
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
	}

	for _, opt := range opts {
		if err := opt(md); err != nil {
			return nil, err
		}
	}

	maxNameLen := len(processes[0].Name)

	for _, download := range processes {
		if nameLen := len(download.Name); nameLen > maxNameLen {
			maxNameLen = nameLen
		}
	}

	for i := range processes {
		processes[i].NameWidth = maxNameLen
		logger := log.G(ctx)
		logger.Logger.Out = md.processes[i]
		md.processes[i].ctx = log.WithLogger(ctx, logger)
	}

	return md, nil
}

func (pd *ParaProgress) Start() error {
	teaOpts := []tea.ProgramOption{}

	if pd.norender {
		teaOpts = append(teaOpts, tea.WithoutRenderer())
	} else {
		// Set this super early (even before bubbletea), as fast exiting processes
		// may not have received the window size update and therefore pd.width is
		// set to zero.
		pd.width, pd.maxConcurrent, _ = term.GetSize(int(os.Stdout.Fd()))
	}

	tprog = tea.NewProgram(pd, teaOpts...)

	go func() {
		err := tprog.Start()
		if err == nil {
			pd.errChan <- pd.err
		} else {
			pd.errChan <- err
		}
	}()

	err := <-pd.errChan
	return err
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
			md.curr += 1
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
			complete += 1
		}
	}

	// Update each process to have the same width
	for i := range md.processes {
		md.processes[i].timerMax = md.timerWidth
	}

	if complete == len(md.processes) {
		md.quitting = true
		return md, tea.Sequentially(tea.Batch(cmds...), tea.Quit)
	}

	return md, tea.Batch(cmds...)
}

func (md ParaProgress) View() string {
	var content []string

	for _, process := range md.processes {
		content = append(content, process.View())
	}

	if md.quitting {
		content = append(content, "")
	} else {
		content = append(content, "ctrl+c to cancel")
	}

	return lipgloss.JoinVertical(lipgloss.Left, content...)
}
