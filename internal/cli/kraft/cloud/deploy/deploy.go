// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package deploy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/cloud/instance/create"
	"kraftkit.sh/internal/cli/kraft/cloud/utils"
	"kraftkit.sh/log"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/tui/processtree"
	"kraftkit.sh/unikraft/app"

	kraftcloud "sdk.kraft.cloud"
	kraftcloudinstances "sdk.kraft.cloud/instances"
)

type DeployOptions struct {
	Auth      *config.AuthConfig        `noattribute:"true"`
	Client    kraftcloud.KraftCloud     `noattribute:"true"`
	Env       []string                  `local:"true" long:"env" short:"e" usage:"Environmental variables"`
	FQDN      string                    `local:"true" long:"fqdn" short:"d" usage:"Set the fully qualified domain name for the service"`
	Kraftfile string                    `local:"true" long:"kraftfile" short:"K" usage:"Set the Kraftfile to use"`
	Memory    int64                     `local:"true" long:"memory" short:"M" usage:"Specify the amount of memory to allocate"`
	Metro     string                    `noattribute:"true"`
	Name      string                    `local:"true" long:"name" short:"n" usage:"Name of the deployment"`
	NoStart   bool                      `local:"true" long:"no-start" short:"S" usage:"Do not start the instance after creation"`
	Output    string                    `local:"true" long:"output" short:"o" usage:"Set output format" default:"table"`
	Ports     []string                  `local:"true" long:"port" short:"p" usage:"Specify the port mapping between external to internal"`
	Project   app.Application           `noattribute:"true"`
	Replicas  int                       `local:"true" long:"replicas" short:"R" usage:"Number of replicas of the instance" default:"0"`
	Strategy  packmanager.MergeStrategy `noattribute:"true"`
	SubDomain string                    `local:"true" long:"subdomain" short:"s" usage:"Set the name to use when provisioning a subdomain"`
	Timeout   time.Duration             `local:"true" long:"timeout" usage:"Set the timeout for remote procedure calls"`
	Workdir   string                    `local:"true" long:"workdir" short:"w" usage:"Set an alternative working directory (default is cwd)"`
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&DeployOptions{}, cobra.Command{
		Short:   "Deploy your application",
		Use:     "deploy",
		Aliases: []string{"launch", "run"},
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "kraftcloud",
		},
		Example: heredoc.Docf(`
		# Create a new deployment at https://hello-world.fra0.kraft.cloud in Frankfurt
		# of your current working directory which exposes a port at 8080:
		kraft cloud --metro fra0 deploy --subdomain hello-world -p 443:8080 .`),
	})
	if err != nil {
		panic(err)
	}

	cmd.Flags().Var(
		cmdfactory.NewEnumFlag[packmanager.MergeStrategy](
			append(packmanager.MergeStrategies(), packmanager.StrategyPrompt),
			packmanager.StrategyPrompt,
		),
		"strategy",
		"When a package of the same name exists, use this strategy when applying targets.",
	)

	cmd.Flags().String(
		"domain",
		"",
		"Alias for --fqdn|-d",
	)

	return cmd
}

func (opts *DeployOptions) Pre(cmd *cobra.Command, _ []string) error {
	opts.Metro = cmd.Flag("metro").Value.String()
	if opts.Metro == "" {
		opts.Metro = os.Getenv("KRAFTCLOUD_METRO")
	}
	if opts.Metro == "" {
		return fmt.Errorf("kraftcloud metro is unset")
	}

	log.G(cmd.Context()).WithField("metro", opts.Metro).Debug("using")

	opts.Strategy = packmanager.MergeStrategy(cmd.Flag("strategy").Value.String())

	domain := cmd.Flag("domain").Value.String()
	if len(domain) > 0 && len(opts.FQDN) > 0 {
		return fmt.Errorf("cannot use --domain and --fqdn together")
	} else if len(domain) > 0 && len(opts.FQDN) == 0 {
		opts.FQDN = domain
	}

	ctx, err := packmanager.WithDefaultUmbrellaManagerInContext(cmd.Context())
	if err != nil {
		return err
	}

	cmd.SetContext(ctx)

	return nil
}

func (opts *DeployOptions) Run(ctx context.Context, args []string) error {
	var err error

	opts.Auth, err = config.GetKraftCloudAuthConfigFromContext(ctx)
	if err != nil {
		return fmt.Errorf("could not retrieve credentials: %w", err)
	}

	opts.Client = kraftcloud.NewClient(
		kraftcloud.WithToken(config.GetKraftCloudTokenAuthConfig(*opts.Auth)),
	)

	if len(args) > 0 {
		if fi, err := os.Stat(args[0]); err == nil && fi.IsDir() {
			abs, err := filepath.Abs(args[0])
			if err != nil {
				return fmt.Errorf("could not calculate absolute path of '%s': %w", args[0], err)
			}

			opts.Workdir = abs
			args = args[1:]
		}
	}

	if opts.Workdir == "" {
		opts.Workdir, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("could not get current working directory")
		}
	}

	var d deployer

	deployers := deployers()

	// Iterate through the list of built-in builders which sequentially tests
	// the current context and Kraftfile match specific requirements towards
	// performing a type of build.
	for _, candidate := range deployers {
		log.G(ctx).
			WithField("packager", candidate.String()).
			Trace("checking compatibility")

		capable, err := candidate.Deployable(ctx, opts, args...)
		if capable && err == nil {
			d = candidate
			break
		}

		log.G(ctx).
			WithError(err).
			WithField("packager", candidate.String()).
			Trace("incompatbile")
	}

	if d == nil {
		return fmt.Errorf("could not determine what or how to deploy from the given context")
	}

	var pkgName string

	if len(opts.Name) > 0 {
		pkgName = opts.Name
	} else if opts.Project != nil && len(opts.Project.Name()) > 0 {
		pkgName = opts.Project.Name()
	} else {
		pkgName = filepath.Base(opts.Workdir)
	}

	if strings.HasPrefix(pkgName, "unikraft.io") {
		pkgName = "index." + pkgName
	}
	if !strings.HasPrefix(pkgName, "index.unikraft.io") {
		pkgName = fmt.Sprintf(
			"index.unikraft.io/%s/%s:latest",
			strings.TrimSuffix(strings.TrimPrefix(opts.Auth.User, "robot$"), ".users.kraftcloud"),
			pkgName,
		)
	}

	packs, err := d.Prepare(ctx, opts, pkgName)
	if err != nil {
		return fmt.Errorf("could not prepare deployment: %w", err)
	}

	// FIXME(nderjung): Gathering the digest like this really dirty.
	metadata := packs[0].Columns()
	var digest string
	for _, m := range metadata {
		if m.Name != "index" {
			continue
		}

		digest = m.Value
	}

	// TODO(nderjung): This is a quirk that will be removed.  Remove the `index.`
	// from the name.
	if pkgName[0:17] == "index.unikraft.io" {
		pkgName = pkgName[6:]
	}
	if pkgName[0:12] == "unikraft.io/" {
		pkgName = pkgName[12:]
	}

	var instance *kraftcloudinstances.Instance

	paramodel, err := processtree.NewProcessTree(
		ctx,
		[]processtree.ProcessTreeOption{
			processtree.IsParallel(false),
			processtree.WithRenderer(
				log.LoggerTypeFromString(config.G[config.KraftKit](ctx).Log.Type) != log.FANCY,
			),
			processtree.WithFailFast(true),
			processtree.WithHideOnSuccess(true),
			processtree.WithTimeout(opts.Timeout),
		},
		processtree.NewProcessTreeItem(
			"deploying",
			"",
			func(ctx context.Context) error {
			checkRemoteImages:
				for {
					// First check if the context has been cancelled
					select {
					case <-ctx.Done():
						return fmt.Errorf("context cancelled")
					default:
					}

					ctxTimeout, cancel := context.WithTimeout(ctx, 30*time.Second)
					defer cancel()

					images, err := opts.Client.Images().WithMetro(opts.Metro).List(ctxTimeout)
					if err != nil {
						return fmt.Errorf("could not check list of images: %w", err)
					}

					for _, image := range images {
						split := strings.Split(image.Digest, "@sha256:")
						if !strings.HasPrefix(split[len(split)-1], digest) {
							continue
						}

						break checkRemoteImages
					}
				}

				instance, err = create.Create(ctx, &create.CreateOptions{
					Env:       opts.Env,
					FQDN:      opts.FQDN,
					Memory:    opts.Memory,
					Metro:     opts.Metro,
					Name:      opts.Name,
					Ports:     opts.Ports,
					Replicas:  opts.Replicas,
					Start:     !opts.NoStart,
					SubDomain: opts.SubDomain,
				}, pkgName)
				if err != nil {
					return fmt.Errorf("could not create instance: %w", err)
				}

				return nil
			},
		),
	)
	if err != nil {
		return err
	}

	if err := paramodel.Start(); err != nil {
		return err
	}

	utils.PrettyPrintInstance(ctx, instance, !opts.NoStart)

	return nil
}
