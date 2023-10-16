// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package list

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"kraftkit.sh/unikraft/app"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/internal/tableprinter"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"
	"kraftkit.sh/pack"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/unikraft"
)

type List struct {
	Kraftfile string `long:"kraftfile" short:"K" usage:"Set an alternative path of the Kraftfile"`
	Limit     int    `long:"limit" short:"l" usage:"Set the maximum number of results" default:"50"`
	NoLimit   bool   `long:"no-limit" usage:"Do not limit the number of items to print"`
	ShowApps  bool   `long:"apps" short:"" usage:"Show applications"`
	ShowArchs bool   `long:"archs" short:"M" usage:"Show architectures"`
	ShowCore  bool   `long:"core" short:"C" usage:"Show Unikraft core versions"`
	ShowLibs  bool   `long:"libs" short:"L" usage:"Show libraries"`
	ShowPlats bool   `long:"plats" short:"P" usage:"Show platforms"`
	Update    bool   `long:"update" short:"u" usage:"Get latest information about components before listing results"`
	Output    string `long:"output" short:"o" usage:"Set output format" default:"table"`
}

func New() *cobra.Command {
	cmd, err := cmdfactory.New(&List{}, cobra.Command{
		Short:   "List installed Unikraft component packages",
		Use:     "ls [FLAGS] [DIR]",
		Aliases: []string{"l", "list"},
		Args:    cmdfactory.MaxDirArgs(1),
		Long: heredoc.Doc(`
			List installed Unikraft component packages.
		`),
		Example: heredoc.Doc(`
			$ kraft pkg list`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "pkg",
		},
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (*List) Pre(cmd *cobra.Command, _ []string) error {
	ctx, err := packmanager.WithDefaultUmbrellaManagerInContext(cmd.Context())
	if err != nil {
		return err
	}

	cmd.SetContext(ctx)

	return nil
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

	types := []unikraft.ComponentType{}
	if opts.ShowCore {
		types = append(types, unikraft.ComponentTypeCore)
	}
	if opts.ShowArchs {
		types = append(types, unikraft.ComponentTypeArch)
	}
	if opts.ShowPlats {
		types = append(types, unikraft.ComponentTypePlat)
	}
	if opts.ShowLibs {
		types = append(types, unikraft.ComponentTypeLib)
	}
	if opts.ShowApps {
		types = append(types, unikraft.ComponentTypeApp)
	}

	packages := make(map[string][]pack.Package)

	// List pacakges part of a project
	if f, err := os.Stat(workdir); len(workdir) > 0 && err == nil && f.IsDir() {
		popts := []app.ProjectOption{
			app.WithProjectWorkdir(workdir),
		}

		if len(opts.Kraftfile) > 0 {
			popts = append(popts, app.WithProjectKraftfile(opts.Kraftfile))
		} else {
			popts = append(popts, app.WithProjectDefaultKraftfiles())
		}

		project, err := app.NewProjectFromOptions(ctx, popts...)
		if err != nil {
			return err
		}

		fmt.Fprint(iostreams.G(ctx).Out, project.PrintInfo(ctx))
	} else {
		found, err := packmanager.G(ctx).Catalog(ctx,
			packmanager.WithUpdate(opts.Update),
			packmanager.WithTypes(types...),
		)
		if err != nil {
			return err
		}

		for _, p := range found {
			format := p.Format().String()
			if _, ok := packages[format]; !ok {
				packages[format] = make([]pack.Package, 0)
			}

			packages[format] = append(packages[format], p)
		}
	}

	for format, packs := range packages {
		// Sort packages by type, name, version, format
		sort.Slice(packs, func(i, j int) bool {
			if packages[format][i].Type() != packages[format][j].Type() {
				return packages[format][i].Type() < packages[format][j].Type()
			}

			if packages[format][i].Name() != packages[format][j].Name() {
				return packages[format][i].Name() < packages[format][j].Name()
			}

			if packages[format][i].Version() != packages[format][j].Version() {
				return packages[format][i].Version() < packages[format][j].Version()
			}

			return packages[format][i].Format() < packages[format][j].Format()
		})
	}

	err = iostreams.G(ctx).StartPager()
	if err != nil {
		log.G(ctx).Errorf("error starting pager: %v", err)
	}

	defer iostreams.G(ctx).StopPager()

	cs := iostreams.G(ctx).ColorScheme()

	for _, packs := range packages {
		table, err := tableprinter.NewTablePrinter(ctx,
			tableprinter.WithMaxWidth(iostreams.G(ctx).TerminalWidth()),
			tableprinter.WithOutputFormatFromString(opts.Output),
		)
		if err != nil {
			return err
		}

		// Header row
		table.AddField("TYPE", cs.Bold)
		table.AddField("NAME", cs.Bold)
		table.AddField("VERSION", cs.Bold)
		table.AddField("FORMAT", cs.Bold)

		if len(packs) == 0 {
			continue
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

			for _, v := range pack.Columns() {
				table.AddField(v.Value, nil)
			}

			table.EndRow()
		}

		if err := table.Render(iostreams.G(ctx).Out); err != nil {
			return err
		}
	}

	return nil
}
