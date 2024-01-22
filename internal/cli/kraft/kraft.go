// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package kraft

import (
	"context"
	"fmt"
	"os"

	"github.com/MakeNowJust/heredoc"
	"github.com/rancher/wrangler/pkg/signals"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/bootstrap"
	"kraftkit.sh/internal/cli"
	"kraftkit.sh/internal/cli/kraft/lib"
	kitupdate "kraftkit.sh/internal/update"
	kitversion "kraftkit.sh/internal/version"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"

	"kraftkit.sh/internal/cli/kraft/build"
	"kraftkit.sh/internal/cli/kraft/clean"
	"kraftkit.sh/internal/cli/kraft/cloud"
	"kraftkit.sh/internal/cli/kraft/events"
	"kraftkit.sh/internal/cli/kraft/fetch"
	"kraftkit.sh/internal/cli/kraft/login"
	"kraftkit.sh/internal/cli/kraft/logs"
	"kraftkit.sh/internal/cli/kraft/menu"
	"kraftkit.sh/internal/cli/kraft/net"
	"kraftkit.sh/internal/cli/kraft/pkg"
	"kraftkit.sh/internal/cli/kraft/ps"
	"kraftkit.sh/internal/cli/kraft/remove"
	"kraftkit.sh/internal/cli/kraft/run"
	"kraftkit.sh/internal/cli/kraft/set"
	"kraftkit.sh/internal/cli/kraft/start"
	"kraftkit.sh/internal/cli/kraft/stop"
	"kraftkit.sh/internal/cli/kraft/unset"
	"kraftkit.sh/internal/cli/kraft/version"
	"kraftkit.sh/internal/cli/kraft/x"

	// Additional initializers
	_ "kraftkit.sh/manifest"
	_ "kraftkit.sh/oci"
)

type KraftOptions struct{}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&KraftOptions{}, cobra.Command{
		Short: "Build and use highly customized and ultra-lightweight unikernels",
		Use:   "kraft [FLAGS] SUBCOMMAND",
		Long: heredoc.Docf(`
        .
       /^\     Build and use highly customized and ultra-lightweight unikernels.
      :[ ]:
      | = |    Version:          %s
     /|/=\|\   Documentation:    https://kraftkit.sh/
    (_:| |:_)  Issues & support: https://github.com/unikraft/kraftkit/issues
       v v     Platform:         https://kraft.cloud/ (Join the beta!)
       ' '`, kitversion.Version()),
		CompletionOptions: cobra.CompletionOptions{
			HiddenDefaultCmd: true,
		},
	})
	if err != nil {
		panic(err)
	}

	cmd.AddGroup(&cobra.Group{ID: "build", Title: "BUILD COMMANDS"})
	cmd.AddCommand(build.NewCmd())
	cmd.AddCommand(clean.NewCmd())
	cmd.AddCommand(fetch.NewCmd())
	cmd.AddCommand(menu.NewCmd())
	cmd.AddCommand(set.NewCmd())
	cmd.AddCommand(unset.NewCmd())

	cmd.AddGroup(&cobra.Group{ID: "lib", Title: "PROJECT LIBRARY COMMANDS"})
	cmd.AddCommand(lib.NewCmd())

	cmd.AddGroup(&cobra.Group{ID: "pkg", Title: "PACKAGING COMMANDS"})
	cmd.AddCommand(pkg.NewCmd())

	cmd.AddGroup(&cobra.Group{ID: "run", Title: "LOCAL RUNTIME COMMANDS"})
	cmd.AddCommand(events.NewCmd())
	cmd.AddCommand(logs.NewCmd())
	cmd.AddCommand(ps.NewCmd())
	cmd.AddCommand(remove.NewCmd())
	cmd.AddCommand(run.NewCmd())
	cmd.AddCommand(start.NewCmd())
	cmd.AddCommand(stop.NewCmd())

	cmd.AddGroup(&cobra.Group{ID: "net", Title: "LOCAL NETWORKING COMMANDS"})
	cmd.AddCommand(net.NewCmd())

	cmd.AddGroup(&cobra.Group{ID: "kraftcloud", Title: "KRAFT CLOUD COMMANDS"})
	cmd.AddCommand(cloud.NewCmd())

	cmd.AddGroup(&cobra.Group{ID: "kraftcloud-img", Title: "KRAFT CLOUD IMAGE COMMANDS"})
	cmd.AddGroup(&cobra.Group{ID: "kraftcloud-instance", Title: "KRAFT CLOUD INSTANCE COMMANDS"})
	cmd.AddGroup(&cobra.Group{ID: "kraftcloud-vol", Title: "KRAFT CLOUD VOLUME COMMANDS"})
	cmd.AddGroup(&cobra.Group{ID: "kraftcloud-svc", Title: "KRAFT CLOUD SERVICE GROUP COMMANDS"})

	cmd.AddGroup(&cobra.Group{ID: "misc", Title: "MISCELLANEOUS COMMANDS"})
	cmd.AddCommand(login.NewCmd())
	cmd.AddCommand(version.NewCmd())

	cmd.AddCommand(x.NewCmd())

	return cmd
}

func (k *KraftOptions) Run(_ context.Context, args []string) error {
	return pflag.ErrHelp
}

func Main(args []string) int {
	cmd := NewCmd()
	ctx := signals.SetupSignalContext()
	copts := &cli.CliOptions{}

	for _, o := range []cli.CliOption{
		cli.WithDefaultConfigManager(cmd),
		cli.WithDefaultIOStreams(),
		cli.WithDefaultPluginManager(),
		cli.WithDefaultLogger(),
		cli.WithDefaultHTTPClient(),
	} {
		if err := o(copts); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}

	// Set up the config manager in the context if it is available
	if copts.ConfigManager != nil {
		ctx = config.WithConfigManager(ctx, copts.ConfigManager)
	}

	// Hydrate KraftCloud configuration
	if newCtx, err := config.HydrateKraftCloudAuthInContext(ctx); err == nil {
		ctx = newCtx
	}

	// Set up the logger in the context if it is available
	if copts.Logger != nil {
		ctx = log.WithLogger(ctx, copts.Logger)
	}

	// Set up the iostreams in the context if it is available
	if copts.IOStreams != nil {
		ctx = iostreams.WithIOStreams(ctx, copts.IOStreams)
	}

	// Add the kraftkit version to the debug logs
	log.G(ctx).Debugf("kraftkit %s", kitversion.Version())

	if !config.G[config.KraftKit](ctx).NoCheckUpdates {
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

	if err := bootstrap.InitKraftkit(ctx); err != nil {
		log.G(ctx).Errorf("could not init kraftkit: %v", err)
		os.Exit(1)
	}

	return cmdfactory.Main(ctx, cmd)
}
