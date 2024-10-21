// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package push

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"

	cloud "sdk.kraft.cloud"
	ukcinstances "sdk.kraft.cloud/instances"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/compose"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/cloud/utils"
	"kraftkit.sh/log"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/tui/processtree"
)

type PushOptions struct {
	Auth        *config.AuthConfig            `noattribute:"true"`
	Client      ukcinstances.InstancesService `noattribute:"true"`
	Composefile string                        `noattribute:"true"`
	Metro       string                        `noattribute:"true"`
	Project     *compose.Project              `noattribute:"true"`
	Token       string                        `noattribute:"true"`
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&PushOptions{}, cobra.Command{
		Short:   "Push the images services to Unikraft Cloud from a Compose project",
		Use:     "push [FLAGS] [COMPONENT]",
		Args:    cobra.ArbitraryArgs,
		Aliases: []string{"p"},
		Example: heredoc.Doc(`
			# Push the nginx service image to Unikraft Cloud
			$ kraft cloud compose push nginx
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "cloud-compose",
		},
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func Push(ctx context.Context, opts *PushOptions, args ...string) error {
	var err error

	if opts.Auth == nil {
		opts.Auth, err = config.GetUnikraftCloudAuthConfig(ctx, opts.Token)
		if err != nil {
			return fmt.Errorf("could not retrieve credentials: %w", err)
		}
	}

	if opts.Client == nil {
		opts.Client = cloud.NewInstancesClient(
			cloud.WithToken(config.GetUnikraftCloudTokenAuthConfig(*opts.Auth)),
		)
	}

	workdir, err := os.Getwd()
	if err != nil {
		return err
	}

	if opts.Project == nil {
		opts.Project, err = compose.NewProjectFromComposeFile(ctx, workdir, opts.Composefile)
		if err != nil {
			return err
		}
	}

	if err := opts.Project.Validate(ctx); err != nil {
		return err
	}

	// If no services are specified, push all services.
	if len(args) == 0 {
		for service := range opts.Project.Services {
			args = append(args, service)
		}
	}

	var processes []*processtree.ProcessTreeItem
	for _, serviceName := range args {
		service, ok := opts.Project.Services[serviceName]
		if !ok {
			return fmt.Errorf("service '%s' not found", serviceName)
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

		packages, err := packmanager.G(ctx).Catalog(ctx,
			packmanager.WithRemote(false),
			packmanager.WithName(pkgName),
		)
		if err != nil {
			return err
		}

		if len(packages) == 0 {
			log.G(ctx).
				WithField("package", pkgName).
				Warn("not found")
		}

		for _, p := range packages {
			p := p

			processes = append(processes, processtree.NewProcessTreeItem(
				"pushing",
				humanize.Bytes(uint64(p.Size())),
				func(ctx context.Context) error {
					return p.Push(ctx)
				},
			))
		}
	}

	model, err := processtree.NewProcessTree(
		ctx,
		[]processtree.ProcessTreeOption{
			processtree.IsParallel(!config.G[config.KraftKit](ctx).NoParallel),
			processtree.WithRenderer(log.LoggerTypeFromString(config.G[config.KraftKit](ctx).Log.Type) != log.FANCY),
			processtree.WithFailFast(true),
		},
		processes...,
	)
	if err != nil {
		return err
	}

	return model.Start()
}

func (opts *PushOptions) Pre(cmd *cobra.Command, args []string) error {
	err := utils.PopulateMetroToken(cmd, &opts.Metro, &opts.Token)
	if err != nil {
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

func (opts *PushOptions) Run(ctx context.Context, args []string) error {
	return Push(ctx, opts, args...)
}
