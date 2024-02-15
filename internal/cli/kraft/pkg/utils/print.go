// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package utils

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/dustin/go-humanize"
	"kraftkit.sh/internal/tableprinter"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/pack"
)

// PrintPackages is a utility method for outputting information about a the set
// of provided packages with the given style to the provided output.
func PrintPackages(ctx context.Context, out io.Writer, style string, packs ...pack.Package) error {
	cs := iostreams.G(ctx).ColorScheme()

	formats := map[pack.PackageFormat][]pack.Package{}

	for _, p := range packs {
		if _, ok := formats[p.Format()]; !ok {
			formats[p.Format()] = []pack.Package{}
		}

		formats[p.Format()] = append(formats[p.Format()], p)
	}

	for _, packs := range formats {
		table, err := tableprinter.NewTablePrinter(ctx,
			tableprinter.WithMaxWidth(iostreams.G(ctx).TerminalWidth()),
			tableprinter.WithOutputFormatFromString(style),
		)
		if err != nil {
			return err
		}

		table.AddField("TYPE", cs.Bold)
		table.AddField("NAME", cs.Bold)
		table.AddField("VERSION", cs.Bold)
		table.AddField("FORMAT", cs.Bold)
		table.AddField("PULLED", cs.Bold)

		if len(packs) == 0 {
			return nil
		}

		metadata := packs[0].Columns()
		for _, k := range metadata {
			table.AddField(strings.ToUpper(k.Name), cs.Bold)
		}

		table.EndRow()

		for _, pack := range packs {
			table.AddField(string(pack.Type()), nil)
			table.AddField(pack.Name(), nil)
			table.AddField(pack.Version(), nil)
			table.AddField(pack.Format().String(), nil)
			pulledAt := "never"
			pulled, pulledTime, err := pack.PulledAt(ctx)
			if err != nil {
				pulledAt = err.Error()
			} else if pulled {
				pulledAt = humanize.Time(pulledTime)
			}
			table.AddField(pulledAt, nil)

			for _, v := range pack.Columns() {
				table.AddField(v.Value, nil)
			}

			table.EndRow()
		}

		if err := table.Render(out); err != nil {
			return fmt.Errorf("rendering table: %w", err)
		}

		fmt.Fprint(out, "\n")
	}

	return nil
}
