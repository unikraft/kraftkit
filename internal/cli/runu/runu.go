// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package runu

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"

	"kraftkit.sh/internal/cli/runu/create"
	"kraftkit.sh/internal/cli/runu/delete"
	"kraftkit.sh/internal/cli/runu/kill"
	"kraftkit.sh/internal/cli/runu/ps"
	"kraftkit.sh/internal/cli/runu/start"
	"kraftkit.sh/internal/cli/runu/state"
)

// [OCI runtime] for unikernels. Implements the runc [command-line interface].
//
// [OCI runtime]: https://github.com/opencontainers/runtime-spec/blob/v1.1.0/runtime.md
// [command-line interface]: https://github.com/opencontainers/runtime-tools/blob/v0.9.0/docs/command-line-interface.md
type Runu struct {
	Root          string `long:"root" usage:"Root directory for storage of unikernel state" default:"/run/runu"`
	Log           string `long:"log" usage:"Set the log file path where internal debug information is written" default:"/run/runu/runu.log"`
	LogFormat     string `long:"log-format" usage:"set the format used by logs" default:"text"`
	SystemdCgroup bool   `long:"systemd-cgroup" usage:"enable systemd cgroup support"`
}

func New() *cobra.Command {
	cmd, err := cmdfactory.New(&Runu{}, cobra.Command{
		Short: "Run OCI-compatible unikernels",
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
			HiddenDefaultCmd:  true,
		},
	})
	if err != nil {
		panic(err)
	}

	cmd.AddCommand(state.New())
	cmd.AddCommand(create.New())
	cmd.AddCommand(start.New())
	cmd.AddCommand(kill.New())
	cmd.AddCommand(delete.New())
	cmd.AddCommand(ps.New())

	return cmd
}

func (opts *Runu) PersistentPre(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	if opts.Root == "" {
		return fmt.Errorf("--root cannot be empty")
	}

	if opts.Log != "" {
		f, err := os.OpenFile(opts.Log, os.O_CREATE|os.O_WRONLY|os.O_APPEND|os.O_SYNC, 0o644)
		if err != nil {
			return err
		}

		log.G(ctx).SetOutput(f)
	}

	if opts.LogFormat == "json" {
		log.G(ctx).SetFormatter(new(logrus.JSONFormatter))
	}

	return nil
}

func (*Runu) Run(cmd *cobra.Command, args []string) error {
	return cmd.Help()
}

func Main(args []string) int {
	cmd := New()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

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
			return 1
		}
	}

	// Set up the config manager in the context if it is available
	if copts.ConfigManager != nil {
		ctx = config.WithConfigManager(ctx, copts.ConfigManager)
	}

	// Set up the logger in the context if it is available
	if copts.Logger != nil {
		ctx = log.WithLogger(ctx, copts.Logger)
	}

	// Set up the iostreams in the context if it is available
	if copts.IOStreams != nil {
		ctx = iostreams.WithIOStreams(ctx, copts.IOStreams)
	}

	return cmdfactory.Main(ctx, cmd)
}
