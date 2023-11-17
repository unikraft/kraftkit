// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package build

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/machine/platform"

	"kraftkit.sh/log"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/unikraft/app"
	"kraftkit.sh/unikraft/target"
)

type BuildOptions struct {
	All          bool   `long:"all" usage:"Build all targets"`
	Architecture string `long:"arch" short:"m" usage:"Filter the creation of the build by architecture of known targets"`
	DotConfig    string `long:"config" short:"c" usage:"Override the path to the KConfig .config file"`
	ForcePull    bool   `long:"force-pull" usage:"Force pulling packages before building"`
	Jobs         int    `long:"jobs" short:"j" usage:"Allow N jobs at once"`
	KernelDbg    bool   `long:"dbg" usage:"Build the debuggable (symbolic) kernel image instead of the stripped image"`
	Kraftfile    string `long:"kraftfile" short:"K" usage:"Set an alternative path of the Kraftfile"`
	NoCache      bool   `long:"no-cache" short:"F" usage:"Force a rebuild even if existing intermediate artifacts already exist"`
	NoConfigure  bool   `long:"no-configure" usage:"Do not run Unikraft's configure step before building"`
	NoFast       bool   `long:"no-fast" usage:"Do not use maximum parallelization when performing the build"`
	NoFetch      bool   `long:"no-fetch" usage:"Do not run Unikraft's fetch step before building"`
	NoUpdate     bool   `long:"no-update" usage:"Do not update package index before running the build"`
	Platform     string `long:"plat" short:"p" usage:"Filter the creation of the build by platform of known targets"`
	Rootfs       string `long:"rootfs" usage:"Specify a path to use as root file system (can be volume or initramfs)"`
	SaveBuildLog string `long:"build-log" usage:"Use the specified file to save the output from the build"`
	Target       string `long:"target" short:"t" usage:"Build a particular known target"`

	project app.Application
	workdir string
}

// Build a Unikraft unikernel.
func Build(ctx context.Context, opts *BuildOptions, args ...string) error {
	if opts == nil {
		opts = &BuildOptions{}
	}
	return opts.Run(ctx, args)
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&BuildOptions{}, cobra.Command{
		Short: "Configure and build Unikraft unikernels",
		Use:   "build [FLAGS] [SUBCOMMAND|DIR]",
		Args:  cmdfactory.MaxDirArgs(1),
		Long: heredoc.Docf(`
			Build a Unikraft unikernel.

			The default behaviour of %[1]skraft build%[1]s is to build a project.  Given no
			arguments, you will be guided through interactive mode.
		`, "`"),
		Example: heredoc.Doc(`
			# Build the current project (cwd)
			$ kraft build

			# Build path to a Unikraft project
			$ kraft build path/to/app`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "build",
		},
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *BuildOptions) Pre(cmd *cobra.Command, args []string) error {
	ctx, err := packmanager.WithDefaultUmbrellaManagerInContext(cmd.Context())
	if err != nil {
		return err
	}

	cmd.SetContext(ctx)

	if len(args) == 0 {
		opts.workdir, err = os.Getwd()
		if err != nil {
			return err
		}
	} else {
		opts.workdir = args[0]
	}

	popts := []app.ProjectOption{
		app.WithProjectWorkdir(opts.workdir),
	}

	if len(opts.Kraftfile) > 0 {
		popts = append(popts, app.WithProjectKraftfile(opts.Kraftfile))
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

	opts.Platform = platform.PlatformByName(opts.Platform).String()

	return nil
}

func (opts *BuildOptions) Run(ctx context.Context, args []string) error {
	// Filter project targets by any provided CLI options
	selected := opts.project.Targets()
	if len(selected) == 0 {
		return fmt.Errorf("no targets to build")
	}
	if !opts.All {
		selected = target.Filter(
			selected,
			opts.Architecture,
			opts.Platform,
			opts.Target,
		)

		if !config.G[config.KraftKit](ctx).NoPrompt {
			res, err := target.Select(selected)
			if err != nil {
				return err
			}
			selected = []target.Target{res}
		}
	}

	if len(selected) == 0 {
		return fmt.Errorf("no targets selected to build")
	}

	var build builder
	builders := builders()

	// Iterate through the list of built-in builders which sequentially tests
	// the current context and Kraftfile match specific requirements towards
	// performing a type of build.
	for _, candidate := range builders {
		log.G(ctx).
			WithField("builder", candidate.String()).
			Trace("checking buildability")

		capable, err := candidate.Buildable(ctx, opts, args...)
		if capable && err == nil {
			build = candidate
			break
		}
	}

	if build == nil {
		return fmt.Errorf("could not determine what or how to build from the given context")
	}

	log.G(ctx).WithField("builder", build.String()).Debug("using")

	if err := build.Prepare(ctx, opts, selected, args...); err != nil {
		return fmt.Errorf("could not complete build: %w", err)
	}

	if err := opts.buildRootfs(ctx); err != nil {
		return err
	}

	if err := build.Build(ctx, opts, selected, args...); err != nil {
		return fmt.Errorf("could not complete build: %w", err)
	}

	return nil
}
