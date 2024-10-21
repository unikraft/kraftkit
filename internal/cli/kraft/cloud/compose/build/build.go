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

	kraftcloud "sdk.kraft.cloud"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/compose"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/build"
	"kraftkit.sh/internal/cli/kraft/cloud/utils"
	"kraftkit.sh/internal/cli/kraft/pkg"
	"kraftkit.sh/log"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/unikraft/app"
	"kraftkit.sh/unikraft/runtime"
	"kraftkit.sh/unikraft/target"
)

type BuildOptions struct {
	Auth        *config.AuthConfig    `noattribute:"true"`
	Composefile string                `noattribute:"true"`
	Client      kraftcloud.KraftCloud `noattribute:"true"`
	Metro       string                `noattribute:"true"`
	Project     *compose.Project      `noattribute:"true"`
	Push        bool                  `long:"push" usage:"Push the built service images"`
	Runtimes    []string              `long:"runtime" usage:"Alternative runtime to use when packaging a service"`
	Token       string                `noattribute:"true"`
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&BuildOptions{}, cobra.Command{
		Short:   "Build a compose project",
		Use:     "build [FLAGS] [COMPONENT]",
		Args:    cobra.ArbitraryArgs,
		Aliases: []string{"b"},
		Long: heredoc.Doc(`
		Build a compose project
		`),
		Example: heredoc.Doc(`
			# Build a compose project
			$ kraft cloud compose build

			# (If applicable) Set or override a runtime for a particular service
			$ kraft cloud compose build --runtime app=base:latest

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

	if opts.Client == nil {
		opts.Client = kraftcloud.NewClient(
			kraftcloud.WithToken(config.GetKraftCloudTokenAuthConfig(*opts.Auth)),
		)
	}

	userName := strings.TrimSuffix(
		strings.TrimPrefix(opts.Auth.User, "robot$"), ".users.kraftcloud",
	)

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

	runtimes := map[string]string{}

	for _, runtime := range opts.Runtimes {
		service, runtime, ok := strings.Cut(runtime, "=")
		if !ok {
			return fmt.Errorf("expected --runtime flag to be prefixed with service name, e.g. --runtime nginx=%s/nginx:latest", userName)
		}

		if _, ok := opts.Project.Services[service]; !ok {
			log.G(ctx).
				WithField("service", service).
				Warn("supplied runtime does not exist in the compose project")
			continue
		}

		runtimes[service] = runtime
	}

	for _, serviceName := range args {
		service, ok := opts.Project.Services[serviceName]
		if !ok {
			return fmt.Errorf("service '%s' not found", serviceName)
		}

		var (
			pkgName string
			appName string
		)

		if service.Image != "" {
			if !strings.Contains(service.Image, ":") {
				service.Image += ":latest"
				opts.Project.Services[serviceName] = service
			}

			appName = strings.ReplaceAll(service.Image, "/", "-")
			pkgName = fmt.Sprintf(
				"index.unikraft.io/%s/%s",
				userName,
				strings.ReplaceAll(service.Image, "_", "-"),
			)
		} else {
			appName = opts.Project.Name + "-" + service.Name
			pkgName = fmt.Sprintf(
				"index.unikraft.io/%s/%s-%s",
				userName,
				strings.ReplaceAll(opts.Project.Name, "_", "-"),
				strings.ReplaceAll(service.Name, "_", "-"),
			)
		}

		var project app.Application
		bopts := &build.BuildOptions{
			Platform:     "kraftcloud",
			Architecture: "x86_64",
			NoRootfs:     true,
		}

		popts := &pkg.PkgOptions{
			Architecture: "x86_64",
			Compress:     false,
			Format:       "oci",
			Name:         pkgName,
			NoPull:       true,
			Platform:     "kraftcloud",
			Push:         opts.Push,
			Project:      project,
			Strategy:     packmanager.StrategyOverwrite,
		}

		// If no build context can be determined, assume a build via a unikernel
		// runtime.
		if service.Build != nil {
			// First determine whether the context has a Kraftfile as this determines
			// whether we supply an artificial project defined with KraftCloud
			// defaults.
			project, err := app.NewProjectFromOptions(ctx,
				app.WithProjectWorkdir(service.Build.Context),
				app.WithProjectDefaultKraftfiles(),
			)
			if err != nil && errors.Is(err, app.ErrNoKraftfile) {
				runtime, err := runtime.NewRuntime(ctx, runtime.DefaultKraftCloudRuntime,
					runtime.WithPlatform(project.Targets()[0].Platform().String()),
					runtime.WithArchitecture(project.Targets()[0].Architecture().String()),
				)
				if err != nil {
					return fmt.Errorf("could not create runtime: %w", err)
				}
				project, err = app.NewApplicationFromOptions(
					app.WithRuntime(runtime),
					app.WithName(appName),
					app.WithTargets([]*target.TargetConfig{target.DefaultKraftCloudTarget}),
					app.WithCommand(service.Command...),
					app.WithWorkingDir(service.Build.Context),
					app.WithRootfs(filepath.Join(service.Build.Context, service.Build.Dockerfile)),
				)
				if err != nil {
					return fmt.Errorf("could not create unikernel application: %w", err)
				}
			} else if err != nil {
				return err
			}

			// Only set the supplied dockerfile as the rootfs if it exists, this is
			// because the contents of `service.Build.Dockerfile` is supplied with a
			// default value even if a Dockerfile does not actually exist.
			rootfs := filepath.Join(service.Build.Context, service.Build.Dockerfile)
			if _, err := os.Stat(rootfs); err == nil {
				bopts.Rootfs = rootfs
				popts.Rootfs = rootfs
			}

			bopts.Workdir = service.Build.Context
			popts.Workdir = service.Build.Context
			bopts.Project = project
			popts.Project = project
		} else if exists, _ := opts.imageExists(ctx, service.Image); exists {
			// Nothing to do.
			continue
		} else if exists, _ := opts.imageExists(ctx, pkgName); exists {
			// Override the image name if it is set with the new package name.
			service.Image = pkgName
			opts.Project.Services[serviceName] = service
			continue
		} else {
			var runtimeName string
			if found, ok := runtimes[serviceName]; ok {
				if !strings.Contains(found, "/") {
					found = fmt.Sprintf("index.unikraft.io/official/%s", found)
				}
				runtimeName = found
			} else {
				runtimeName = runtime.DefaultKraftCloudRuntime
			}

			log.G(ctx).
				WithField("runtime", runtimeName).
				WithField("service", serviceName).
				Debug("using")

			rt, err := runtime.NewRuntime(ctx, runtimeName,
				runtime.WithPlatform(project.Targets()[0].Platform().String()),
				runtime.WithArchitecture(project.Targets()[0].Architecture().String()),
			)
			if err != nil {
				return fmt.Errorf("could not create runtime: %w", err)
			}

			project, err = app.NewApplicationFromOptions(
				app.WithRuntime(rt),
				app.WithName(appName),
				app.WithTargets([]*target.TargetConfig{target.DefaultKraftCloudTarget}),
				app.WithRootfs(service.Image),
			)
			if err != nil {
				return fmt.Errorf("could not create unikernel application: %w", err)
			}

			bopts.Project = project
			popts.Project = project
			popts.Name = pkgName
		}

		log.G(ctx).
			WithField("service", serviceName).
			Info("building")

		if err := build.Build(ctx, bopts); err != nil {
			if !errors.Is(err, build.ErrContextNotBuildable) {
				return fmt.Errorf("could not build service %s: %w", service.Name, err)
			}

			log.G(ctx).
				WithField("service", serviceName).
				Trace("not a")
		}

		log.G(ctx).
			WithField("service", serviceName).
			Info("packaging")

		p, err := pkg.Pkg(ctx, popts)
		if err != nil {
			return fmt.Errorf("could not package service %s: %w", service.Name, err)
		}

		// Override the image name with the ID associated with the package, which
		// represents its exact name (i.e. one with a digest).  This guarantees in
		// later steps that the image is correctly referenced.
		service.Image = p[0].ID()
		opts.Project.Services[serviceName] = service
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

// imageExists checks if an image exists in the KraftCloud registry.
func (opts *BuildOptions) imageExists(ctx context.Context, name string) (exists bool, err error) {
	if name == "" {
		return false, fmt.Errorf("image name is empty")
	}

	log.G(ctx).
		WithField("image", name).
		Trace("checking exists")

	imageResp, err := opts.Client.Images().Get(ctx, name)
	if err != nil {
		return false, err
	}

	image, err := imageResp.FirstOrErr()
	if err != nil {
		return false, err
	} else if image == nil {
		return false, nil
	}

	return true, nil
}
