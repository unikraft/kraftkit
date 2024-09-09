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
	Current, Max float64
	Width        int
}

func (cell GuageCell) String() string {
	if cell.Width == 0 {
		return "    " // This is enough to cover "100%"
	}

	var ret strings.Builder

	percent := math.Floor(cell.Current / cell.Max * 100)
	if percent > 100 {
		percent = 100
	}

	color := "green"
	if percent > 85 {
		color = "red"
	} else if percent > 65 {
		color = "yellow"
	}

	ret.WriteString(cell.Cs.ColorFromString(color)(strings.Repeat("█", int(percent)/cell.Width)))
	ret.WriteString(strings.Repeat(" ", cell.Width-(int(percent)/cell.Width)))

	ret.WriteString(" ")
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
	)

	s := table.DefaultStyles()

	s.Header = lipgloss.NewStyle().Bold(true)
	s.Cell = lipgloss.NewStyle()
	s.Selected = s.Selected.
		Background(lipgloss.AdaptiveColor{
			Light: "0",
			Dark:  "0",
		}).
		Foreground(lipgloss.AdaptiveColor{
			Light: "0",
			Dark:  "#AFAFAF",
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
		pstable.width = msg.Width
		pstable.height = msg.Height
		pstable.table.SetWidth(msg.Width)

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
	delimSize := 2
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

	colWidths := make([]int, numCols)

	// Never truncate the first column
	colWidths[0] = maxColWidths[0]

	availWidth := func() int {
		setWidths := 0
		for col := 0; col < numCols; col++ {
			setWidths += colWidths[col]
		}

		return pstable.width - delimSize*(numCols-1) - setWidths
	}

	numFixedCols := func() int {
		fixedCols := 0

		for col := 0; col < numCols; col++ {
			if colWidths[col] > 0 {
				fixedCols++
			}
		}

		return fixedCols
	}

	// Set the widths of short columns
	if w := availWidth(); w > 0 {
		if numFlexColumns := numCols - numFixedCols(); numFlexColumns > 0 {
			perColumn := w / numFlexColumns
			for col := 0; col < numCols; col++ {
				if max := maxColWidths[col]; max < perColumn {
					colWidths[col] = max
				}
			}
		}
	}

	firstFlexCol := -1

	// truncate long columns to the remaining available width
	if numFlexColumns := numCols - numFixedCols(); numFlexColumns > 0 {
		perColumn := availWidth() / numFlexColumns
		for col := 0; col < numCols; col++ {
			if colWidths[col] == 0 {
				if firstFlexCol == -1 {
					firstFlexCol = col
				}
				if max := maxColWidths[col]; max < perColumn {
					colWidths[col] = max
				} else {
					colWidths[col] = perColumn
				}
			}
		}
	}

	// Add remainder to the first flex column
	if w := availWidth(); w > 0 && firstFlexCol > -1 {
		colWidths[firstFlexCol] += w
		if max := maxColWidths[firstFlexCol]; max < colWidths[firstFlexCol] {
			colWidths[firstFlexCol] = max
		}
	}

	// Pad all the columns if there is still space left
	if w := availWidth(); w > 0 {
		padding := w / numCols
		for col := 0; col < numCols; col++ {
			colWidths[col] += padding
		}
	}

	return colWidths
}

func (pstable *PsTable) renderTitle() string {
	return lipgloss.NewStyle().
		Background(lipgloss.AdaptiveColor{
			Light: "0",
			Dark:  "0",
		}).
		Foreground(lipgloss.AdaptiveColor{
			Light: "15",
			Dark:  "0",
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
