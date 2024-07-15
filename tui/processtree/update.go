// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package processtree

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

func (pt *ProcessTree) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	// Update the global timer
	pt.timer, cmd = pt.timer.Update(msg)
	cmds = append(cmds, cmd)

	// Update timers on all active items and their parents
	_ = pt.traverseTreeAndCall(pt.tree, func(pti *ProcessTreeItem) error {
		if pti.status == StatusRunning ||
			pti.status == StatusRunningChild ||
			pti.status == StatusRunningButAChildHasFailed {
			pti.timer, cmd = pti.timer.Update(msg)
			cmds = append(cmds, cmd)
		}

		return nil
	})

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			pt.quitting = true
			pt.err = fmt.Errorf("force quit")
			return pt, tea.Quit
		}

	case spinner.TickMsg:
		_ = pt.traverseTreeAndCall(pt.tree, func(pti *ProcessTreeItem) error {
			if pti.timeout != 0 && pti.timer.Elapsed() > pti.timeout {
				pti.err = fmt.Errorf("process timed out after %s", pti.timeout.String())
				pti.status = StatusFailed
				if pt.failFast {
					pt.quitting = true
					pt.err = pti.err
					cmd = tea.Quit
				}
			} else {
				pti.spinner, cmd = pti.spinner.Update(msg)
			}

			if pti.status == StatusRunning ||
				pti.status == StatusRunningChild ||
				pti.status == StatusRunningButAChildHasFailed {
				pti.ellipsis = strings.Repeat(".", int(pti.timer.Elapsed().Seconds())%4)
			} else {
				pti.ellipsis = ""
			}

			cmds = append(cmds, cmd)

			return nil
		})

		return pt, tea.Batch(cmds...)

	case processExitMsg:
		cmds = append(cmds, msg.timer.Stop())

		if msg.status == StatusSuccess ||
			msg.status == StatusFailed ||
			msg.status == StatusFailedChild {
			pt.finished++
		}

		// No more processes then exit
		if pt.total == pt.finished {
			pt.quitting = true
			cmds = append(cmds, tea.Quit)
		} else {
			_ = pt.traverseTreeAndCall(pt.tree, func(pti *ProcessTreeItem) error {
				if !pti.timer.Running() {
					cmds = append(cmds, pti.timer.Init())
				}
				return nil
			})

			children := pt.getNextReadyChildren(pt.tree)
			for _, pti := range children {
				pti := pti
				cmds = append(cmds, pt.waitForProcessCmd(pti))
			}

			cmds = append(cmds, waitForProcessExit(pt.channel))
		}

		return pt, tea.Batch(cmds...)

	case tea.WindowSizeMsg:
		pt.width = msg.Width
		return pt, nil
	}

	return pt, tea.Batch(cmds...)
}
