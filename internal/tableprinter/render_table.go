// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package tableprinter

import (
	"fmt"
	"io"
	"strings"
)

func (printer *TablePrinter) renderTable(w io.Writer) error {
	numCols := len(printer.rows[0])
	colWidths := printer.calculateColumnWidths(len(printer.delimeter))

	for _, row := range printer.rows {
		for col, field := range row {
			if col > 0 {
				_, err := fmt.Fprint(w, printer.delimeter)
				if err != nil {
					return err
				}
			}

			truncVal := printer.truncateFunc(colWidths[col], field.text)

			if col < numCols-1 {
				// pad value with spaces on the right
				if padWidth := colWidths[col] - field.DisplayWidth(); padWidth > 0 {
					truncVal += strings.Repeat(" ", padWidth)
				}
			}

			if field.color != nil {
				truncVal = field.color(truncVal)
			}

			_, err := fmt.Fprint(w, truncVal)
			if err != nil {
				return err
			}
		}

		if len(row) > 0 {
			_, err := fmt.Fprint(w, "\n")
			if err != nil {
				return err
			}
		}
	}

	return nil
}
