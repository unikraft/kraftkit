// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package push

import (
	"context"
	"os"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/compose"

	pkgpush "kraftkit.sh/internal/cli/kraft/pkg/push"
	"kraftkit.sh/log"
	"kraftkit.sh/packmanager"
)

type PushOptions struct {
	composefile string
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&PushOptions{}, cobra.Command{
		Short:   "Push images of services of current project",
		Use:     "push [FLAGS]",
		Args:    cobra.ArbitraryArgs,
		Aliases: []string{},
		Example: heredoc.Doc(`
			# Push images for current project
			$ kraft compose push
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

func (opts *PushOptions) Pre(cmd *cobra.Command, _ []string) error {
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

func (opts *PushOptions) Run(ctx context.Context, args []string) error {
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

	pushedImages := make(map[string]string)
	for _, service := range services {
		if service.Image == "" {
			continue
		}

		if pushedBy, ok := pushedImages[service.Image]; ok {
			log.G(ctx).WithField("service", service.Name).WithField("image", service.Image).WithField("pushed by", pushedBy).Info("Image already pushed")
			continue
		}

		pushedImages[service.Image] = service.Name
		pushOptions := pkgpush.PushOptions{
			Format: "oci",
		}

		if err := pushOptions.Run(ctx, []string{service.Image}); err != nil {
			log.G(ctx).WithField("service", service.Name).WithError(err).Warn("Failed to push image")
		}
	}

	return nil
}
