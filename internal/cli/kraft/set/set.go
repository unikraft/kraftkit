// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package set

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/spellcheck"
	"kraftkit.sh/kconfig"
	"kraftkit.sh/log"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/tui/confirm"
	"kraftkit.sh/unikraft/app"
	"kraftkit.sh/unikraft/target"
)

type SetOptions struct {
	Architecture string `long:"arch" short:"m" usage:"Filter targets by architecture"`
	Force        bool   `long:"force" short:"F" usage:"Skip KConfig validation"`
	Kraftfile    string `long:"kraftfile" short:"K" usage:"Set an alternative path of the Kraftfile"`
	Platform     string `long:"plat" short:"p" usage:"Filter targets by platform"`
	Target       string `long:"target" short:"t" usage:"Set config for a specific target"`
	Workdir      string `long:"workdir" short:"w" usage:"Work on a unikernel at a path"`
}

// Set a KConfig variable in a Unikraft project.
func Set(ctx context.Context, opts *SetOptions, args ...string) error {
	if opts == nil {
		opts = &SetOptions{}
	}

	return opts.Run(ctx, args)
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&SetOptions{}, cobra.Command{
		Short:   "Set a variable for a Unikraft project",
		Use:     "set [OPTIONS] [param=value ...]",
		Aliases: []string{"s"},
		Long: heredoc.Doc(`
			Set a variable for a Unikraft project.
		`),
		Example: heredoc.Doc(`
			# Set variables in the cwd project
			$ kraft set LIBDEVFS_DEV_STDOUT=/dev/null LWIP_TCP_SND_BUF=4096

			# Set variables in a project at a path
			$ kraft set -w path/to/app LIBDEVFS_DEV_STDOUT=/dev/null LWIP_TCP_SND_BUF=4096

			# Set variables for a specific target
			$ kraft set --plat=qemu --arch=x86_64 CONFIG_LIBUKDEBUG=y
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

func (*SetOptions) Pre(cmd *cobra.Command, _ []string) error {
	ctx, err := packmanager.WithDefaultUmbrellaManagerInContext(cmd.Context())
	if err != nil {
		return err
	}

	cmd.SetContext(ctx)

	return nil
}

func (opts *SetOptions) Run(ctx context.Context, args []string) error {
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
		return fmt.Errorf("no options to set")
	}

	confOpts := []string{}

	for _, arg := range args {
		if !strings.ContainsRune(arg, '=') || strings.HasSuffix(arg, "=") {
			return fmt.Errorf("invalid or malformed argument: %s", arg)
		}

		confOpts = append(confOpts, arg)
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
	for _, opt := range confOpts {
		if split := strings.SplitN(opt, "=", 2); len(split) == 2 {
			extraConfig.Set(split[0], split[1])
		}
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

		// Validate user-provided config metadata against known KConfig symbols
		if !opts.Force {
			configPath := filepath.Join(workdir, tc.ConfigFilename())
			knownConfigs, err := spellcheck.AllKConfigSymbols(configPath)
			if err == nil {
				results := spellcheck.Validate(confOpts, knownConfigs)
				if len(results) > 0 {
					for _, r := range results {
						log.G(ctx).Error(FormatSpellCheckWarning(r))
					}
					return fmt.Errorf("kconfig validation failed")
				}
			}
		}

		// Apply the user's config values (this also generates config if needed)
		if err := project.Set(ctx, tc, extraConfig); err != nil {
			return fmt.Errorf("setting config for target %s: %w", tc.Name(), err)
		}
		log.G(ctx).Infof("Config applied to target: %s", tc.Name())
	}

	return nil
}

// FormatSpellCheckWarning returns a human-readable warning string for a spellcheck Result.
func FormatSpellCheckWarning(r spellcheck.Result) string {
	if r.Suggestion != "" {
		return fmt.Sprintf(
			"unknown KConfig option %s: did you mean %s? Use kraft set --force %s to override",
			r.Option, r.Suggestion, r.Option,
		)
	}

	return fmt.Sprintf(
		"unknown KConfig option %s: Use kraft set --force %s to override",
		r.Option, r.Option,
	)
}
