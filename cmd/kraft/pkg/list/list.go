// SPDX-License-Identifier: BSD-3-Clause
//
// Authors: Alexander Jung <alex@unikraft.io>
//
// Copyright (c) 2022, Unikraft GmbH.  All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions
// are met:
//
// 1. Redistributions of source code must retain the above copyright
//    notice, this list of conditions and the following disclaimer.
// 2. Redistributions in binary form must reproduce the above copyright
//    notice, this list of conditions and the following disclaimer in the
//    documentation and/or other materials provided with the distribution.
// 3. Neither the name of the copyright holder nor the names of its
//    contributors may be used to endorse or promote products derived from
//    this software without specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
// AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
// IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
// ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE
// LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR
// CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF
// SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS
// INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN
// CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE)
// ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE
// POSSIBILITY OF SUCH DAMAGE.

package list

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"kraftkit.sh/config"
	"kraftkit.sh/unikraft/app"

	"kraftkit.sh/internal/cmdfactory"
	"kraftkit.sh/internal/cmdutil"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"
	"kraftkit.sh/pack"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/utils"
)

type ListOptions struct {
	PackageManager func(opts ...packmanager.PackageManagerOption) (packmanager.PackageManager, error)
	ConfigManager  func() (*config.ConfigManager, error)
	Logger         func() (log.Logger, error)
	IO             *iostreams.IOStreams

	LimitResults int
	AsJSON       bool
	Update       bool
	ShowCore     bool
	ShowArchs    bool
	ShowPlats    bool
	ShowLibs     bool
	ShowApps     bool
}

func ListCmd(f *cmdfactory.Factory) *cobra.Command {
	opts := &ListOptions{
		PackageManager: f.PackageManager,
		ConfigManager:  f.ConfigManager,
		Logger:         f.Logger,
		IO:             f.IOStreams,
	}

	cmd, err := cmdutil.NewCmd(f, "list")
	if err != nil {
		panic("could not initialize 'kraft pkg list' command")
	}

	cmd.Short = "List installed Unikraft component packages"
	cmd.Use = "list [FLAGS] [DIR]"
	cmd.Aliases = []string{"l", "ls"}
	cmd.Args = cmdutil.MaxDirArgs(1)
	cmd.Long = heredoc.Doc(`
		List installed Unikraft component packages.
	`)
	cmd.Example = heredoc.Doc(`
		$ kraft pkg list
	`)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		workdir := ""
		if len(args) > 0 {
			workdir = args[0]
		}
		return listRun(opts, workdir)
	}

	cmd.Flags().IntVarP(
		&opts.LimitResults,
		"limit", "l",
		30,
		"Maximum number of items to print (-1 returns all)",
	)

	noLimitResults := false
	cmd.Flags().BoolVarP(
		&noLimitResults,
		"no-limit", "T",
		false,
		"Do not limit the number of items to print",
	)
	if noLimitResults {
		opts.LimitResults = -1
	}

	cmd.Flags().BoolVarP(
		&opts.Update,
		"update", "U",
		false,
		"Get latest information about components before listing results",
	)

	cmd.Flags().BoolVarP(
		&opts.ShowCore,
		"core", "C",
		false,
		"Show Unikraft core versions",
	)

	cmd.Flags().BoolVarP(
		&opts.ShowArchs,
		"arch", "M",
		false,
		"Show architectures",
	)

	cmd.Flags().BoolVarP(
		&opts.ShowPlats,
		"plats", "P",
		false,
		"Show platforms",
	)

	cmd.Flags().BoolVarP(
		&opts.ShowLibs,
		"libs", "L",
		false,
		"Show libraries",
	)

	cmd.Flags().BoolVarP(
		&opts.ShowApps,
		"apps", "A",
		false,
		"Show applications",
	)

	return cmd
}

func listRun(opts *ListOptions, workdir string) error {
	var err error

	pm, err := opts.PackageManager()
	if err != nil {
		return err
	}

	plog, err := opts.Logger()
	if err != nil {
		return err
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
			app.WithLogger(plog),
			app.WithWorkingDirectory(workdir),
			app.WithDefaultConfigPath(),
			app.WithPackageManager(&pm),
		)
		if err != nil {
			return err
		}

		// Interpret the application
		app, err := app.NewApplicationFromOptions(projectOpts)
		if err != nil {
			return err
		}

		app.PrintInfo(opts.IO)

	} else {
		packages, err = pm.Catalog(query,
			pack.WithWorkdir(workdir),
		)
		if err != nil {
			return err
		}
	}

	err = opts.IO.StartPager()
	if err != nil {
		plog.Errorf("error starting pager: %v", err)
	}

	defer opts.IO.StopPager()

	cs := opts.IO.ColorScheme()
	table := utils.NewTablePrinter(opts.IO)

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
