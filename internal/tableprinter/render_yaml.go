// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package tableprinter

import (
	"fmt"
	"io"
	"strings"

	"gopkg.in/yaml.v2"
)

func (printer *TablePrinter) renderYAML(w io.Writer) error {
	header := printer.rows[0]
	var rows []map[string]string

	for i, row := range printer.rows {
		if i == 0 {
			continue
		}
		m := make(map[string]string)

		for j, column := range row {
			m[strings.ToLower(header[j].text)] = column.text
		}

		if len(m) > 0 {
			rows = append(rows, m)
		}
	}

	b, err := yaml.Marshal(rows)
	if err != nil {
		return err
	}

	if _, err = fmt.Fprint(w, string(b)); err != nil {
		return err
	}

	return nil
}
