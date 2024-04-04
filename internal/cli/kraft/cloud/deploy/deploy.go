// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package deploy

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	"kraftkit.sh/tui/selection"
	"kraftkit.sh/unikraft/app"

	kraftcloud "sdk.kraft.cloud"
	kcservices "sdk.kraft.cloud/services"
)

type DeployOptions struct {
	Auth                   *config.AuthConfig        `noattribute:"true"`
	Client                 kraftcloud.KraftCloud     `noattribute:"true"`
	Compress               bool                      `local:"true" long:"compress" short:"z" usage:"Compress the initrd package (experimental)"`
	DeployAs               string                    `local:"true" long:"as" short:"D" usage:"Set the deployment type"`
	Domain                 []string                  `local:"true" long:"domain" short:"d" usage:"Set the domain names for the service"`
	DotConfig              string                    `long:"config" short:"c" usage:"Override the path to the KConfig .config file"`
	Env                    []string                  `local:"true" long:"env" short:"e" usage:"Environmental variables"`
	Features               []string                  `local:"true" long:"feature" short:"f" usage:"Specify the special features to enable"`
	ForcePull              bool                      `long:"force-pull" usage:"Force pulling packages before building"`
	Jobs                   int                       `long:"jobs" short:"j" usage:"Allow N jobs at once"`
	KernelDbg              bool                      `long:"dbg" usage:"Build the debuggable (symbolic) kernel image instead of the stripped image"`
	Kraftfile              string                    `local:"true" long:"kraftfile" short:"K" usage:"Set the Kraftfile to use"`
	Memory                 int                       `local:"true" long:"memory" short:"M" usage:"Specify the amount of memory to allocate (MiB)"`
	Metro                  string                    `noattribute:"true"`
	Name                   string                    `local:"true" long:"name" short:"n" usage:"Name of the deployment"`
	NoCache                bool                      `long:"no-cache" short:"F" usage:"Force a rebuild even if existing intermediate artifacts already exist"`
	NoConfigure            bool                      `long:"no-configure" usage:"Do not run Unikraft's configure step before building"`
	NoFast                 bool                      `long:"no-fast" usage:"Do not use maximum parallelization when performing the build"`
	NoFetch                bool                      `long:"no-fetch" usage:"Do not run Unikraft's fetch step before building"`
	NoStart                bool                      `local:"true" long:"no-start" short:"S" usage:"Do not start the instance after creation"`
	NoUpdate               bool                      `long:"no-update" usage:"Do not update package index before running the build"`
	Output                 string                    `local:"true" long:"output" short:"o" usage:"Set output format"`
	Ports                  []string                  `local:"true" long:"port" short:"p" usage:"Specify the port mapping between external to internal"`
	Project                app.Application           `noattribute:"true"`
	Replicas               int                       `local:"true" long:"replicas" short:"R" usage:"Number of replicas of the instance" default:"0"`
	Rollout                bool                      `local:"true" long:"rollout" short:"r" usage:"Whether to perform a rolling update of all instances in a service group"`
	Rootfs                 string                    `local:"true" long:"rootfs" usage:"Specify a path to use as root filesystem"`
	Runtime                string                    `local:"true" long:"runtime" usage:"Set an alternative project runtime"`
	SaveBuildLog           string                    `long:"build-log" usage:"Use the specified file to save the output from the build"`
	ScaleToZero            bool                      `local:"true" long:"scale-to-zero" short:"0" usage:"Scale the instance to zero after deployment"`
	ServiceGroupNameOrUUID string                    `long:"service-group" short:"g" usage:"Attach the new deployment to an existing service group"`
	Strategy               packmanager.MergeStrategy `noattribute:"true"`
	SubDomain              []string                  `local:"true" long:"subdomain" short:"s" usage:"Set the names to use when provisioning subdomains"`
	Timeout                time.Duration             `local:"true" long:"timeout" usage:"Set the timeout for remote procedure calls" default:"60s"`
	Token                  string                    `noattribute:"true"`
	Volumes                []string                  `long:"volume" short:"v" usage:"Specify the volume mapping(s) in the form NAME:DEST or NAME:DEST:OPTIONS"`
	Workdir                string                    `local:"true" long:"workdir" short:"w" usage:"Set an alternative working directory (default is cwd)"`
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&DeployOptions{}, cobra.Command{
		Short:   "Deploy your application",
		Use:     "deploy",
		Aliases: []string{"launch", "run"},
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "kraftcloud",
		},
		Long: heredoc.Doc(`
			'kraft cloud deploy' combines a number of kraft cloud sub-commands
			to enable you to build, package, ship and deploy your application
			with a single command.
		`),
		Example: heredoc.Docf(`
			# Run an image from KraftCloud's catalog:
			$ kraft cloud --metro fra0 deploy -p 443:8080 caddy:latest
		`),
	})
	if err != nil {
		panic(err)
	}

	cmd.Flags().Var(
		cmdfactory.NewEnumFlag[packmanager.MergeStrategy](
			append(packmanager.MergeStrategies(), packmanager.StrategyPrompt),
			packmanager.StrategyOverwrite,
		),
		"strategy",
		"When a package of the same name exists, use this strategy when applying targets.",
	)

	return cmd
}

func (opts *DeployOptions) Pre(cmd *cobra.Command, _ []string) error {
	err := utils.PopulateMetroToken(cmd, &opts.Metro, &opts.Token)
	if err != nil {
		return fmt.Errorf("could not populate metro and token: %w", err)
	}

	opts.Strategy = packmanager.MergeStrategy(cmd.Flag("strategy").Value.String())

	ctx, err := packmanager.WithDefaultUmbrellaManagerInContext(cmd.Context())
	if err != nil {
		return err
	}

	if opts.Rollout && opts.ServiceGroupNameOrUUID == "" {
		return errors.New("cannot use --rollout without a --service-group")
	}

	if opts.Rollout && opts.Replicas > 0 {
		return errors.New("cannot use --rollout with --replicas")
	}

	cmd.SetContext(ctx)

	return nil
}

func (opts *DeployOptions) Run(ctx context.Context, args []string) error {
	var err error

	opts.Auth, err = config.GetKraftCloudAuthConfig(ctx, opts.Token)
	if err != nil {
		return fmt.Errorf("could not retrieve credentials: %w", err)
	}

	opts.Client = kraftcloud.NewClient(
		kraftcloud.WithToken(config.GetKraftCloudTokenAuthConfig(*opts.Auth)),
	)

	// TODO: Preflight check: check if `--subdomain` is already taken

	// Preflight check: check if `--name` is already taken:
	if len(opts.Name) > 0 {
		resp, err := opts.Client.Instances().WithMetro(opts.Metro).Get(ctx, opts.Name)
		if err != nil {
			return err
		}
		if _, err = resp.AllOrErr(); err == nil {
			return fmt.Errorf("instance name '%s' is already taken", opts.Name)
		}
	}

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
	var errs []error
	var candidates []deployer

	for _, candidate := range deployers() {
		if opts.DeployAs != "" && candidate.Name() != opts.DeployAs {
			continue
		}

		log.G(ctx).
			WithField("deployer", candidate.Name()).
			Trace("checking deployability")

		capable, err := candidate.Deployable(ctx, opts, args...)
		if capable && err == nil {
			candidates = append(candidates, candidate)
		} else if err != nil {
			errs = append(errs, err)
			log.G(ctx).
				WithField("deployer", candidate.Name()).
				Debugf("cannot run because: %v", err)
		}
	}

	if len(candidates) == 0 {
		return fmt.Errorf("could not determine how to run provided input: %w", errors.Join(errs...))
	} else if len(candidates) == 1 {
		d = candidates[0]
	} else if !config.G[config.KraftKit](ctx).NoPrompt {
		candidate, err := selection.Select[deployer]("multiple deployable contexts discovered: how would you like to proceed?", candidates...)
		if err != nil {
			return err
		}

		d = *candidate

		log.G(ctx).Infof("use --as=%s to skip this prompt in the future", d.Name())
	} else {
		return fmt.Errorf("multiple contexts discovered: %v", candidates)
	}

	log.G(ctx).WithField("deployer", d.Name()).Debug("using")

	instsResp, sgs, err := d.Deploy(ctx, opts, args...)
	if err != nil {
		return fmt.Errorf("could not prepare deployment: %w", err)
	}
	insts, err := instsResp.AllOrErr()
	if err != nil {
		return err
	}

	if opts.Rollout {
		if len(sgs[0].Instances) == 1 {
			log.G(ctx).Warn("cannot perform a rolling update on no instances")
			return nil
		}

		paramodel, err := processtree.NewProcessTree(
			ctx,
			[]processtree.ProcessTreeOption{
				processtree.IsParallel(false),
				processtree.WithRenderer(
					log.LoggerTypeFromString(config.G[config.KraftKit](ctx).Log.Type) != log.FANCY,
				),
				processtree.WithFailFast(true),
				processtree.WithHideOnSuccess(false),
				processtree.WithTimeout(opts.Timeout),
			},
			processtree.NewProcessTreeItem(
				"updating "+fmt.Sprintf("%d", len(sgs[0].Instances)-1)+" instances of "+sgs[0].Name,
				"",
				func(ctx context.Context) error {
					return create.Rollout(ctx, &create.CreateOptions{
						Auth:                   opts.Auth,
						Client:                 opts.Client,
						Env:                    opts.Env,
						Features:               opts.Features,
						Domain:                 opts.Domain,
						Image:                  insts[0].Image,
						Memory:                 opts.Memory,
						Metro:                  opts.Metro,
						Name:                   opts.Name,
						Output:                 opts.Output,
						Ports:                  opts.Ports,
						Replicas:               opts.Replicas,
						Rollout:                opts.Rollout,
						ServiceGroupNameOrUUID: opts.ServiceGroupNameOrUUID,
						Start:                  !opts.NoStart,
						ScaleToZero:            opts.ScaleToZero,
						SubDomain:              opts.SubDomain,
						Token:                  opts.Token,
						Volumes:                opts.Volumes,
						WaitForImage:           true,
					}, &insts[0], &sgs[0])
				},
			),
		)
		if err != nil {
			return nil
		}

		err = paramodel.Start()
		if err != nil {
			return fmt.Errorf("could not start the process tree: %w", err)
		}
	}

	if len(insts) == 1 && opts.Output == "" {
		if len(sgs) == 0 {
			sgs = append(sgs, kcservices.GetResponseItem{})
		}

		utils.PrettyPrintInstance(ctx, &insts[0], &sgs[0], !opts.NoStart)
		return nil
	}

	return utils.PrintInstances(ctx, opts.Output, instsResp)
}
