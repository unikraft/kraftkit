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

package paraprogress

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"go.unikraft.io/kit/pkg/log"
	"golang.org/x/term"
)

var tprog *tea.Program

type ParaProgress struct {
	processes     []*Process
	quitting      bool
	width         int
	parallel      bool
	log           log.Logger
	norender      bool
	maxConcurrent int
}

func NewParaProgress(processes []*Process, opts ...ParaProgressOption) (*ParaProgress, error) {
	if len(processes) == 0 {
		return nil, fmt.Errorf("no processes to perform")
	}

	md := &ParaProgress{
		processes: processes,
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

		if md.parallel {
			md.processes[i].log = md.log.Clone()
			md.processes[i].log.SetOutput(md.processes[i])
		} else {
			md.processes[i].log = md.log
		}
	}

	return md, nil
}

func (pd *ParaProgress) Start() error {
	teaOpts := []tea.ProgramOption{
		// tea.WithAltScreen(),
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

	return tprog.Start()
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

func (md ParaProgress) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	// tea.KeyMsg is sent whenever there is a keyboard interrupt
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			md.quitting = true
			return md, tea.Quit
		}
	}

	var cmd tea.Cmd
	complete := 0
	for i := range md.processes {
		md.processes[i], cmd = md.processes[i].Update(msg)
		cmds = append(cmds, cmd)

		if md.processes[i].Status == StatusFailed ||
			md.processes[i].Status == StatusSuccess {
			complete += 1
		}
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
