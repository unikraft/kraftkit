// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package pull

import (
	"context"
	"os"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/compose"
	"kraftkit.sh/internal/cli/kraft/compose/utils"

	pkgpull "kraftkit.sh/internal/cli/kraft/pkg/pull"
	"kraftkit.sh/log"
	"kraftkit.sh/packmanager"
)

type PullOptions struct {
	composefile string
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&PullOptions{}, cobra.Command{
		Short:   "Pull images of services of current project",
		Use:     "pull [FLAGS]",
		Args:    cobra.ArbitraryArgs,
		Aliases: []string{},
		Example: heredoc.Doc(`
			# Pull images for current project
			$ kraft compose pull
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "compose",
		},
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *PullOptions) Pre(cmd *cobra.Command, _ []string) error {
	ctx, err := packmanager.WithDefaultUmbrellaManagerInContext(cmd.Context())
	if err != nil {
		return err
	}

	cmd.SetContext(ctx)

	if cmd.Flag("file").Changed {
		opts.composefile = cmd.Flag("file").Value.String()
	}

	log.G(cmd.Context()).WithField("composefile", opts.composefile).Debug("using")
	return nil
}

func (opts *PullOptions) Run(ctx context.Context, args []string) error {
	workdir, err := os.Getwd()
	if err != nil {
		return err
	}

	project, err := compose.NewProjectFromComposeFile(ctx, workdir, opts.composefile)
	if err != nil {
		return err
	}

	if err := project.Validate(ctx); err != nil {
		return err
	}

	services, err := project.GetServices(args...)
	if err != nil {
		return err
	}

	pulledImages := make(map[string]string)
	for _, service := range services {
		if service.Image == "" {
			continue
		}

		if pulledBy, ok := pulledImages[service.Image]; ok {
			log.G(ctx).WithField("service", service.Name).WithField("image", service.Image).WithField("pulled by", pulledBy).Info("Image already pulled")
			continue
		}

		pulledImages[service.Image] = service.Name

		plat, arch, err := utils.PlatArchFromService(service)
		if err != nil {
			return err
		}

		pullOptions := pkgpull.PullOptions{
			Architecture: arch,
			Format:       "oci",
			Platform:     plat,
		}

		if err := pullOptions.Run(ctx, []string{service.Image}); err != nil {
			log.G(ctx).WithField("service", service.Name).WithError(err).Warn("Failed to pull image")
		}
	}

	return nil
}
