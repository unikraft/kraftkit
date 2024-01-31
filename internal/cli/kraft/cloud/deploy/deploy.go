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
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/cloud/utils"
	"kraftkit.sh/log"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/unikraft/app"

	kraftcloud "sdk.kraft.cloud"
)

type DeployOptions struct {
	Auth         *config.AuthConfig        `noattribute:"true"`
	Client       kraftcloud.KraftCloud     `noattribute:"true"`
	DotConfig    string                    `long:"config" short:"c" usage:"Override the path to the KConfig .config file"`
	Env          []string                  `local:"true" long:"env" short:"e" usage:"Environmental variables"`
	Features     []string                  `local:"true" long:"feature" short:"f" usage:"Specify the special features to enable"`
	ForcePull    bool                      `long:"force-pull" usage:"Force pulling packages before building"`
	FQDN         string                    `local:"true" long:"fqdn" short:"d" usage:"Set the fully qualified domain name for the service"`
	Jobs         int                       `long:"jobs" short:"j" usage:"Allow N jobs at once"`
	KernelDbg    bool                      `long:"dbg" usage:"Build the debuggable (symbolic) kernel image instead of the stripped image"`
	Kraftfile    string                    `local:"true" long:"kraftfile" short:"K" usage:"Set the Kraftfile to use"`
	Memory       int64                     `local:"true" long:"memory" short:"M" usage:"Specify the amount of memory to allocate"`
	Metro        string                    `noattribute:"true"`
	Name         string                    `local:"true" long:"name" short:"n" usage:"Name of the deployment"`
	NoCache      bool                      `long:"no-cache" short:"F" usage:"Force a rebuild even if existing intermediate artifacts already exist"`
	NoConfigure  bool                      `long:"no-configure" usage:"Do not run Unikraft's configure step before building"`
	NoFast       bool                      `long:"no-fast" usage:"Do not use maximum parallelization when performing the build"`
	NoFetch      bool                      `long:"no-fetch" usage:"Do not run Unikraft's fetch step before building"`
	NoStart      bool                      `local:"true" long:"no-start" short:"S" usage:"Do not start the instance after creation"`
	NoUpdate     bool                      `long:"no-update" usage:"Do not update package index before running the build"`
	Output       string                    `local:"true" long:"output" short:"o" usage:"Set output format"`
	Ports        []string                  `local:"true" long:"port" short:"p" usage:"Specify the port mapping between external to internal"`
	Project      app.Application           `noattribute:"true"`
	Replicas     int                       `local:"true" long:"replicas" short:"R" usage:"Number of replicas of the instance" default:"0"`
	Rootfs       string                    `local:"true" long:"rootfs" usage:"Specify a path to use as root filesystem"`
	Runtime      string                    `local:"true" long:"runtime" usage:"Set an alternative project runtime"`
	SaveBuildLog string                    `long:"build-log" usage:"Use the specified file to save the output from the build"`
	ScaleToZero  bool                      `local:"true" long:"scale-to-zero" short:"0" usage:"Scale the instance to zero after deployment"`
	Strategy     packmanager.MergeStrategy `noattribute:"true"`
	SubDomain    string                    `local:"true" long:"subdomain" short:"s" usage:"Set the name to use when provisioning a subdomain"`
	Timeout      time.Duration             `local:"true" long:"timeout" usage:"Set the timeout for remote procedure calls"`
	Workdir      string                    `local:"true" long:"workdir" short:"w" usage:"Set an alternative working directory (default is cwd)"`
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
		# of your current working directory which exposes a port at 8080.
		kraft cloud --metro fra0 deploy --subdomain hello-world -p 443:8080 .
		
		# Alternatively supply an existing image which is available in the catalog:
		kraft cloud --metro fra0 deploy -p 443:8080 caddy:latest`),
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

	instances, err := d.Deploy(ctx, opts, args...)
	if err != nil {
		return fmt.Errorf("could not prepare deployment: %w", err)
	}

	if len(instances) == 1 && opts.Output == "" {
		utils.PrettyPrintInstance(ctx, &instances[0], !opts.NoStart)
		return nil
	}

	return utils.PrintInstances(ctx, opts.Output, instances...)
}
