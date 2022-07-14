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
	"strings"
	"sync"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/indent"

	"go.unikraft.io/kit/pkg/log"
)

var (
	lastID int
	idMtx  sync.Mutex
)

func nextID() int {
	idMtx.Lock()
	defer idMtx.Unlock()
	lastID++
	return lastID
}

type ProcessStatus uint

const (
	StatusPending ProcessStatus = iota
	StatusRunning
	StatusFailed
	StatusSuccess
)

const (
	INDENTS = 4
	LOGLEN  = 5
)

// StatusMsg is sent when the stopwatch should start or stop.
type StatusMsg struct {
	ID     int
	status ProcessStatus
	err    error
}

// ProgressMsg is sent when an update in the progress percentage occurs.
type ProgressMsg struct {
	ID       int
	progress float64
}

// Process ...
type Process struct {
	id          int
	percent     float64
	processFunc func(log.Logger, func(float64)) error
	log         log.Logger
	progress    progress.Model
	spinner     spinner.Model
	logs        []string
	err         error

	Name      string
	NameWidth int
	Status    ProcessStatus
}

func NewProcess(name string, processFunc func(log.Logger, func(float64)) error) *Process {
	d := &Process{
		id:          nextID(),
		Name:        name,
		spinner:     spinner.New(),
		progress:    progress.New(),
		Status:      StatusPending,
		NameWidth:   len(name),
		processFunc: processFunc,
	}

	d.progress.Full = '#'
	d.progress.Empty = ' '
	d.progress.ShowPercentage = true
	d.progress.PercentFormat = " %3.0f%%"

	return d
}

func (p Process) Init() tea.Cmd {
	return nil
}

func (p Process) Start() tea.Cmd {
	cmds := []tea.Cmd{
		spinner.Tick,
		func() tea.Msg {
			return StatusMsg{
				ID:     p.id,
				status: StatusRunning,
			}
		},
	}

	cmds = append(cmds, func() tea.Msg {
		err := p.processFunc(p.log, p.onProgress)
		status := StatusSuccess
		if err != nil {
			status = StatusFailed
		}

		if tprog != nil {
			tprog.Send(StatusMsg{
				ID:     p.id,
				status: status,
				err:    err,
			})
		}

		return nil
	})

	return tea.Batch(cmds...)
}

// onProgress is called to dynamically inject ProgressMsg into the bubbletea
// runtime
func (p Process) onProgress(progress float64) {
	if tprog == nil || progress < 0 {
		return
	}

	tprog.Send(ProgressMsg{
		ID:       p.id,
		progress: progress,
	})
}

// Write implements `io.Writer` so we can correctly direct the output from the
// process to an inline fancy logger
func (p *Process) Write(b []byte) (int, error) {
	// Remove the last line which is usually appended by a logger
	line := strings.TrimSuffix(string(b), "\n")

	// Split all lines up so we can individually append them
	lines := strings.Split(strings.ReplaceAll(line, "\r\n", "\n"), "\n")

	p.logs = append(p.logs, lines...)

	return len(b), nil
}

func (d *Process) Update(msg tea.Msg) (*Process, tea.Cmd) {
	switch msg := msg.(type) {
	// ProgressMsg is sent when the progress bar wishes
	case ProgressMsg:
		var cmd tea.Cmd
		if msg.ID != d.id {
			return d, nil
		}

		if msg.progress > 1.0 {
			msg.progress = 1.0
		}

		d.percent = msg.progress

		return d, cmd

	// TickMsg is sent when the spinner wants to animate itself
	case spinner.TickMsg:
		spinnerModel, cmd := d.spinner.Update(msg)
		d.spinner = spinnerModel
		return d, cmd

	// StatusMsg is sent when the status of the process changes
	case StatusMsg:
		if msg.ID != d.id {
			return d, nil
		}

		d.Status = msg.status
		if d.Status == StatusFailed {
			d.err = msg.err
		} else if d.Status == StatusSuccess {
			d.percent = 1.0
		}

		return d, nil

	// tea.WindowSizeMsg is sent when the terminal window is resized
	case tea.WindowSizeMsg:
		d.progress.Width = msg.Width - d.NameWidth - 5
		return d, nil

	default:
		return d, nil
	}
}

func (p Process) View() string {
	s := "["

	switch p.Status {
	case StatusFailed:
		s += "-"
	case StatusRunning:
		s += p.spinner.View()
	case StatusSuccess:
		s += "+"
	default:
		s += " "
	}

	s += "] "

	if p.err != nil {
		s += fmt.Sprintf(
			"error %s: %s",
			p.Name,
			p.err.Error(),
		)
	} else {
		s += lipgloss.NewStyle().
			Width(p.NameWidth + 1).
			Render(p.Name)

		s += p.progress.ViewAs(p.percent)
	}

	// Print the logs for this item
	if p.Status != StatusSuccess {
		// Newline for the logs
		if len(p.logs) > 0 {
			s += "\n"
		}

		truncate := 0
		loglen := len(p.logs) - LOGLEN

		if loglen > 0 {
			truncate = loglen
		}

		for _, line := range p.logs[truncate:] {
			s += indent.String(line, INDENTS) + "\n"
		}
	}

	return s
}
