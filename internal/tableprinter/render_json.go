package tableprinter

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

func (printer *TablePrinter) renderJSON(w io.Writer) error {
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

	b, err := json.Marshal(rows)
	if err != nil {
		return err
	}

	if _, err = fmt.Fprint(w, string(b)); err != nil {
		return err
	}

	return nil
}
