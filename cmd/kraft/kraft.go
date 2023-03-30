// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package main

import (
	"fmt"
	"os"

	"github.com/MakeNowJust/heredoc"
	"github.com/rancher/wrangler/pkg/signals"
	"github.com/spf13/cobra"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	kitupdate "kraftkit.sh/internal/update"
	kitversion "kraftkit.sh/internal/version"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"

	"kraftkit.sh/cmd/kraft/build"
	"kraftkit.sh/cmd/kraft/clean"
	"kraftkit.sh/cmd/kraft/events"
	"kraftkit.sh/cmd/kraft/fetch"
	"kraftkit.sh/cmd/kraft/logs"
	"kraftkit.sh/cmd/kraft/menu"
	"kraftkit.sh/cmd/kraft/pkg"
	"kraftkit.sh/cmd/kraft/prepare"
	"kraftkit.sh/cmd/kraft/properclean"
	"kraftkit.sh/cmd/kraft/ps"
	"kraftkit.sh/cmd/kraft/rm"
	"kraftkit.sh/cmd/kraft/run"
	"kraftkit.sh/cmd/kraft/set"
	"kraftkit.sh/cmd/kraft/stop"
	"kraftkit.sh/cmd/kraft/unset"
	"kraftkit.sh/cmd/kraft/version"

	// Additional initializers
	_ "kraftkit.sh/manifest"
)

type Kraft struct{}

func New() *cobra.Command {
	cmd := cmdfactory.New(&Kraft{}, cobra.Command{
		Short: "Build and use highly customized and ultra-lightweight unikernels",
		Long: heredoc.Docf(`
        .
       /^\     Build and use highly customized and ultra-lightweight unikernels.
      :[ ]:
      | = |    Version:          %s
     /|/=\|\   Documentation:    https://kraftkit.sh/
    (_:| |:_)  Issues & support: https://github.com/unikraft/kraftkit/issues
       v v
       ' '`, kitversion.Version()),
		CompletionOptions: cobra.CompletionOptions{
			HiddenDefaultCmd: true,
		},
	})

	cmd.AddGroup(&cobra.Group{ID: "build", Title: "BUILD COMMANDS"})
	cmd.AddCommand(build.New())
	cmd.AddCommand(clean.New())
	cmd.AddCommand(fetch.New())
	cmd.AddCommand(menu.New())
	cmd.AddCommand(prepare.New())
	cmd.AddCommand(properclean.New())
	cmd.AddCommand(set.New())
	cmd.AddCommand(unset.New())

	cmd.AddGroup(&cobra.Group{ID: "pkg", Title: "PACKAGING COMMANDS"})
	cmd.AddCommand(pkg.New())

	cmd.AddGroup(&cobra.Group{ID: "run", Title: "RUNTIME COMMANDS"})
	cmd.AddCommand(events.New())
	cmd.AddCommand(logs.New())
	cmd.AddCommand(ps.New())
	cmd.AddCommand(rm.New())
	cmd.AddCommand(run.New())
	cmd.AddCommand(stop.New())

	cmd.AddGroup(&cobra.Group{ID: "misc", Title: "MISCELLANEOUS COMMANDS"})
	cmd.AddCommand(version.New())

	return cmd
}

func (k *Kraft) Run(cmd *cobra.Command, args []string) error {
	return cmd.Help()
}

func main() {
	cmd := New()
	ctx := signals.SetupSignalContext()
	copts := &CliOptions{}

	for _, o := range []CliOption{
		withDefaultConfigManager(cmd),
		withDefaultIOStreams(),
		withDefaultPackageManager(),
		withDefaultPluginManager(),
		withDefaultLogger(),
		withDefaultHTTPClient(),
	} {
		if err := o(copts); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}

	// Set up the config manager in the context if it is available
	if copts.configManager != nil {
		ctx = config.WithConfigManager(ctx, copts.configManager)
	}

	// Set up the logger in the context if it is available
	if copts.logger != nil {
		ctx = log.WithLogger(ctx, copts.logger)
	}

	// Set up the iostreams in the context if it is available
	if copts.ioStreams != nil {
		ctx = iostreams.WithIOStreams(ctx, copts.ioStreams)
	}

	if !config.G[config.KraftKit](ctx).NoCheckUpdates {
		if err := kitupdate.Check(ctx); err != nil {
			log.G(ctx).Debugf("could not check for updates: %v", err)
		}
	}

	cmdfactory.Main(ctx, cmd)
}
