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
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

func (pt ProcessTree) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	// Update the global timer
	pt.timer, cmd = pt.timer.Update(msg)
	cmds = append(cmds, cmd)

	// Update timers on all active items and their parents
	TraverseTreeAndCall(pt.tree, func(pti *ProcessTreeItem) error {
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
			return pt, tea.Quit
		}

	case spinner.TickMsg:
		TraverseTreeAndCall(pt.tree, func(pti *ProcessTreeItem) error {
			pti.spinner, cmd = pti.spinner.Update(msg)
			cmds = append(cmds, cmd)

			return nil
		})

		return pt, tea.Batch(cmds...)

	case processExitMsg:
		cmds = append(cmds, msg.timer.Stop())

		if msg.status == StatusSuccess ||
			msg.status == StatusFailed ||
			msg.status == StatusFailedChild {
			pt.finished += 1
		}

		// No more processes then exit
		if pt.total == pt.finished {
			pt.quitting = true
			cmds = append(cmds, tea.Quit)
		} else {
			TraverseTreeAndCall(pt.tree, func(pti *ProcessTreeItem) error {
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
