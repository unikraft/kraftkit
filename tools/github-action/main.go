// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/rancher/wrangler/pkg/signals"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/log"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/unikraft/app"
	"kraftkit.sh/unikraft/target"

	_ "kraftkit.sh/manifest"
	_ "kraftkit.sh/oci"
)

type GithubAction struct {
	// Input arguments for the action
	// Global flags
	Loglevel string `long:"loglevel" env:"INPUT_LOGLEVEL" usage:"" default:"info"`

	// Project flags
	Workdir   string `long:"workdir" env:"INPUT_WORKDIR" usage:"Path to working directory (default is cwd)"`
	Kraftfile string `long:"kraftfile" env:"INPUT_KRAFTFILE" usage:"Path to Kraftfile or contents of Kraftfile"`

	// Build flags
	Arch   string `long:"arch" env:"INPUT_ARCH" usage:"Architecture to build for"`
	Plat   string `long:"plat" env:"INPUT_PLAT" usage:"Platform to build for"`
	Target string `long:"target" env:"INPUT_TARGET" usage:"Name of the target to build for"`

	// Running flags
	Execute bool   `long:"execute" env:"INPUT_EXECUTE" usage:"If to run the unikernel"`
	Timeout uint64 `long:"timeout" env:"INPUT_TIMEOUT" usage:"Timeout for the unikernel"`

	// Packaging flags
	Args    []string `long:"args" env:"INPUT_ARGS" usage:"Arguments to pass to the unikernel"`
	InitRd  string   `long:"initrd" env:"INPUT_INITRD" usage:"Include an initrd at path"`
	Memory  string   `long:"memory" env:"INPUT_MEMORY" usage:"Set the memory size"`
	Name    string   `long:"name" env:"INPUT_NAME" usage:"Set the name of the output"`
	Output  string   `long:"output" env:"INPUT_OUTPUT" usage:"Set the output path"`
	Kconfig bool     `long:"kconfig" env:"INPUT_KCONFIG" usage:"Include all set KConfig with the output"`
	Push    bool     `long:"push" env:"INPUT_PUSH" usage:"Push the output"`

	// Internal attributes
	project app.Application
	target  target.Target
}

func (opts *GithubAction) Pre(cmd *cobra.Command, args []string) (err error) {
	if (len(opts.Arch) > 0 || len(opts.Plat) > 0) && len(opts.Target) > 0 {
		return fmt.Errorf("target and platform/architecture are mutually exclusive")
	}

	ctx := cmd.Context()

	switch opts.Loglevel {
	case "debug":
		log.G(ctx).SetLevel(logrus.DebugLevel)
	case "trace":
		log.G(ctx).SetLevel(logrus.TraceLevel)
	}

	pm, err := packmanager.NewUmbrellaManager(ctx)
	if err != nil {
		return err
	}

	cmd.SetContext(packmanager.WithPackageManager(ctx, pm))

	if len(opts.Workdir) == 0 {
		opts.Workdir, err = os.Getwd()
		if err != nil {
			return err
		}
	}

	popts := []app.ProjectOption{
		app.WithProjectWorkdir(opts.Workdir),
	}

	// Check if the provided Kraftfile is set, and whether it's either a path or
	// an inline file.
	if len(opts.Kraftfile) > 0 {
		if _, err := os.Stat(opts.Kraftfile); err == nil {
			popts = append(popts, app.WithProjectKraftfile(opts.Kraftfile))
		} else {
			// Dump the contents to a file
			fi, err := os.CreateTemp("", "*.Kraftfile")
			if err != nil {
				return fmt.Errorf("could not create temporary file for Kraftfile: %w", err)
			}

			defer fi.Close()

			n, err := fi.Write([]byte(opts.Kraftfile))
			if err != nil {
				return fmt.Errorf("could not write to temporary Kraftfile: %w", err)
			}

			if n != len(opts.Kraftfile) {
				return fmt.Errorf("could not write entire Kraftfile to %s", fi.Name())
			}

			popts = append(popts, app.WithProjectKraftfile(fi.Name()))
		}
	} else {
		popts = append(popts, app.WithProjectDefaultKraftfiles())
	}

	// Initialize at least the configuration options for a project
	opts.project, err = app.NewProjectFromOptions(ctx, popts...)
	if err != nil && errors.Is(err, app.ErrNoKraftfile) {
		return fmt.Errorf("cannot build project directory without a Kraftfile")
	} else if err != nil {
		return fmt.Errorf("could not initialize project directory: %w", err)
	}

	// Filter project targets by any provided input arguments
	targets := target.Filter(
		opts.project.Targets(),
		opts.Arch,
		opts.Plat,
		opts.Target,
	)

	if len(targets) != 1 {
		// TODO(nderjung): We should support building multiple targets in the
		// future, but for now we disable this ability.  This is largely to do with
		// package management afterwards which does not yet support multi-target
		// artifacts.  Once this is supported, we can enable multiple target-builds
		// (and packaging).  Moreover, since it is possible to also execute the
		// unikernel after a successful build via this action, multiple targets
		// would also fail at this step.
		return fmt.Errorf("cannot build more than one target using action")
	}

	opts.target = targets[0]

	// Infer arguments implicitly if there is only one target.  If we've made it
	// this far, `target.Filter` only had one target to choose from.
	if opts.Plat == "" {
		opts.Plat = opts.target.Platform().Name()
	}
	if opts.Arch == "" {
		opts.Arch = opts.target.Architecture().Name()
	}

	return nil
}

func (opts *GithubAction) Run(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	if err := opts.pull(ctx); err != nil {
		return fmt.Errorf("could not pull project components: %w", err)
	}

	if err := opts.build(ctx); err != nil {
		return fmt.Errorf("could not build unikernel: %w", err)
	}

	if opts.Execute {
		if err := opts.execute(ctx); err != nil {
			return fmt.Errorf("could not run unikernel: %w", err)
		}
	}

	if opts.Output != "" {
		if err := opts.packAndPush(ctx); err != nil {
			return fmt.Errorf("could not package unikernel: %w", err)
		}
	}

	return nil
}

func main() {
	cmd, err := cmdfactory.New(&GithubAction{}, cobra.Command{})
	if err != nil {
		fmt.Printf("prepare command: %w", err)
		os.Exit(1)
	}

	ctx := signals.SetupSignalContext()

	cfg, err := config.NewDefaultKraftKitConfig()
	if err != nil {
		fmt.Printf("could not prepare internal configuration: %w", err)
		os.Exit(1)
	}

	cfgm, err := config.NewConfigManager(cfg)
	if err != nil {
		fmt.Printf("could initialize config manager: %w", err)
		os.Exit(1)
	}

	// Set up the config manager in the context if it is available
	ctx = config.WithConfigManager(ctx, cfgm)

	cmd, args, err := cmd.Find(os.Args[1:])
	if err != nil {
		fmt.Printf("could not fing flag: %w", err)
		os.Exit(1)
	}

	if err := cmdfactory.AttributeFlags(cmd, cfg, args...); err != nil {
		fmt.Printf("could not attribute flags: %w", err)
		os.Exit(1)
	}

	// Set up a default logger based on the internal TextFormatter
	logger := logrus.New()

	formatter := new(log.TextFormatter)
	formatter.ForceColors = true
	formatter.ForceFormatting = true
	formatter.FullTimestamp = true
	formatter.DisableTimestamp = true
	logger.Formatter = formatter

	// Set up the logger in the context if it is available
	ctx = log.WithLogger(ctx, logger)

	cmdfactory.Main(ctx, cmd)
}
