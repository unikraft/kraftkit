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
	"slices"
	"strconv"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/compose-spec/compose-go/v2/types"
	"github.com/spf13/cobra"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/cloud/instance/create"
	"kraftkit.sh/internal/cli/kraft/cloud/instance/logs"
	"kraftkit.sh/internal/cli/kraft/cloud/utils"
	"kraftkit.sh/log"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/tui/selection"
	"kraftkit.sh/unikraft/app"

	kraftcloud "sdk.kraft.cloud"
	kcinstances "sdk.kraft.cloud/instances"
)

type DeployOptions struct {
	Auth                *config.AuthConfig             `noattribute:"true"`
	Client              kraftcloud.KraftCloud          `noattribute:"true"`
	Certificate         []string                       `local:"true" long:"certificate" short:"C" usage:"Set the certificates to use for the service"`
	Compress            bool                           `local:"true" long:"compress" short:"z" usage:"Compress the initrd package (experimental)"`
	DeployAs            string                         `local:"true" long:"as" short:"D" usage:"Set the deployment type"`
	Domain              []string                       `local:"true" long:"domain" short:"d" usage:"Set the domain names for the service"`
	DotConfig           string                         `long:"config" short:"c" usage:"Override the path to the KConfig .config file"`
	Entrypoint          types.ShellCommand             `local:"true" long:"entrypoint" usage:"Set the entrypoint for the instance"`
	Env                 []string                       `local:"true" long:"env" short:"e" usage:"Environmental variables"`
	Features            []string                       `local:"true" long:"feature" usage:"Specify the special features to enable"`
	Follow              bool                           `local:"true" long:"follow" short:"f" usage:"Follow the logs of the instance"`
	ForcePull           bool                           `long:"force-pull" usage:"Force pulling packages before building"`
	Image               string                         `long:"image" short:"i" usage:"Set the image name to use"`
	Jobs                int                            `long:"jobs" short:"j" usage:"Allow N jobs at once"`
	KernelDbg           bool                           `long:"dbg" usage:"Build the debuggable (symbolic) kernel image instead of the stripped image"`
	Kraftfile           string                         `local:"true" long:"kraftfile" short:"K" usage:"Set the Kraftfile to use"`
	Memory              string                         `local:"true" long:"memory" short:"M" usage:"Specify the amount of memory to allocate (MiB increments)"`
	Metro               string                         `noattribute:"true"`
	Name                string                         `local:"true" long:"name" short:"n" usage:"Name of the deployment"`
	NoCache             bool                           `long:"no-cache" short:"F" usage:"Force a rebuild even if existing intermediate artifacts already exist"`
	NoConfigure         bool                           `long:"no-configure" usage:"Do not run Unikraft's configure step before building"`
	NoFast              bool                           `long:"no-fast" usage:"Do not use maximum parallelization when performing the build"`
	NoFetch             bool                           `long:"no-fetch" usage:"Do not run Unikraft's fetch step before building"`
	NoStart             bool                           `local:"true" long:"no-start" short:"S" usage:"Do not start the instance after creation"`
	NoUpdate            bool                           `long:"no-update" usage:"Do not update package index before running the build"`
	Output              string                         `local:"true" long:"output" short:"o" usage:"Set output format"`
	Ports               []string                       `local:"true" long:"port" short:"p" usage:"Specify the port mapping between external to internal"`
	Project             app.Application                `noattribute:"true"`
	Replicas            uint                           `local:"true" long:"replicas" short:"R" usage:"Number of replicas of the instance" default:"0"`
	RestartPolicy       kcinstances.RestartPolicy      `noattribute:"true"`
	Rollout             create.RolloutStrategy         `noattribute:"true"`
	RolloutQualifier    create.RolloutQualifier        `noattribute:"true"`
	RolloutWait         time.Duration                  `local:"true" long:"rollout-wait" usage:"Time to wait before performing rolling out action (ms/s/m/h)" default:"10s"`
	Rootfs              string                         `local:"true" long:"rootfs" usage:"Specify a path to use as root filesystem"`
	Runtime             string                         `local:"true" long:"runtime" usage:"Set an alternative project runtime"`
	SaveBuildLog        string                         `long:"build-log" usage:"Use the specified file to save the output from the build"`
	ScaleToZero         *kcinstances.ScaleToZeroPolicy `noattribute:"true"`
	ScaleToZeroStateful *bool                          `local:"true" long:"scale-to-zero-stateful" usage:"Save state when scaling to zero"`
	ScaleToZeroCooldown time.Duration                  `local:"true" long:"scale-to-zero-cooldown" usage:"Cooldown period before scaling to zero (ms/s/m/h)"`
	ServiceNameOrUUID   string                         `long:"service" short:"g" usage:"Attach the new deployment to an existing service"`
	Strategy            packmanager.MergeStrategy      `noattribute:"true"`
	SubDomain           []string                       `local:"true" long:"subdomain" short:"s" usage:"Set the names to use when provisioning subdomains"`
	Timeout             time.Duration                  `local:"true" long:"timeout" usage:"Set the timeout for remote procedure calls (ms/s/m/h)" default:"60s"`
	Token               string                         `noattribute:"true"`
	Vcpus               uint                           `local:"true" long:"vcpus" short:"V" usage:"Specify the number of vCPUs to allocate"`
	Volumes             []string                       `long:"volume" short:"v" usage:"Specify the volume mapping(s) in the form NAME:DEST or NAME:DEST:OPTIONS"`
	Workdir             string                         `local:"true" long:"workdir" short:"w" usage:"Set an alternative working directory (default is cwd)"`
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&DeployOptions{}, cobra.Command{
		Short:   "Deploy your application",
		Use:     "deploy [ARGS] [CONTEXT] [-- [APP ARGS]]",
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
			# Deploy a working directory with a Kraftfile or Dockerfile:
			$ kraft cloud --metro fra0 deploy -p 443:8080

			# Run an image from Unikraft Cloud's image catalog:
			$ kraft cloud --metro fra0 deploy -p 443:8080 caddy:latest

			# Supply arguments to the instance of the existing image
			$ kraft cloud --metro fra0 deploy -p 443:8080 caddy:latest -- /bin/server --debug

			# Supply arguments to the instance of the project (overriding the cmd):
			$ kraft cloud --metro fra0 deploy -p 443:8080 . -- /bin/server --debug

			# Immediately start following the log tail
			$ kraft cloud --metro fra0 deploy -p 443:8080 -f caddy:latest
		`),
	})
	if err != nil {
		panic(err)
	}

	cmd.Flags().Var(
		cmdfactory.NewEnumFlag[kcinstances.RestartPolicy](
			kcinstances.RestartPolicies(),
			kcinstances.RestartPolicyNever,
		),
		"restart",
		"Set the restart policy for the instance (never/always/on-failure)",
	)

	cmd.Flags().Var(
		cmdfactory.NewEnumFlag[create.RolloutStrategy](
			append(create.RolloutStrategies(), create.StrategyPrompt),
			create.StrategyPrompt,
		),
		"rollout",
		"Set the rollout strategy for an instance which has been previously run in the provided service.",
	)

	cmd.Flags().Var(
		cmdfactory.NewEnumFlag[create.RolloutQualifier](
			create.RolloutQualifiers(),
			create.RolloutQualifierImageName,
		),
		"rollout-qualifier",
		"Set the rollout qualifier used to determine which instances should be affected by the strategy in the supplied service.",
	)

	cmd.Flags().Var(
		cmdfactory.NewEnumFlag[kcinstances.ScaleToZeroPolicy](
			kcinstances.ScaleToZeroPolicies(),
			kcinstances.ScaleToZeroPolicyOff,
		),
		"scale-to-zero",
		"Scale to zero policy of the instance (on/off/idle)",
	)

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

	opts.RestartPolicy = kcinstances.RestartPolicy(cmd.Flag("restart").Value.String())
	opts.Rollout = create.RolloutStrategy(cmd.Flag("rollout").Value.String())
	opts.RolloutQualifier = create.RolloutQualifier(cmd.Flag("rollout-qualifier").Value.String())
	opts.Strategy = packmanager.MergeStrategy(cmd.Flag("strategy").Value.String())

	if cmd.Flag("scale-to-zero").Changed {
		s20v := kcinstances.ScaleToZeroPolicy(cmd.Flag("scale-to-zero").Value.String())
		opts.ScaleToZero = &s20v
	}

	if cmd.Flag("scale-to-zero-stateful").Changed {
		statefulFlag, err := strconv.ParseBool(cmd.Flag("scale-to-zero-stateful").Value.String())
		if err != nil {
			return fmt.Errorf("could not parse scale-to-zero-stateful: %w", err)
		}

		opts.ScaleToZeroStateful = &statefulFlag
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

	opts.Auth, err = config.GetKraftCloudAuthConfig(ctx, opts.Token)
	if err != nil {
		return fmt.Errorf("could not retrieve credentials: %w", err)
	}

	opts.Client = kraftcloud.NewClient(
		kraftcloud.WithToken(config.GetKraftCloudTokenAuthConfig(*opts.Auth)),
	)

	// TODO: Preflight check: check if `--subdomain` is already taken

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
		// Remove any candidates that do not have String prompts.
		candidates = slices.DeleteFunc(candidates, func(d deployer) bool {
			return d.String() == ""
		})

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

	instsResp, svcResp, err := d.Deploy(ctx, opts, args...)
	if err != nil {
		return fmt.Errorf("could not prepare deployment: %w", err)
	}

	insts, err := instsResp.AllOrErr()
	if err != nil {
		return err
	}

	if !opts.Follow {
		if len(insts) == 1 && opts.Output == "" {
			// No need to check for error, we check if-nil inside PrettyPrintInstance.
			svc, _ := svcResp.FirstOrErr()
			utils.PrettyPrintInstance(ctx, opts.Metro, insts[0], svc, !opts.NoStart)
			return nil
		}

		return utils.PrintInstances(ctx, opts.Output, *instsResp)
	}

	var uuids []string

	for _, inst := range insts {
		uuids = append(uuids, inst.UUID)
	}

	return logs.Logs(ctx, &logs.LogOptions{
		Auth:   opts.Auth,
		Client: opts.Client,
		Metro:  opts.Metro,
		Follow: true,
		Tail:   -1,
	}, uuids...)
}
