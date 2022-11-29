// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package list

import (
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"kraftkit.sh/unikraft/app"

	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"
	"kraftkit.sh/pack"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/utils"

	"kraftkit.sh/internal/cli"
)

type List struct {
	Limit     int  `long:"limit" short:"l" usage:"Set the maximum number of results" default:"50"`
	NoLimit   bool `long:"no-limit" usage:"Do not limit the number of items to print"`
	ShowApps  bool `long:"apps" short:"" usage:"Show applications"`
	ShowArchs bool `long:"archs" short:"M" usage:"Show architectures"`
	ShowCore  bool `long:"core" short:"C" usage:"Show Unikraft core versions"`
	ShowLibs  bool `long:"libs" short:"L" usage:"Show libraries"`
	ShowPlats bool `long:"plats" short:"P" usage:"Show platforms"`
	Update    bool `long:"update" short:"u" usage:"Get latest information about components before listing results"`
}

func New() *cobra.Command {
	return cli.New(&List{}, cobra.Command{
		Short:   "List installed Unikraft component packages",
		Use:     "list [FLAGS] [DIR]",
		Aliases: []string{"l", "ls"},
		Args:    cli.MaxDirArgs(1),
		Long: heredoc.Doc(`
			List installed Unikraft component packages.
		`),
		Example: heredoc.Doc(`
			$ kraft pkg list
		`),
	})
}

func (opts *List) Run(cmd *cobra.Command, args []string) error {
	var err error

	ctx := cmd.Context()
	workdir := ""
	if len(args) > 0 {
		workdir = args[0]
	}

	if opts.NoLimit {
		opts.Limit = -1
	}

	query := packmanager.CatalogQuery{}
	if opts.ShowCore {
		query.Types = append(query.Types, unikraft.ComponentTypeCore)
	}
	if opts.ShowArchs {
		query.Types = append(query.Types, unikraft.ComponentTypeArch)
	}
	if opts.ShowPlats {
		query.Types = append(query.Types, unikraft.ComponentTypePlat)
	}
	if opts.ShowLibs {
		query.Types = append(query.Types, unikraft.ComponentTypeLib)
	}
	if opts.ShowApps {
		query.Types = append(query.Types, unikraft.ComponentTypeApp)
	}

	var packages []pack.Package

	// List pacakges part of a project
	if len(workdir) > 0 {
		projectOpts, err := app.NewProjectOptions(
			nil,
			app.WithWorkingDirectory(workdir),
			app.WithDefaultConfigPath(),
		)
		if err != nil {
			return err
		}

		// Interpret the application
		app, err := app.NewApplicationFromOptions(projectOpts)
		if err != nil {
			return err
		}

		fmt.Fprint(iostreams.G(ctx).Out, app.PrintInfo())

	} else {
		packages, err = packmanager.G(ctx).Catalog(
			ctx,
			query,
			pack.WithWorkdir(workdir),
		)
		if err != nil {
			return err
		}
	}

	err = iostreams.G(ctx).StartPager()
	if err != nil {
		log.G(ctx).Errorf("error starting pager: %v", err)
	}

	defer iostreams.G(ctx).StopPager()

	cs := iostreams.G(ctx).ColorScheme()
	table := utils.NewTablePrinter(ctx)

	// Header row
	table.AddField("TYPE", nil, cs.Bold)
	table.AddField("PACKAGE", nil, cs.Bold)
	table.AddField("LATEST", nil, cs.Bold)
	table.AddField("FORMAT", nil, cs.Bold)
	table.EndRow()

	for _, pack := range packages {
		table.AddField(string(pack.Options().Type), nil, nil)
		table.AddField(pack.Name(), nil, nil)
		table.AddField(pack.Options().Version, nil, nil)
		table.AddField(pack.Format(), nil, nil)
		table.EndRow()
	}

	return table.Render()
}
