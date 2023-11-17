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

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/cloud/instance/create"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/tui/processtree"
	"kraftkit.sh/unikraft/app"

	kraftcloud "sdk.kraft.cloud"
)

type DeployOptions struct {
	Auth      *config.AuthConfig        `noattribute:"true"`
	Client    kraftcloud.KraftCloud     `noattribute:"true"`
	Env       []string                  `local:"true" long:"env" short:"e" usage:"Environmental variables"`
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
	Timeout   time.Duration             `local:"true" long:"timeout" usage:"Set the timeout for remote procedure calls"`
	Workdir   string                    `local:"true" long:"workdir" short:"w" usage:"Set an alternative working directory (default is cwd)"`
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&DeployOptions{}, cobra.Command{
		Short:   "Deploy your application",
		Use:     "deploy",
		Aliases: []string{"launch"},
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "kraftcloud",
		},
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

	ctx, err := packmanager.WithDefaultUmbrellaManagerInContext(cmd.Context())
	if err != nil {
		return err
	}

	cmd.SetContext(ctx)

	return nil
}

var (
	textRed    = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render
	textGreen  = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render
	textYellow = lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Render
	textGray   = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render
)

func (opts *DeployOptions) Run(ctx context.Context, args []string) error {
	var err error

	opts.Auth, err = config.GetKraftCloudLoginFromContext(ctx)
	if err != nil {
		return fmt.Errorf("could not retrieve credentials: %w", err)
	}

	opts.Client = kraftcloud.NewClient(
		kraftcloud.WithToken(opts.Auth.Token),
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

	if opts.Name == "" {
		opts.Name = filepath.Base(opts.Workdir)
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

	packs, err := d.Prepare(ctx, opts, args...)
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
	if opts.Name[0:17] == "index.unikraft.io" {
		opts.Name = opts.Name[6:]
	}

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
					images, err := opts.Client.Images().WithMetro(opts.Metro).List(ctx, nil)
					if err != nil {
						return fmt.Errorf("could not check list of images: %w", err)
					}

					for _, image := range images {
						split := strings.Split(image.Digest, "/")
						if !strings.HasPrefix(split[len(split)-1], digest) {
							continue
						}

						break checkRemoteImages
					}
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

	instance, err := create.Create(ctx, &create.CreateOptions{
		Env:      opts.Env,
		Memory:   opts.Memory,
		Ports:    opts.Ports,
		Replicas: opts.Replicas,
		Start:    !opts.NoStart,
		Name:     opts.Name,
		Metro:    opts.Metro,
	}, append([]string{opts.Name}, args...)...)
	if err != nil {
		return fmt.Errorf("could not create instance: %w", err)
	}

	log.G(ctx).
		WithField("uuid", instance.UUID).
		Debug("created instance")

	for {
		status, err := opts.Client.Instances().WithMetro(opts.Metro).Status(ctx, instance.UUID)
		if err != nil {
			return fmt.Errorf("could not get instance status: %w", err)
		}

		if string(*status) == "starting" {
			continue
		}

		break
	}

	fqdn := instance.FQDN
	if len(fqdn) > 0 {
		for _, port := range opts.Ports {
			if strings.HasPrefix(port, "443") {
				fqdn = "https://" + fqdn
				break
			}
		}
	}

	var color func(...string) string
	if instance.Status == "running" || instance.Status == "starting" {
		color = textGreen
	} else if instance.Status == "stopped" {
		color = textRed
	} else {
		color = textYellow
	}

	out := iostreams.G(ctx).Out

	fmt.Fprintf(out, "\n%s%s%s %s\n", textGray("["), color("●"), textGray("]"), instance.UUID)
	fmt.Fprintf(out, "     %s: %s\n", textGray("state"), color(instance.Status))
	if len(fqdn) > 0 {
		fmt.Fprintf(out, "       %s: %s\n", textGray("dns"), fqdn)
	}
	fmt.Fprintf(out, " %s: %.2f ms\n", textGray("boot time"), float64(instance.BootTimeUS)/1000)
	fmt.Fprintf(out, "    %s: %d MiB\n", textGray("memory"), instance.MemoryMB)
	fmt.Fprintf(out, "      %s: %s\n\n", textGray("args"), strings.Join(instance.Args, " "))

	if (instance.Status != "running" || instance.Status == "starting") && !opts.NoStart {
		log.G(ctx).Info("it looks like the instance did not come online, to view logs run:")
		fmt.Fprintf(out, "\n    kraft cloud instance logs %s\n\n", instance.UUID)
	}

	return nil
}
