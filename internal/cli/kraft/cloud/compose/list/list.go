// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package list

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/compose"
	"kraftkit.sh/internal/tableprinter"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"
)

type ListOptions struct {
	Composefile string `noattribute:"true"`
	Output      string `long:"output" short:"o" usage:"Set output format. Options: table,yaml,json,list" default:"table"`
	Token       string `noattribute:"true"`
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&ListOptions{}, cobra.Command{
		Short:   "List KraftCloud deployments at a given path",
		Use:     "ls [FLAGS] [PATH]",
		Args:    cobra.MaximumNArgs(1),
		Aliases: []string{"l", "ls"},
		Long: heredoc.Doc(`
			List KraftCloud deployments at a given path.
		`),
		Example: heredoc.Doc(`
			# List KraftCloud deployments at a given path.
			$ kraft cloud compose ls /path/to/deployment

			# List KraftCloud deployments in the current directory.
			$ kraft cloud compose ls
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "kraftcloud-compose",
		},
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *ListOptions) Run(ctx context.Context, args []string) error {
	var err error
	var workdir string

	if len(args) == 0 {
		workdir, err = os.Getwd()
		if err != nil {
			return err
		}
	} else {
		workdir = args[0]
	}

	project, err := compose.NewProjectFromComposeFile(ctx, workdir, opts.Composefile)
	if err != nil {
		return err
	}

	if err := project.Validate(ctx); err != nil {
		return err
	}

	if err = iostreams.G(ctx).StartPager(); err != nil {
		log.G(ctx).Errorf("error starting pager: %v", err)
	}

	defer iostreams.G(ctx).StopPager()

	cs := iostreams.G(ctx).ColorScheme()

	table, err := tableprinter.NewTablePrinter(ctx,
		tableprinter.WithMaxWidth(iostreams.G(ctx).TerminalWidth()),
		tableprinter.WithOutputFormatFromString(opts.Output),
	)
	if err != nil {
		return err
	}

	table.AddField("NAME", cs.Bold)
	table.AddField("STATUS", cs.Bold)
	table.AddField("COMPOSEFILE", cs.Bold)
	table.EndRow()

	table.AddField(project.Name, nil)
	table.AddField("valid", cs.Green)

	var files string
	for _, file := range project.ComposeFiles {
		files += filepath.Join(workdir, file) + ", "
	}

	table.AddField(strings.TrimSuffix(files, ", "), nil)
	table.EndRow()

	return table.Render(iostreams.G(ctx).Out)
}
