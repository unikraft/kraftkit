// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package kraft

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"runtime/debug"
	"runtime/pprof"
	"strings"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/getsentry/sentry-go"
	"github.com/rancher/wrangler/pkg/signals"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/bootstrap"
	"kraftkit.sh/internal/cli"
	kitupdate "kraftkit.sh/internal/update"
	kitversion "kraftkit.sh/internal/version"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"

	"kraftkit.sh/internal/cli/kraft/build"
	"kraftkit.sh/internal/cli/kraft/clean"
	"kraftkit.sh/internal/cli/kraft/cloud"
	"kraftkit.sh/internal/cli/kraft/compose"
	"kraftkit.sh/internal/cli/kraft/events"
	"kraftkit.sh/internal/cli/kraft/fetch"
	"kraftkit.sh/internal/cli/kraft/lib"
	"kraftkit.sh/internal/cli/kraft/login"
	"kraftkit.sh/internal/cli/kraft/logs"
	"kraftkit.sh/internal/cli/kraft/menu"
	"kraftkit.sh/internal/cli/kraft/net"
	"kraftkit.sh/internal/cli/kraft/pause"
	"kraftkit.sh/internal/cli/kraft/pkg"
	"kraftkit.sh/internal/cli/kraft/ps"
	"kraftkit.sh/internal/cli/kraft/remove"
	"kraftkit.sh/internal/cli/kraft/run"
	"kraftkit.sh/internal/cli/kraft/set"
	"kraftkit.sh/internal/cli/kraft/start"
	"kraftkit.sh/internal/cli/kraft/stop"
	"kraftkit.sh/internal/cli/kraft/system"
	"kraftkit.sh/internal/cli/kraft/toolchain"
	"kraftkit.sh/internal/cli/kraft/unset"
	"kraftkit.sh/internal/cli/kraft/version"
	"kraftkit.sh/internal/cli/kraft/volume"
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
     /|/=\|\   Documentation:    https://unikraft.org/docs/cli
    (_:| |:_)  Issues & support: https://github.com/unikraft/kraftkit/issues
       v v     Platform:         https://unikraft.cloud
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
	cmd.AddCommand(pause.NewCmd())

	cmd.AddGroup(&cobra.Group{ID: "net", Title: "LOCAL NETWORKING COMMANDS"})
	cmd.AddCommand(net.NewCmd())

	cmd.AddGroup(&cobra.Group{ID: "vol", Title: "LOCAL VOLUME COMMANDS"})
	cmd.AddCommand(volume.NewCmd())

	cmd.AddGroup(&cobra.Group{ID: "compose", Title: "COMPOSE COMMANDS"})
	cmd.AddCommand(compose.NewCmd())

	cmd.AddGroup(&cobra.Group{ID: "kraftcloud", Title: "UNIKRAFT CLOUD COMMANDS"})
	cmd.AddCommand(cloud.NewCmd())

	cmd.AddGroup(&cobra.Group{ID: "kraftcloud-image", Title: "UNIKRAFT CLOUD IMAGE COMMANDS"})
	cmd.AddGroup(&cobra.Group{ID: "kraftcloud-instance", Title: "UNIKRAFT CLOUD INSTANCE COMMANDS"})
	cmd.AddGroup(&cobra.Group{ID: "kraftcloud-volume", Title: "UNIKRAFT CLOUD VOLUME COMMANDS"})
	cmd.AddGroup(&cobra.Group{ID: "kraftcloud-volume-template", Title: "UNIKRAFT CLOUD VOLUME TEMPLATE COMMANDS"})
	cmd.AddGroup(&cobra.Group{ID: "kraftcloud-service", Title: "UNIKRAFT CLOUD SERVICE COMMANDS"})
	cmd.AddGroup(&cobra.Group{ID: "kraftcloud-scale", Title: "UNIKRAFT CLOUD AUTOSCALE COMMANDS"})
	cmd.AddGroup(&cobra.Group{ID: "kraftcloud-certificate", Title: "UNIKRAFT CLOUD CERTIFICATE COMMANDS"})
	cmd.AddGroup(&cobra.Group{ID: "kraftcloud-compose", Title: "UNIKRAFT CLOUD COMPOSE COMMANDS"})

	cmd.AddGroup(&cobra.Group{ID: "misc", Title: "MISCELLANEOUS COMMANDS"})
	cmd.AddCommand(login.NewCmd())
	cmd.AddCommand(version.NewCmd())
	cmd.AddCommand(system.NewCmd())
	cmd.AddCommand(x.NewCmd())
	cmd.AddCommand(toolchain.NewCmd())

	return cmd
}

func (k *KraftOptions) Run(_ context.Context, args []string) error {
	return pflag.ErrHelp
}

// The Sentry DSN to use for anonymous telemetry.
var sentryDsn = ""

func Main(args []string) int {
	cmd := NewCmd()
	ctx := signals.SetupSignalContext()
	copts := &cli.CliOptions{}

	// Start CPU profile when environmental variable is set.
	if cpuProfile := os.Getenv("KRAFTKIT_CPU_PROFILE"); len(cpuProfile) > 0 {
		pprofFile, err := os.OpenFile(cpuProfile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
		if err != nil {
			fmt.Println("could not create cpu profile file")
			os.Exit(1)
		}

		// Start profiling
		if err = pprof.StartCPUProfile(pprofFile); err != nil {
			fmt.Println("could not start cpu profiling")
			os.Exit(1)
		}

		// Stop profiling on exit
		defer func() {
			pprof.StopCPUProfile()
			_ = pprofFile.Close()
		}()
	}

	for _, o := range []cli.CliOption{
		cli.WithDefaultConfigManager(cmd),
		cli.WithDefaultIOStreams(),
		cli.WithDefaultLogger(),
		cli.WithDefaultHTTPClient(),
	} {
		if err := o(copts); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}

	// Set up the config manager in the context if it is available
	ctx = config.WithConfigManager(ctx, copts.ConfigManager)

	// Hydrate KraftCloud configuration
	if newCtx, err := config.HydrateKraftCloudAuthInContext(ctx); err == nil {
		ctx = newCtx
	}

	// Set up the logger in the context if it is available
	ctx = log.WithLogger(ctx, copts.Logger)

	// Add the kraftkit version to the debug logs
	log.G(ctx).
		WithField("version", kitversion.Version()).
		Debugf("kraftkit")

	collectTelemetry := sentryDsn != "" && config.G[config.KraftKit](ctx).CollectAnonymousTelemetry

	if collectTelemetry {
		sentryDsn, err := base64.StdEncoding.DecodeString(sentryDsn)
		if err != nil {
			collectTelemetry = false
		} else {
			if err := sentry.Init(sentry.ClientOptions{
				Dsn:              string(sentryDsn),
				Release:          kitversion.Version(),
				TracesSampleRate: 1.0,
			}); err != nil {
				log.G(ctx).Debugf("could not initialize sentry: %v", err)
			} else {
				log.G(ctx).Debug("collecting anonymous telemetry - to disable export KRAFTKIT_COLLECT_ANONYMOUS_TELEMETRY=false")
			}
		}
	}

	defer func() {
		if err := recover(); err != nil {
			if collectTelemetry {
				sentry.CurrentHub().RecoverWithContext(ctx, err)
				sentry.Flush(time.Second * 5)
			}

			// Use copts.Logger as access to log.G(ctx) may not be available in the
			// panic state.
			copts.Logger.Logf(logrus.FatalLevel, "a fatal error occurred: %s", err)

			recoverLevel := logrus.DebugLevel
			if !collectTelemetry {
				recoverLevel = logrus.ErrorLevel
			}

			// Only log the stack trace if the user has opted-out of telemetry such
			// that they can see the stack trace and report the issue.  Otherwise,
			// silently log these to the debug level and suggest the user to open an
			// issue.
			for _, line := range strings.Split(string(debug.Stack()), "\n") {
				copts.Logger.Log(recoverLevel, line)
			}
			if !collectTelemetry {
				copts.Logger.Log(logrus.FatalLevel, "please consider opening an issue at: https://github.com/unikraft/kraftkit/issues/new?template=bug_report.yml")
			}
		}
	}()

	// Set up the iostreams in the context if it is available
	if copts.IOStreams != nil {
		ctx = iostreams.WithIOStreams(ctx, copts.IOStreams)
	}

	if (os.Getenv("SUDO_UID") != "" || os.Getenv("SUDO_GID") != "" || os.Getenv("SUDO_USER") != "") && !config.G[config.KraftKit](ctx).NoWarnSudo {
		log.G(ctx).Warn("detected invocation via sudo!")
		log.G(ctx).Warn("")
		log.G(ctx).Warn("mixing invocations of kraft with sudo can lead to unexpected behavior")
		log.G(ctx).Warn("read more on how to use kraft without sudo at:")
		log.G(ctx).Warn("")
		log.G(ctx).Warn("\thttps://unikraft.org/sudoless")
		log.G(ctx).Warn("")
		log.G(ctx).Warn("to hide and ignore this warning message, set the environmental variable:")
		log.G(ctx).Warn("")
		log.G(ctx).Warn("\texport KRAFTKIT_NO_WARN_SUDO=1")
		log.G(ctx).Warn("")
	}

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
