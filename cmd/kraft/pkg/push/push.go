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

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/log"
	"kraftkit.sh/pack"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/tui/paraprogress"
	"kraftkit.sh/unikraft/app"
)

type Push struct {
	Format    string `local:"true" long:"as" short:"M" usage:"Force the packaging despite possible conflicts" default:"auto"`
	Kraftfile string `long:"kraftfile" usage:"Set an alternative path of the Kraftfile"`
}

func New() *cobra.Command {
	cmd, err := cmdfactory.New(&Push{}, cobra.Command{
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
			$ kraft pkg push unikraft.org/helloworld:latest`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "pkg",
		},
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *Push) Pre(cmd *cobra.Command, _ []string) error {
	ctx, err := packmanager.WithDefaultUmbrellaManagerInContext(cmd.Context())
	if err != nil {
		return err
	}

	cmd.SetContext(ctx)

	return nil
}

func (opts *Push) Run(cmd *cobra.Command, args []string) error {
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

	ctx := cmd.Context()
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

	var pmananger packmanager.PackageManager
	if opts.Format != "auto" {
		umbrella, err := packmanager.PackageManagers()
		if err != nil {
			return err
		}
		pmananger = umbrella[pack.PackageFormat(opts.Format)]
		if pmananger == nil {
			return errors.New("invalid package format specified")
		}
	} else {
		pmananger = packmanager.G(ctx)
	}

	if pm, compatible, err := pmananger.IsCompatible(ctx, ref); err == nil && compatible {
		packages, err := pm.Catalog(ctx,
			packmanager.WithCache(true),
			packmanager.WithName(ref),
		)
		if err != nil {
			return err
		}

		if len(packages) == 0 {
			return errors.New("no packages found")
		} else if len(packages) > 1 {
			return errors.New("multiple packages found")
		}

		// Call push if it exists
		// TODO push if it doesn't exist too
		proc := paraprogress.NewProcess(
			fmt.Sprintf("pushing %s",
				ref,
			),
			func(ctx context.Context, w func(progress float64)) error {
				return packages[0].Push(
					ctx,
					pack.WithPushProgressFunc(w),
				)
			},
		)

		var processes []*paraprogress.Process
		processes = append(processes, proc)

		paramodel, err := paraprogress.NewParaProgress(
			ctx,
			processes,
			paraprogress.IsParallel(false),
			paraprogress.WithRenderer(norender),
			paraprogress.WithFailFast(true),
		)
		if err != nil {
			return err
		}

		if err := paramodel.Start(); err != nil {
			return err
		}
	} else {
		return err
	}

	return nil
}
