// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package prepare

import (
	"os"

	"github.com/MakeNowJust/heredoc"
	"github.com/juju/errors"
	"github.com/spf13/cobra"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/unikraft/app"
	"kraftkit.sh/unikraft/target"
)

type Prepare struct {
	Architecture string `long:"arch" short:"m" usage:"Filter prepare based on a target's architecture"`
	Kraftfile    string `long:"kraftfile" usage:"Set an alternative path of the Kraftfile"`
	Platform     string `long:"plat" short:"p" usage:"Filter prepare based on a target's platform"`
	Target       string `long:"target" short:"t" usage:"Filter prepare based on a specific target"`
}

func New() *cobra.Command {
	cmd, err := cmdfactory.New(&Prepare{}, cobra.Command{
		Short:   "Prepare a Unikraft unikernel",
		Use:     "prepare [DIR]",
		Aliases: []string{"p"},
		Args:    cmdfactory.MaxDirArgs(1),
		Long: heredoc.Doc(`
			prepare a Unikraft unikernel`),
		Example: heredoc.Doc(`
			# Prepare the cwd project
			$ kraft prepare

			# Prepare a project at a path
			$ kraft prepare path/to/app`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "build",
		},
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (*Prepare) Pre(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	pm, err := packmanager.NewUmbrellaManager(ctx)
	if err != nil {
		return err
	}

	cmd.SetContext(packmanager.WithPackageManager(ctx, pm))

	return nil
}

func (opts *Prepare) Run(cmd *cobra.Command, args []string) error {
	var err error

	ctx := cmd.Context()
	workdir := ""

	if len(args) == 0 {
		workdir, err = os.Getwd()
		if err != nil {
			return err
		}
	} else {
		workdir = args[0]
	}

	popts := []app.ProjectOption{
		app.WithProjectWorkdir(workdir),
	}

	if len(opts.Kraftfile) > 0 {
		popts = append(popts, app.WithProjectKraftfile(opts.Kraftfile))
	} else {
		popts = append(popts, app.WithProjectDefaultKraftfiles())
	}

	// Initialize at least the configuration options for a project
	project, err := app.NewProjectFromOptions(ctx, popts...)
	if err != nil {
		return err
	}

	// Filter project targets by any provided CLI options
	targets := cli.FilterTargets(
		project.Targets(),
		opts.Architecture,
		opts.Platform,
		opts.Target,
	)

	var t target.Target

	switch {
	case len(targets) == 1:
		t = targets[0]

	case config.G[config.KraftKit](ctx).NoPrompt:
		return errors.New("could not determine which target to prepare")

	default:
		t, err = cli.SelectTarget(targets)
		if err != nil {
			return err
		}
	}

	return project.Prepare(ctx, t)
}
