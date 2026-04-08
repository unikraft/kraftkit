// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package push

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/cloud/utils"
	"kraftkit.sh/log"
	"kraftkit.sh/pack"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/tui/paraprogress"
	"kraftkit.sh/unikraft/app"
)

type PushOptions struct {
	AllowInsecure bool   `local:"true" long:"allow-insecure" usage:"Allow insecure connections to the registry" hidden:"true"`
	Format        string `local:"true" long:"as" short:"M" usage:"Force the packaging despite possible conflicts" default:"auto"`
	Kraftfile     string `long:"kraftfile" short:"K" usage:"Set an alternative path of the Kraftfile"`
	Metro         string `local:"true" long:"metro" env:"UKC_METRO" usage:"Unikraft Cloud metro location" hidden:"true"`
	Token         string `local:"true" long:"token" env:"UKC_TOKEN" usage:"Unikraft Cloud access token" hidden:"true"`
}

// Push a Unikraft component.
func Push(ctx context.Context, opts *PushOptions, args ...string) error {
	if opts == nil {
		opts = &PushOptions{}
	}

	return opts.Run(ctx, args)
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&PushOptions{}, cobra.Command{
		Short:   "Push a Unikraft unikernel package to registry",
		Use:     "push [FLAGS] [PACKAGE]",
		Aliases: []string{"ph"},
		Long: heredoc.Doc(`
			Push a Unikraft unikernel, component microlibrary to a remote location
		`),
		Example: heredoc.Doc(`
			# Push the image for a project in the current directory
			$ kraft pkg push

			# Push the image for a project at a path with tag latest
			$ kraft pkg push /path/to/app

			# Push the image with a given name
			$ kraft pkg push unikraft.org/helloworld:latest
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "pkg",
		},
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *PushOptions) Pre(cmd *cobra.Command, _ []string) error {
	// Suppress interactive metro prompting: pkg commands do not require a
	// metro; token population proceeds silently via flags/env vars only.
	origNoPrompt := config.G[config.KraftKit](cmd.Context()).NoPrompt
	config.G[config.KraftKit](cmd.Context()).NoPrompt = true
	if err := utils.PopulateMetroToken(cmd, &opts.Metro, &opts.Token, &opts.AllowInsecure); err != nil {
		log.G(cmd.Context()).WithError(err).Debug("could not populate metro/token for pkg push")
	}
	config.G[config.KraftKit](cmd.Context()).NoPrompt = origNoPrompt

	if opts.Token != "" {
		if _, err := config.GetKraftCloudAuthConfig(cmd.Context(), opts.Token); err != nil {
			log.G(cmd.Context()).WithError(err).Debug("could not hydrate kraft cloud auth from token")
		}
		if opts.Metro != "" {
			cmd.SetContext(config.ContextWithIndexAuth(cmd.Context(), opts.Metro, opts.Token))
		}
	}

	ctx, err := packmanager.WithDefaultUmbrellaManagerInContext(cmd.Context())
	if err != nil {
		return err
	}

	cmd.SetContext(ctx)

	return nil
}

func (opts *PushOptions) Run(ctx context.Context, args []string) error {
	var err error
	var workdir string

	if len(args) == 0 {
		workdir, err = os.Getwd()
		if err != nil {
			return err
		}
	} else if f, err := os.Stat(args[0]); err == nil && f.IsDir() {
		workdir = args[0]
	} else {
		workdir = ""
	}

	norender := log.LoggerTypeFromString(config.G[config.KraftKit](ctx).Log.Type) != log.FANCY
	ref := ""
	if workdir != "" {
		popts := []app.ProjectOption{
			app.WithProjectWorkdir(workdir),
		}

		if len(opts.Kraftfile) > 0 {
			popts = append(popts, app.WithProjectKraftfile(opts.Kraftfile))
		} else {
			popts = append(popts, app.WithProjectDefaultKraftfiles())
		}

		// Read the kraft yaml specification and get the target name
		project, err := app.NewProjectFromOptions(ctx, popts...)
		if err != nil {
			return err
		}

		// Get the target name
		ref = project.Name()
	} else {
		// Argument is a reference name
		ref = args[0]
	}

	if !strings.Contains(ref, ":") {
		ref = fmt.Sprintf("%s:latest", ref)
	}

	var pm packmanager.PackageManager
	if opts.Format != "auto" {
		umbrella, err := packmanager.PackageManagers()
		if err != nil {
			return err
		}
		pm = umbrella[pack.PackageFormat(opts.Format)]
		if pm == nil {
			return errors.New("invalid package format specified")
		}
	} else {
		pm = packmanager.G(ctx)
	}

	pm, compatible, err := pm.IsCompatible(ctx, ref)
	if err != nil {
		return fmt.Errorf("package manager is not compatible: %w", err)
	} else if !compatible {
		return fmt.Errorf("package manager is not compatible")
	}

	packages, err := pm.Catalog(ctx,
		packmanager.WithRemote(false),
		packmanager.WithName(ref),
	)
	if err != nil {
		return err
	}

	if len(packages) == 0 {
		return errors.New("no packages found")
	}

	var processes []*paraprogress.Process

	for _, p := range packages {
		p := p

		processes = append(processes, paraprogress.NewProcess(
			fmt.Sprintf(
				"pushing (%s)",
				humanize.Bytes(uint64(p.Size())),
			),
			func(ctx context.Context, w func(progress float64)) error {
				return p.Push(ctx,
					pack.WithPushProgressFunc(w),
					pack.WithPushAuthConfig(config.G[config.KraftKit](ctx).Auth),
				)
			},
		))
	}

	model, err := paraprogress.NewParaProgress(
		ctx,
		processes,
		paraprogress.IsParallel(!config.G[config.KraftKit](ctx).NoParallel),
		paraprogress.WithRenderer(norender),
		paraprogress.WithFailFast(true),
	)
	if err != nil {
		return err
	}

	return model.Start()
}
