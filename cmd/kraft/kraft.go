// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package main

import (
	"os"

	"github.com/MakeNowJust/heredoc"
	"github.com/rancher/wrangler/pkg/signals"
	"github.com/spf13/cobra"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/bootstrap"
	"kraftkit.sh/internal/cli"
	kitupdate "kraftkit.sh/internal/update"
	kitversion "kraftkit.sh/internal/version"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"

	"kraftkit.sh/cmd/kraft/build"
	"kraftkit.sh/cmd/kraft/clean"
	"kraftkit.sh/cmd/kraft/events"
	"kraftkit.sh/cmd/kraft/fetch"
	"kraftkit.sh/cmd/kraft/login"
	"kraftkit.sh/cmd/kraft/logs"
	"kraftkit.sh/cmd/kraft/menu"
	"kraftkit.sh/cmd/kraft/net"
	"kraftkit.sh/cmd/kraft/pkg"
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
	_ "kraftkit.sh/oci"
)

type Kraft struct{}

func New(cfg *config.ConfigManager[config.KraftKit]) *cobra.Command {
	cmd, err := cmdfactory.New(&Kraft{}, cobra.Command{
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
	}, cfg)
	if err != nil {
		panic(err)
	}

	cmd.AddGroup(&cobra.Group{ID: "build", Title: "BUILD COMMANDS"})
	cmd.AddCommand(build.New(cfg))
	cmd.AddCommand(clean.New(cfg))
	cmd.AddCommand(fetch.New(cfg))
	cmd.AddCommand(menu.New(cfg))
	cmd.AddCommand(properclean.New(cfg))
	cmd.AddCommand(set.New(cfg))
	cmd.AddCommand(unset.New(cfg))

	cmd.AddGroup(&cobra.Group{ID: "pkg", Title: "PACKAGING COMMANDS"})
	cmd.AddCommand(pkg.New(cfg))

	cmd.AddGroup(&cobra.Group{ID: "run", Title: "RUNTIME COMMANDS"})
	cmd.AddCommand(events.New(cfg))
	cmd.AddCommand(logs.New(cfg))
	cmd.AddCommand(ps.New(cfg))
	cmd.AddCommand(rm.New(cfg))
	cmd.AddCommand(run.New(cfg))
	cmd.AddCommand(stop.New(cfg))

	cmd.AddGroup(&cobra.Group{ID: "net", Title: "LOCAL NETWORKING COMMANDS"})
	cmd.AddCommand(net.New(cfg))

	cmd.AddGroup(&cobra.Group{ID: "misc", Title: "MISCELLANEOUS COMMANDS"})
	cmd.AddCommand(login.New(cfg))
	cmd.AddCommand(version.New(cfg))

	return cmd
}

func (k *Kraft) Run(cmd *cobra.Command, args []string, cfg *config.ConfigManager[config.KraftKit]) error {
	return cmd.Help()
}

func main() {
	ctx := signals.SetupSignalContext()
	copts := &cli.CliOptions{}

	// TODO(jake-ciolek): We use the command tree to hydrate config.
	//                    We use config to hydrate the command tree.
	//                    This results in a circular initialization dependency.
	//                    Right now, we'll instantiate a short-lived, unhydrated
	//                    command tree to initialize a config manager.
	//                    Then, we'll use that to obtain the final hydrated command tree.
	//                    This will do for now, but spend some time thinking about how to make it nicer.
	cmdInit := New(nil)
	_, args, err := cmdInit.Find(os.Args[1:])
	if err != nil {
		log.G(ctx).Error(err)
		os.Exit(1)
	}

	cfgMgr, err := cli.ConfigManagerFromArgs(args)
	if err != nil {
		log.G(ctx).Error(err)
		os.Exit(1)
	}
	cmd := New(cfgMgr)

	for _, o := range []cli.CliOption{
		cli.WithConfigManager(cmd, cfgMgr),
		cli.WithDefaultIOStreams(cfgMgr.Config),
		cli.WithDefaultPluginManager(cfgMgr.Config),
		cli.WithDefaultLogger(cfgMgr.Config),
		cli.WithDefaultHTTPClient(cfgMgr.Config),
	} {
		if err := o(copts); err != nil {
			log.G(ctx).Error(err)
			os.Exit(1)
		}
	}

	// Set up the logger in the context if it is available
	if copts.Logger != nil {
		ctx = log.WithLogger(ctx, copts.Logger)
	}

	// Set up the iostreams in the context if it is available
	if copts.IOStreams != nil {
		ctx = iostreams.WithIOStreams(ctx, copts.IOStreams)
	}

	if !cfgMgr.Config.NoCheckUpdates {
		if err := kitupdate.Check(ctx); err != nil {
			log.G(ctx).Debugf("could not check for updates: %v", err)
			log.G(ctx).Debug("")
			log.G(ctx).Debug("to turn off this check, set:")
			log.G(ctx).Debug("")
			log.G(ctx).Debug("\texport KRAFTKIT_NO_CHECK_UPDATES=true")
			log.G(ctx).Debug("")
			log.G(ctx).Debug("or use the globally accessible flag '--no-check-updates'")
		}
	}

	if err := bootstrap.InitKraftkit(ctx, cfgMgr.Config); err != nil {
		log.G(ctx).Errorf("could not init kraftkit: %v", err)
		os.Exit(1)
	}

	cmdfactory.Main(ctx, cmd)
}
