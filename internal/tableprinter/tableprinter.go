// SPDX-License-Identifier: MIT
//
// Copyright (c) 2019 GitHub Inc.
//               2022 Unikraft GmbH.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package tableprinter

import (
	"context"
	"io"
	"sort"
	"strings"

	"kraftkit.sh/internal/text"
)

type TableOutputFormat string

const (
	OutputFormatTable = TableOutputFormat("table")
	OutputFormatJSON  = TableOutputFormat("json")
	OutputFormatYAML  = TableOutputFormat("yaml")
	OutputFormatList  = TableOutputFormat("list")

	DefaultDelimeter = "  "
)

type TableField struct {
	text  string
	color func(string) string
}

func (f *TableField) DisplayWidth() int {
	return text.DisplayWidth(f.text)
}

type TablePrinter struct {
	format       TableOutputFormat
	rows         [][]TableField
	maxWidth     int
	delimeter    string
	truncateFunc func(int, string) string
}

// NewTablePrinter returns a pointer instance of TablePrinter struct.
func NewTablePrinter(ctx context.Context, topts ...TablePrinterOption) (*TablePrinter, error) {
	printer := TablePrinter{
		format:       OutputFormatTable,
		delimeter:    DefaultDelimeter,
		truncateFunc: text.Truncate,
	}

	for _, tpo := range topts {
		err := tpo(&printer)
		if err != nil {
			return nil, err
		}

	}

	return &printer, nil
}

// AddField adds a new field to the table.
func (printer *TablePrinter) AddField(s string, colorFunc func(string) string) {
	if printer.rows == nil {
		printer.rows = make([][]TableField, 1)
	}

	rowI := len(printer.rows) - 1
	field := TableField{
		text:  s,
		color: colorFunc,
	}

	printer.rows[rowI] = append(printer.rows[rowI], field)
}

// EndRow ends the current row.
func (printer *TablePrinter) EndRow() {
	printer.rows = append(printer.rows, []TableField{})
}

func (printer *TablePrinter) Render(w io.Writer) error {
	if len(printer.rows) == 0 {
		return nil
	}

	switch printer.format {
	case OutputFormatList:
		return printer.renderList(w)
	case OutputFormatJSON:
		return printer.renderJSON(w)
	case OutputFormatYAML:
		return printer.renderYAML(w)
	default:
		return printer.renderTable(w)
	}
}

func (printer *TablePrinter) calculateColumnWidths(delimSize int) []int {
	numCols := len(printer.rows[0])
	allColWidths := make([][]int, numCols)
	for _, row := range printer.rows {
		for col, field := range row {
			allColWidths[col] = append(allColWidths[col], field.DisplayWidth())
		}
	}

	// calculate max & median content width per column
	maxColWidths := make([]int, numCols)
	// medianColWidth := make([]int, numCols)
	for col := 0; col < numCols; col++ {
		widths := allColWidths[col]
		sort.Ints(widths)
		maxColWidths[col] = widths[len(widths)-1]
		// medianColWidth[col] = widths[(len(widths)+1)/2]
	}

	colWidths := make([]int, numCols)

	// never truncate the first column
	colWidths[0] = maxColWidths[0]

	// never truncate the last column if it contains URLs
	if strings.HasPrefix(printer.rows[0][numCols-1].text, "https://") {
		colWidths[numCols-1] = maxColWidths[numCols-1]
	}

	availWidth := func() int {
		setWidths := 0
		for col := 0; col < numCols; col++ {
			setWidths += colWidths[col]
		}
		return printer.maxWidth - delimSize*(numCols-1) - setWidths
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

	// set the widths of short columns
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

	// add remainder to the first flex column
	if w := availWidth(); w > 0 && firstFlexCol > -1 {
		colWidths[firstFlexCol] += w
		if max := maxColWidths[firstFlexCol]; max < colWidths[firstFlexCol] {
			colWidths[firstFlexCol] = max
		}
	}

	return colWidths
}
