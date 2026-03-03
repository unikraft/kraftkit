// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package unset

import (
	"context"
	"fmt"
	"os"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/kconfig"
	"kraftkit.sh/log"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/tui/confirm"
	"kraftkit.sh/unikraft/app"
	"kraftkit.sh/unikraft/target"
)

type UnsetOptions struct {
	Architecture string `long:"arch" short:"m" usage:"Filter targets by architecture"`
	Kraftfile    string `long:"kraftfile" short:"K" usage:"Set an alternative path of the Kraftfile"`
	Platform     string `long:"plat" short:"p" usage:"Filter targets by platform"`
	Target       string `long:"target" short:"t" usage:"Unset config for a specific target"`
	Workdir      string `long:"workdir" short:"w" usage:"Work on a unikernel at a path"`
}

// Unset a KConfig option in a Unikraft project.
func Unset(ctx context.Context, opts *UnsetOptions, args ...string) error {
	if opts == nil {
		opts = &UnsetOptions{}
	}

	return opts.Run(ctx, args)
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&UnsetOptions{}, cobra.Command{
		Short:   "Unset a variable for a Unikraft project",
		Use:     "unset [OPTIONS] [param ...]",
		Aliases: []string{"u"},
		Long: heredoc.Doc(`
			Unset a variable for a Unikraft project.
		`),
		Example: heredoc.Doc(`
			# Unset variables in the cwd project
			$ kraft unset LIBDEVFS_DEV_STDOUT LWIP_TCP_SND_BUF

			# Unset variables in a project at a path
			$ kraft unset -w path/to/app LIBDEVFS_DEV_STDOUT LWIP_TCP_SND_BUF

			# Unset variables for a specific target
			$ kraft unset --plat=qemu --arch=x86_64 CONFIG_LIBUKDEBUG
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "build",
		},
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (*UnsetOptions) Pre(cmd *cobra.Command, _ []string) error {
	ctx, err := packmanager.WithDefaultUmbrellaManagerInContext(cmd.Context())
	if err != nil {
		return err
	}

	cmd.SetContext(ctx)

	return nil
}

func (opts *UnsetOptions) Run(ctx context.Context, args []string) error {
	var err error

	workdir := ""

	if opts.Workdir != "" {
		workdir = opts.Workdir
	} else {
		workdir, err = os.Getwd()
		if err != nil {
			return err
		}
	}

	if len(args) == 0 {
		return fmt.Errorf("no options to unset")
	}

	confOpts := []string{}
	for _, arg := range args {
		confOpts = append(confOpts, arg+"=n")
	}

	// Load the project
	popts := []app.ProjectOption{
		app.WithProjectWorkdir(workdir),
		app.WithProjectConfig(confOpts),
	}

	if len(opts.Kraftfile) > 0 {
		popts = append(popts, app.WithProjectKraftfile(opts.Kraftfile))
	} else {
		popts = append(popts, app.WithProjectDefaultKraftfiles())
	}

	project, err := app.NewProjectFromOptions(ctx, popts...)
	if err != nil {
		return err
	}

	// Prepare the extra config map from user input
	extraConfig := kconfig.KeyValueMap{}
	for _, arg := range args {
		extraConfig.Set(arg, "n")
	}

	// Filter targets based on platform/arch/target flags
	selected := target.Filter(
		project.Targets(),
		opts.Architecture,
		opts.Platform,
		opts.Target,
	)

	if len(selected) == 0 {
		return fmt.Errorf("no targets match the specified criteria")
	}

	// If multiple targets and prompting is enabled, let user select
	if len(selected) > 1 && !config.G[config.KraftKit](ctx).NoPrompt {
		tc, err := target.Select(selected)
		if err != nil {
			return err
		}
		selected = []target.Target{tc}
	}

	for _, tc := range selected {
		// If not configured, prompt user for confirmation before generating
		if !project.IsConfigured(tc) {
			if !config.G[config.KraftKit](ctx).NoPrompt {
				generate, err := confirm.NewConfirm("No configuration found, generate default config?")
				if err != nil {
					return err
				}
				if !generate {
					log.G(ctx).Infof("Skipping target: %s", tc.Name())
					continue
				}
			}
		}

		// Apply the unset config values (this also generates config if needed)
		if err := project.Unset(ctx, tc, extraConfig); err != nil {
			return fmt.Errorf("unsetting config for target %s: %w", tc.Name(), err)
		}
		log.G(ctx).Infof("Config applied to target: %s", tc.Name())
	}

	return nil
}
