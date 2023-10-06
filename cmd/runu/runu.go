// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package main

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

	"kraftkit.sh/cmd/runu/create"
	"kraftkit.sh/cmd/runu/delete"
	"kraftkit.sh/cmd/runu/kill"
	"kraftkit.sh/cmd/runu/ps"
	"kraftkit.sh/cmd/runu/start"
	"kraftkit.sh/cmd/runu/state"
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

func New(cfg *config.ConfigManager[config.KraftKit]) *cobra.Command {
	cmd, err := cmdfactory.New(&Runu{}, cobra.Command{
		Short: "Run OCI-compatible unikernels",
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
			HiddenDefaultCmd:  true,
		},
	}, cfg)
	if err != nil {
		panic(err)
	}

	cmd.AddCommand(state.New(cfg))
	cmd.AddCommand(create.New(cfg))
	cmd.AddCommand(start.New(cfg))
	cmd.AddCommand(kill.New(cfg))
	cmd.AddCommand(delete.New(cfg))
	cmd.AddCommand(ps.New(cfg))

	return cmd
}

func (opts *Runu) PersistentPre(cmd *cobra.Command, _ []string, _ *config.ConfigManager[config.KraftKit]) error {
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

func (*Runu) Run(cmd *cobra.Command, args []string, cfgMgr *config.ConfigManager[config.KraftKit]) error {
	return cmd.Help()
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	copts := &cli.CliOptions{}

	// TODO(jake-ciolek): We use the command tree to hydrate config.
	//                    We use config to hydrate the command tree.
	//                    This results in a circulard initialization dependency.
	//                    Right now, we'll instantiate a short-lived, unhydrated
	//                    command tree to initialize a config manager.
	//                    Then, we'll use that to obtain the final hydrated command tree.
	//                    This will do for now, but spend some time thinking about how to make it nicer.
	cmdInit := New(nil)
	_, args, err := cmdInit.Find(os.Args[1:])
	if err != nil {
		log.G(ctx).Error(err)
	}

	cfgMgr, err := cli.ConfigManagerFromArgs(args)
	if err != nil {
		log.G(ctx).Error(err)
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
			fmt.Println(err)
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

	cmdfactory.Main(ctx, cmd)
}
