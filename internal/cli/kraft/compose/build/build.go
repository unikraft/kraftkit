// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package build

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/compose-spec/compose-go/types"
	"github.com/spf13/cobra"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/compose"
	"kraftkit.sh/internal/cli/kraft/build"
	"kraftkit.sh/internal/cli/kraft/pkg"
	"kraftkit.sh/log"
	"kraftkit.sh/packmanager"
)

type BuildOptions struct {
	composefile string
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&BuildOptions{}, cobra.Command{
		Short: "Build or rebuild services",
		Use:   "build",
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "compose",
		},
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *BuildOptions) Pre(cmd *cobra.Command, _ []string) error {
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

func (opts *BuildOptions) Run(ctx context.Context, args []string) error {
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

	for _, service := range project.Services {
		if service.Build == nil {
			continue
		}

		if err := buildService(ctx, service); err != nil {
			return err
		}

		if service.Image != "" {
			if err := pkgService(ctx, service); err != nil {
				return err
			}
		}
	}

	return nil
}

func platArchFromService(service types.ServiceConfig) (string, string, error) {
	// The service platform should be in the form <platform>/<arch>

	parts := strings.SplitN(service.Platform, "/", 2)

	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid platform: %s for service %s", service.Platform, service.Name)
	}

	return parts[0], parts[1], nil
}

func buildService(ctx context.Context, service types.ServiceConfig) error {
	if service.Build == nil {
		return fmt.Errorf("service %s has no build context", service.Name)
	}

	plat, arch, err := platArchFromService(service)
	if err != nil {
		return err
	}

	log.G(ctx).Infof("Building service %s...", service.Name)

	buildOptions := build.BuildOptions{Platform: plat, Architecture: arch}

	return buildOptions.Run(ctx, []string{service.Build.Context})
}

func pkgService(ctx context.Context, service types.ServiceConfig) error {
	plat, arch, err := platArchFromService(service)
	if err != nil {
		return err
	}

	log.G(ctx).Infof("packaging service %s...", service.Name)

	pkgOptions := pkg.PkgOptions{
		Architecture: arch,
		Name:         service.Image,
		Format:       "oci",
		Platform:     plat,
		Strategy:     packmanager.StrategyOverwrite,
	}

	return pkgOptions.Run(ctx, []string{service.Build.Context})
}
