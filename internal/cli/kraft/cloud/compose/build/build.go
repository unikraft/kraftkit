// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package build

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/compose"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/build"
	"kraftkit.sh/internal/cli/kraft/cloud/utils"
	"kraftkit.sh/internal/cli/kraft/pkg"
	"kraftkit.sh/log"
	"kraftkit.sh/packmanager"
)

type BuildOptions struct {
	Auth        *config.AuthConfig `noattribute:"true"`
	Composefile string             `noattribute:"true"`
	Metro       string             `noattribute:"true"`
	Project     *compose.Project   `noattribute:"true"`
	Push        bool               `long:"push" usage:"Push the built service images"`
	Token       string             `noattribute:"true"`
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&BuildOptions{}, cobra.Command{
		Short:   "Build a compose project for KraftCloud",
		Use:     "build [FLAGS] [COMPONENT]",
		Args:    cobra.ArbitraryArgs,
		Aliases: []string{"b"},
		Long: heredoc.Doc(`
		Build a compose project for KraftCloud
		`),
		Example: heredoc.Doc(`
			# Build a compose project for KraftCloud
			$ kraft cloud compose build

			# Push the service images after a successful build
			$ kraft cloud compose build --push
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "kraftcloud-compose",
		},
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func Build(ctx context.Context, opts *BuildOptions, args ...string) error {
	var err error

	if opts == nil {
		opts = &BuildOptions{}
	}

	if opts.Auth == nil {
		opts.Auth, err = config.GetKraftCloudAuthConfig(ctx, opts.Token)
		if err != nil {
			return fmt.Errorf("could not retrieve credentials: %w", err)
		}
	}

	if opts.Project == nil {
		workdir, err := os.Getwd()
		if err != nil {
			return err
		}

		opts.Project, err = compose.NewProjectFromComposeFile(ctx, workdir, opts.Composefile)
		if err != nil {
			return err
		}
	}

	if err := opts.Project.Validate(ctx); err != nil {
		return err
	}

	// If no services are specified, build all services.
	if len(args) == 0 {
		for service := range opts.Project.Services {
			args = append(args, service)
		}
	}

	for _, serviceName := range args {
		service, ok := opts.Project.Services[serviceName]
		if !ok {
			return fmt.Errorf("service '%s' not found", serviceName)
		}

		if service.Build == nil || service.Build.Context == "" {
			log.G(ctx).WithField("service", service.Name).Debug("build context not defined")
			continue
		}

		if err := build.Build(ctx, &build.BuildOptions{
			Platform:     "kraftcloud",
			Architecture: "x86_64",
			Workdir:      service.Build.Context,
			NoRootfs:     true, // This will be built in the packaging step below.
		}); err != nil && !errors.Is(err, build.ErrContextNotBuildable) {
			return fmt.Errorf("could not build service %s: %w", service.Name, err)
		}

		var pkgName string
		if service.Image != "" {
			pkgName = service.Image
		} else {
			pkgName = service.Name
		}

		user := strings.TrimSuffix(strings.TrimPrefix(opts.Auth.User, "robot$"), ".users.kraftcloud")
		if split := strings.Split(pkgName, "/"); len(split) > 1 {
			user = split[0]
			pkgName = strings.Join(split[1:], "/")
		}

		if strings.HasPrefix(pkgName, "unikraft.io") {
			pkgName = "index." + pkgName
		}
		if !strings.HasPrefix(pkgName, "index.unikraft.io") {
			pkgName = fmt.Sprintf(
				"index.unikraft.io/%s/%s",
				user,
				pkgName,
			)
		}

		// Detect whether the supplied Dockerfile exists before passing it to the
		// packaging step.  If a non-empty value for the Rootfs attribute is
		// supplied to the packaging step, it will trickle through the rootfs build
		// system and may cause failure due to misconfiguration.  This is because
		// the value of `service.Build.Dockerfile` is by default set to "Dockerfile"
		// which may in fact not exist.  The packaging step relies on the present of
		// a Kraftfile which supersedes the Dockerfile.
		rootfs := filepath.Join(service.Build.Context, service.Build.Dockerfile)
		if _, err := os.Stat(rootfs); err != nil && os.IsNotExist(err) {
			rootfs = ""
		}

		if _, err = pkg.Pkg(ctx, &pkg.PkgOptions{
			Architecture: "x86_64",
			Compress:     false,
			Format:       "oci",
			Name:         pkgName,
			NoPull:       true,
			Platform:     "kraftcloud",
			Push:         opts.Push,
			Rootfs:       rootfs,
			Workdir:      service.Build.Context,
			Strategy:     packmanager.StrategyOverwrite,
		}); err != nil {
			return fmt.Errorf("could not package service %s: %w", service.Name, err)
		}
	}

	return nil
}

func (opts *BuildOptions) Pre(cmd *cobra.Command, args []string) error {
	if err := utils.PopulateMetroToken(cmd, &opts.Metro, &opts.Token); err != nil {
		return fmt.Errorf("could not populate metro and token: %w", err)
	}

	ctx, err := packmanager.WithDefaultUmbrellaManagerInContext(cmd.Context())
	if err != nil {
		return err
	}

	cmd.SetContext(ctx)

	if cmd.Flag("file").Changed {
		opts.Composefile = cmd.Flag("file").Value.String()
	}

	return nil
}

func (opts *BuildOptions) Run(ctx context.Context, args []string) error {
	return Build(ctx, opts, args...)
}
