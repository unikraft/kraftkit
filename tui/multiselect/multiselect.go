// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package multiselect

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"kraftkit.sh/tui"
)

var (
	queryMark = lipgloss.NewStyle().
			Background(lipgloss.Color("12")).
			Foreground(lipgloss.AdaptiveColor{
			Light: "0",
			Dark:  "15",
		}).
		Render

	selectedText = lipgloss.NewStyle().
			Foreground(lipgloss.Color("32")).
			Render
)

// MultiSelect is a utility method used in a CLI context to prompt the
// user given a slice of options based on the generic type.
func MultiSelect[T fmt.Stringer](question string, options ...T) ([]T, error) {
	mapped := make(map[string]T)
	items := make([]item, 0, len(options))
	for _, option := range options {
		mapped[option.String()] = option
		items = append(items, item{
			text: option.String(),
		})
	}

	p := tea.NewProgram(&model{
		question: question,
		options:  items,
		item:     0,
	})

	// Run returns the model as a tea.Model.
	m, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("could not start multi selection prompt: %w", err)
	}

	mo := m.(*model)
	selected := make([]T, 0, len(options))
	for _, opt := range mo.options {
		if opt.checked {
			selected = append(selected, mapped[opt.text])
		}
	}

	return selected, nil
}

type model struct {
	question string
	options  []item
	item     int
	quitting bool
}

type item struct {
	text    string
	checked bool
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch typed := msg.(type) {
	case tea.KeyMsg:
		return m, m.handleKeyMsg(typed)
	}
	return m, nil
}

func (m *model) handleKeyMsg(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc", "ctrl+c":
		m.quitting = true
		return tea.Quit
	case "enter":
		m.quitting = true
		return tea.Quit
	case " ", "x", "y":
		m.options[m.item].checked = !m.options[m.item].checked
	case "up":
		if m.item > 0 {
			m.item--
		}
	case "down":
		if m.item+1 < len(m.options) {
			m.item++
		}
	}
	return nil
}

func (m *model) View() string {
	out := queryMark("[?]") + " " + m.question + ":\n"

	for i, item := range m.options {
		var text string
		check := ""
		if item.checked {
			check = tui.TextGreen("[x]")
		} else {
			check = tui.TextLightGray("[ ]")
		}

		if i == m.item && !m.quitting {
			text = selectedText("▸ " + item.text)
		} else {
			text = "  " + item.text
		}

		out += fmt.Sprintf("%s %s\n", check, text)
	}

	if !m.quitting {
		out += "\n"
		out += "use arrow keys; space, y or x to select; enter to continue"
	}

	return out
}
