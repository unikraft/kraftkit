// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package pstable

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"kraftkit.sh/internal/cli/kraft/cloud/utils"
	"kraftkit.sh/internal/text"
	"kraftkit.sh/iostreams"
)

type Cell fmt.Stringer

type Row []Cell

type StringCell string

func (cell StringCell) String() string {
	return string(cell)
}

type GuageCell struct {
	Cs           *iostreams.ColorScheme
	Current, Max int
	Width        int
}

func (cell GuageCell) String() string {
	var ret *strings.Builder

	if cell.Width != 0 {
		ret = utils.ProgressBarBuilder(cell.Cs, cell.Current, cell.Max, cell.Width)
		ret.WriteString(" ")
	} else {
		ret = &strings.Builder{}
	}

	percent := math.Floor(float64(cell.Current) / float64(cell.Max) * 100)
	if percent > 100 {
		percent = 0
	}

	ret.WriteString(fmt.Sprintf("%.0f%%", percent))

	return ret.String()
}

type CallbackFunc func(context.Context) ([]Row, error)

type Column table.Column

type PsTable struct {
	columns  []Column
	callback CallbackFunc
	title    string
	width    int
	height   int
	ctx      context.Context
	table    table.Model
	err      error
}

func NewPSTable(ctx context.Context, title string, cols []Column, callback CallbackFunc) (*PsTable, error) {
	return &PsTable{
		title:    title,
		columns:  cols,
		callback: callback,
	}, nil
}

func (pstable *PsTable) Start(ctx context.Context) error {
	prog := tea.NewProgram(pstable,
		tea.WithAltScreen(),
		tea.WithContext(ctx),
	)

	var cols []table.Column
	for _, col := range pstable.columns {
		cols = append(cols, table.Column(col))
	}

	pstable.table = table.New(
		table.WithColumns(cols),
		table.WithFocused(true),
		table.WithWidth(256),
	)

	s := table.DefaultStyles()

	s.Header = lipgloss.NewStyle().Bold(true)
	s.Cell = lipgloss.NewStyle().Padding(0)
	s.Selected = s.Selected.
		Background(lipgloss.AdaptiveColor{
			Light: "0",
			Dark:  "0",
		}).
		Foreground(lipgloss.AdaptiveColor{
			Light: "#DFDFDF",
			Dark:  "#CFCFCF",
		})
	pstable.table.SetStyles(s)

	pstable.ctx = ctx

	_, err := prog.Run()
	if err != nil {
		return err
	}

	return pstable.err
}

func (pstable *PsTable) Init() tea.Cmd {
	return pstable.tick()
}

func (pstable *PsTable) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		pstable.width = 256
		pstable.height = msg.Height
		pstable.table.SetWidth(256)

	case []Row:
		if len(msg) > 0 {
			widths := pstable.calculateColumnWidths(msg)

			for i := range pstable.columns {
				pstable.columns[i].Width = widths[i]
			}

			var cols []table.Column
			for _, col := range pstable.columns {
				cols = append(cols, table.Column(col))
			}

			pstable.table.SetColumns(cols)

			var rows []table.Row
			for _, row := range msg {
				var nrow table.Row
				for _, cell := range row {
					nrow = append(nrow, cell.String())
				}

				rows = append(rows, nrow)
			}

			pstable.table.SetRows(rows)

			pstable.table, cmd = pstable.table.Update(msg)
			cmds = append(cmds, cmd)
		}

		cmds = append(cmds, pstable.tick())

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return pstable, tea.Quit
		case "up", "k":
			pstable.table.MoveUp(1)
		case "down", "j":
			pstable.table.MoveDown(1)
		}
	}

	return pstable, tea.Batch(cmds...)
}

func (pstable *PsTable) tick() tea.Cmd {
	return func() tea.Msg {
		rows, _ := pstable.callback(pstable.ctx)

		return rows
	}
}

func (pstable *PsTable) calculateColumnWidths(rows []Row) []int {
	numCols := len(rows[0])
	allColWidths := make([][]int, numCols)

	for _, row := range rows {
		for col, field := range row {
			allColWidths[col] = append(allColWidths[col], text.DisplayWidth(field.String()))
		}
	}

	// Calculate max content width per column
	maxColWidths := make([]int, numCols)
	for col := 0; col < numCols; col++ {
		widths := allColWidths[col]
		sort.Ints(widths)
		maxColWidths[col] = widths[len(widths)-1]
	}

	availWidth := func() int {
		setWidths := 0
		for col := 0; col < numCols; col++ {
			setWidths += maxColWidths[col]
		}
		return pstable.width - 2*numCols - setWidths
	}

	// Pad all the columns if there is still space left
	if w := availWidth(); w > 0 {
		padding := w / numCols
		for col := 0; col < numCols; col++ {
			maxColWidths[col] += padding
		}
	}

	return maxColWidths
}

func (pstable *PsTable) renderTitle() string {
	return lipgloss.NewStyle().
		Background(lipgloss.AdaptiveColor{
			Light: "0",
			Dark:  "0",
		}).
		Foreground(lipgloss.AdaptiveColor{
			Light: "#DFDFDF",
			Dark:  "#CFCFCF",
		}).
		Width(pstable.width).
		Render(pstable.title)
}

func (pstable *PsTable) View() string {
	var s string

	if len(pstable.table.Rows()) == 0 {
		s = "0 records found"
	} else {
		s = pstable.table.View()
	}

	return lipgloss.JoinVertical(lipgloss.Top,
		pstable.renderTitle(),
		s,
	)
}
