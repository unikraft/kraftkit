// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package paraprogress

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/stopwatch"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/indent"

	"kraftkit.sh/log"
	"kraftkit.sh/tui"
	"kraftkit.sh/utils"
)

var (
	lastID int
	idMtx  sync.Mutex
	width  = lipgloss.Width
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
	processFunc func(context.Context, func(float64)) error
	progress    progress.Model
	spinner     spinner.Model
	timer       stopwatch.Model
	timerWidth  int
	timerMax    int
	width       int
	logs        []string
	err         error
	norender    bool
	ctx         context.Context
	timeout     time.Duration

	Name      string
	NameWidth int
	Status    ProcessStatus
}

func NewProcess(name string, processFunc func(context.Context, func(float64)) error) *Process {
	d := &Process{
		id:          nextID(),
		Name:        name,
		spinner:     spinner.New(),
		progress:    progress.New(),
		timer:       stopwatch.NewWithInterval(time.Millisecond * 100),
		Status:      StatusPending,
		NameWidth:   len(name),
		processFunc: processFunc,
	}

	d.progress.Full = '•'
	d.progress.Empty = ' '
	d.progress.EmptyColor = ""
	d.progress.FullColor = "245"
	d.progress.ShowPercentage = true
	d.progress.PercentFormat = " %3.0f%%"

	return d
}

func (p *Process) Init() tea.Cmd {
	return nil
}

func (p *Process) Start() tea.Cmd {
	//nolint:staticcheck
	cmds := []tea.Cmd{
		p.timer.Init(),
		p.spinner.Tick,
		func() tea.Msg {
			return StatusMsg{
				ID:     p.id,
				status: StatusRunning,
			}
		},
	}

	cmds = append(cmds, func() tea.Msg {
		p := p // golang closures

		if p.norender {
			log.G(p.ctx).Info(p.Name)
		}

		err := p.processFunc(p.ctx, p.onProgress)
		p.Status = StatusSuccess
		if err != nil {
			p.Status = StatusFailed
		}

		if tprog != nil {
			tprog.Send(StatusMsg{
				ID:     p.id,
				status: p.Status,
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

func (p *Process) Close() error {
	return nil
}

func (p *Process) Fd() uintptr {
	return 0
}

func (d *Process) Update(msg tea.Msg) (*Process, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	d.timer, cmd = d.timer.Update(msg)
	cmds = append(cmds, cmd)

	switch msg := msg.(type) {
	// ProgressMsg is sent when the progress bar wishes
	case ProgressMsg:
		if msg.ID != d.id {
			return d, nil
		}

		d.percent = msg.progress

	// TickMsg is sent when the spinner wants to animate itself
	case spinner.TickMsg:
		if d.timeout != 0 && d.timer.Elapsed() > d.timeout {
			d.err = fmt.Errorf("process timedout after %s", d.timeout.String())
			d.Status = StatusFailed
			cmds = append(cmds, tea.Quit)
		}

		d.spinner, cmd = d.spinner.Update(msg)
		cmds = append(cmds, cmd)

	// StatusMsg is sent when the status of the process changes
	case StatusMsg:
		if msg.ID != d.id {
			return d, nil
		}

		d.Status = msg.status
		if d.Status == StatusFailed {
			d.err = msg.err
			cmds = append(cmds, d.timer.Stop())
		} else if d.Status == StatusSuccess {
			d.percent = 1.0
			cmds = append(cmds, d.timer.Stop())
		}

	// tea.WindowSizeMsg is sent when the terminal window is resized
	case tea.WindowSizeMsg:
		d.width = msg.Width
	}

	return d, tea.Batch(cmds...)
}

func (p Process) View() string {
	left := ""

	switch p.Status {
	case StatusRunning:
		left += tui.TextWhiteBgBlue("[" + p.spinner.View() + "]")
	case StatusSuccess:
		left += tui.TextWhiteBgGreen("[+]")
	default:
		if p.Status == StatusFailed || p.err != nil {
			left += tui.TextWhiteBgRed("<!>")
		} else {
			left += "[ ]"
		}
	}

	left += " "
	leftWidth := width(left)

	elapsed := utils.HumanizeDuration(p.timer.Elapsed())
	p.timerWidth = width(elapsed)
	elapsed = "[" + elapsed + "]"
	if p.timerMax-p.timerWidth < 0 {
		p.timerMax = p.timerWidth
	}

	right := " " + tui.TextLightGray(indent.String(elapsed, uint(p.timerMax-p.timerWidth)))
	rightWidth := width(right)

	middle := lipgloss.NewStyle().
		Width(p.NameWidth + 1).
		Render(p.Name)

	p.progress.Width = p.width - width(middle) - leftWidth - rightWidth
	middle += p.progress.ViewAs(p.percent)

	s := lipgloss.JoinHorizontal(lipgloss.Top,
		left,
		middle,
		right,
	)

	// Print the logs for this item
	if p.Status != StatusSuccess && p.percent < 1 {
		// Newline for the logs
		if len(p.logs) > 0 {
			s += "\n"
		}

		truncate := 0

		if p.Status != StatusFailed {
			loglen := len(p.logs) - LOGLEN
			if loglen > 0 {
				truncate = loglen
			}
		}

		for _, line := range p.logs[truncate:] {
			s += line + "\n"
		}
	}

	return s
}
