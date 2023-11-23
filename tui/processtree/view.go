// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package processtree

import (
	"strconv"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/indent"
	"kraftkit.sh/tui"
	"kraftkit.sh/utils"
)

func (pt ProcessTree) View() string {
	if pt.norender {
		return ""
	}

	s := ""

	finished := 0

	// Update timers on all active items and their parents
	_ = pt.traverseTreeAndCall(pt.tree, func(pti *ProcessTreeItem) error {
		if pti.status == StatusSuccess ||
			pti.status == StatusFailed ||
			pti.status == StatusFailedChild {
			finished++
		}

		return nil
	})

	for _, pti := range pt.tree {
		s += pt.printItem(pti, 0)
	}

	if len(pt.verb) > 0 {
		title := tui.TextTitle(
			pt.verb + " " + utils.HumanizeDuration(pt.timer.Elapsed()) +
				" (" + strconv.Itoa(finished) + "/" + strconv.Itoa(pt.total) + ")",
		)
		s = title + "\n" + s
	}

	if pt.norender {
		s += " no render! \n"
	}

	if !pt.quitting {
		s += tui.TextLightGray("ctrl+c to cancel\n")
	}

	return s
}

func (stm ProcessTree) printItem(pti *ProcessTreeItem, offset uint) string {
	if pti.status == StatusSuccess && stm.hide {
		return ""
	}

	failed := 0
	completed := 0
	running := 0

	// Determine the status of immediate children whilst we're printing
	for _, child := range pti.children {
		if child.status == StatusFailed ||
			child.status == StatusFailedChild {
			failed++
		} else if child.status == StatusSuccess {
			completed++
		} else if child.status == StatusRunningChild ||
			child.status == StatusRunning ||
			child.status == StatusRunningButAChildHasFailed {
			running++
		}
	}

	if len(pti.children) > 0 {
		if failed > 0 && running > 0 {
			pti.status = StatusRunningButAChildHasFailed
		} else if running > 0 {
			pti.status = StatusRunningChild
		} else if failed > 0 {
			pti.status = StatusFailedChild
		}
	}

	width := lipgloss.Width

	textLeft := ""
	switch pti.status {
	case StatusSuccess:
		textLeft += tui.TextGreen("[+]")
	case StatusFailed, StatusFailedChild:
		textLeft += tui.TextRed("<!>")
	case StatusRunning, StatusRunningChild, StatusRunningButAChildHasFailed:
		textLeft += tui.TextBlue("[" + pti.spinner.View() + "]")
	default:
		textLeft += "[ ]"
	}

	textLeft += " " + pti.textLeft

	if pti.status == StatusRunning || pti.status == StatusRunningChild {
		textLeft += pti.ellipsis
	} else if pti.status == StatusSuccess {
		textLeft += "... done!"
	}

	elapsed := utils.HumanizeDuration(pti.timer.Elapsed())
	rightTimerWidth := width(elapsed)
	if rightTimerWidth > stm.rightPad {
		stm.rightPad = rightTimerWidth
	}

	textRight := ""
	if len(pti.textRight) > 0 {
		switch pti.status {
		case StatusSuccess:
			textRight += tui.TextGreen(pti.textRight)
		case StatusFailed, StatusFailedChild:
			textRight += tui.TextRed(pti.textRight)
		case StatusRunning, StatusRunningChild, StatusRunningButAChildHasFailed:
			textRight += tui.TextBlue(pti.textRight)
		default:
			textRight += pti.textRight
		}
	}

	elapsed = "[" + elapsed + "]"
	textRight += " " + tui.TextLightGray(indent.String(elapsed, uint(stm.rightPad-rightTimerWidth)))

	left := lipgloss.NewStyle().
		Width(stm.width - width(textRight) - int(offset*INDENTS)).
		Height(1).
		Render(textLeft)

	right := lipgloss.NewStyle().
		Width(width(textRight)).
		Height(1).
		Render(textRight)

	s := lipgloss.JoinHorizontal(lipgloss.Top,
		left,
		right,
	) + "\n"

	// Print the logs for this item
	truncate := 0
	loglen := len(pti.logs) - LOGLEN
	if loglen > 0 {
		truncate = loglen
	}
	if pti.status != StatusSuccess {
		for _, line := range pti.logs[truncate:] {
			s += line + "\n"
		}
	}

	// Print the child processes
	for _, child := range pti.children {
		s += stm.printItem(child, offset+1)
	}

	// Do not indent the root node
	if offset == 0 {
		return s
	}

	// Since this method is recursive, indent by 1 factor
	return indent.String(s, INDENTS)
}
