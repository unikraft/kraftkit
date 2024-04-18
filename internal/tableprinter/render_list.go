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

func (printer *TablePrinter) renderList(w io.Writer) error {
	maxWidth := 0
	headers := printer.rows[0]

	for _, field := range headers {
		if len(field.text) > maxWidth {
			maxWidth = len(field.text)
		}
	}

	for i := 1; i < len(printer.rows); i++ {
		row := printer.rows[i]

		for col, field := range row {
			header := headers[col]
			headerText := strings.ToLower(header.text)

			if header.color != nil {
				headerText = header.color(headerText)
			}

			// pad value with spaces on the right
			if _, err := fmt.Fprint(w, strings.Repeat(" ", maxWidth-len(header.text)+1)); err != nil {
				return err
			}

			if headerText != "" {
				_, err := fmt.Fprintf(w, "%s: ", headerText)
				if err != nil {
					return err
				}
			}

			for i, value := range strings.Split(field.text, ", ") {
				if i > 0 {
					if _, err := fmt.Fprint(w, strings.Repeat(" ", maxWidth+3)); err != nil {
						return err
					}
				}

				if field.color != nil {
					value = field.color(value)
				}

				if _, err := fmt.Fprint(w, value); err != nil {
					return err
				}

				if _, err := fmt.Fprint(w, "\n"); err != nil {
					return err
				}
			}
		}

		if i < len(printer.rows)-1 {
			if _, err := fmt.Fprint(w, "\n"); err != nil {
				return err
			}
		}
	}

	return nil
}
