// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package purge

import (
	"context"
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	crm "kraftkit.sh/internal/cli/kraft/cloud/certificate/remove"
	mrm "kraftkit.sh/internal/cli/kraft/cloud/image/remove"
	irm "kraftkit.sh/internal/cli/kraft/cloud/instance/remove"
	itrm "kraftkit.sh/internal/cli/kraft/cloud/instance/template/remove"
	srm "kraftkit.sh/internal/cli/kraft/cloud/service/remove"
	"kraftkit.sh/internal/cli/kraft/cloud/utils"
	vrm "kraftkit.sh/internal/cli/kraft/cloud/volume/remove"
	vtrm "kraftkit.sh/internal/cli/kraft/cloud/volume/template/remove"

	"kraftkit.sh/log"
	"kraftkit.sh/tui/selection"

	kraftcloud "sdk.kraft.cloud"
)

type PurgeOptions struct {
	Force         bool                  `local:"true" long:"force" short:"f" usage:"Continue removing everything regardless of any errors." default:"false"`
	All           bool                  `local:"true" long:"all" short:"a" usage:"Remove from all metros." default:"false"`
	KeepImages    bool                  `local:"true" long:"keep-images" usage:"Do not remove images." default:"false"`
	Metro         string                `noattribute:"true"`
	Token         string                `noattribute:"true"`
	Auth          *config.AuthConfig    `noattribute:"true"`
	AllowInsecure bool                  `noattribute:"true"`
	Client        kraftcloud.KraftCloud `noattribute:"true"`

	imagesRemoved bool
}

type purgeAccept string

func (p purgeAccept) String() string {
	return string(p)
}

const (
	PurgeAcceptYes purgeAccept = "yes"
	PurgeAcceptNo  purgeAccept = "no"
)

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&PurgeOptions{}, cobra.Command{
		Short:   "Remove everything on KraftCloud",
		Use:     "purge",
		Args:    cobra.NoArgs,
		Aliases: []string{"p"},
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "kraftcloud",
		},
		Long: heredoc.Doc(`
			Remove everything on KraftCloud.
		`),
		Example: heredoc.Doc(`
			# Remove everything on KraftCloud from the default metro
			$ kraft cloud purge

			# Remove everything on KraftCloud from all metros
			$ kraft cloud purge -a

			# Remove everything on KraftCloud from all metros but keep images
			$ kraft cloud purge -a --keep-images

			# Remove everything on KraftCloud and continue regardless of any errors
			$ kraft cloud purge -f
		`),
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

// Purge removes everything from a metro.
func Purge(ctx context.Context, opts *PurgeOptions) error {
	// 1. Instances
	err := irm.Remove(ctx, &irm.RemoveOptions{
		Metro:         opts.Metro,
		Token:         opts.Token,
		AllowInsecure: opts.AllowInsecure,
		All:           true,
	})
	if err != nil && !opts.Force {
		return fmt.Errorf("could not remove instances: %w", err)
	}

	// 2. Certificates
	err = crm.Remove(ctx, &crm.RemoveOptions{
		Metro:         opts.Metro,
		Token:         opts.Token,
		AllowInsecure: opts.AllowInsecure,
		All:           true,
		Output:        "list",
	})
	if err != nil && !opts.Force {
		return fmt.Errorf("could not remove certificates: %w", err)
	}

	// 3. Services
	err = srm.Remove(ctx, &srm.RemoveOptions{
		Metro:         opts.Metro,
		Token:         opts.Token,
		AllowInsecure: opts.AllowInsecure,
		All:           true,
	})
	if err != nil && !opts.Force {
		return fmt.Errorf("could not remove services: %w", err)
	}

	// 4. Volumes
	err = vrm.Remove(ctx, &vrm.RemoveOptions{
		Metro:         opts.Metro,
		Token:         opts.Token,
		AllowInsecure: opts.AllowInsecure,
		All:           true,
	})
	if err != nil && !opts.Force {
		return fmt.Errorf("could not remove volumes: %w", err)
	}

	// 5. Volume Templates
	err = vtrm.Remove(ctx, &vtrm.RemoveOptions{
		Metro:         opts.Metro,
		Token:         opts.Token,
		AllowInsecure: opts.AllowInsecure,
		All:           true,
	})
	if err != nil && !opts.Force {
		return fmt.Errorf("could not remove volume templates: %w", err)
	}

	// 6. Instance Templates
	err = itrm.Remove(ctx, &itrm.RemoveOptions{
		Metro:         opts.Metro,
		Token:         opts.Token,
		AllowInsecure: opts.AllowInsecure,
		All:           true,
	})
	if err != nil && !opts.Force {
		return fmt.Errorf("could not remove instance templates: %w", err)
	}

	// 7. Images
	if !opts.imagesRemoved && !opts.KeepImages {
		opts.imagesRemoved = true

		err = mrm.Remove(ctx, &mrm.RemoveOptions{
			Metro: opts.Metro,
			Token: opts.Token,
			All:   true,
		})
		if err != nil && !opts.Force {
			return fmt.Errorf("could not remove images: %w", err)
		}
	}

	return nil
}

func (opts *PurgeOptions) Pre(cmd *cobra.Command, _ []string) error {
	err := utils.PopulateMetroToken(cmd, &opts.Metro, &opts.Token, &opts.AllowInsecure)

	// Ignore the metro error if the user wants to remove from all metros
	if err != nil && !(opts.All && strings.Contains(err.Error(), "metro")) {
		return fmt.Errorf("could not populate metro and token: %w", err)
	}

	return nil
}

func (opts *PurgeOptions) Run(ctx context.Context, _ []string) error {
	var metros []string
	var err error

	if opts.Auth == nil {
		opts.Auth, err = config.GetKraftCloudAuthConfig(ctx, opts.Token)
		if err != nil {
			return fmt.Errorf("could not retrieve credentials: %w", err)
		}
	}

	if opts.Client == nil {
		opts.Client = kraftcloud.NewClient(
			kraftcloud.WithToken(config.GetKraftCloudTokenAuthConfig(*opts.Auth)),
			kraftcloud.WithAllowInsecure(opts.AllowInsecure),
		)
	}

	if opts.All {
		metrosResp, err := opts.Client.Metros().List(ctx, true)
		if err != nil {
			return fmt.Errorf("could not list metros: %w", err)
		}

		for _, metro := range metrosResp {
			if !metro.Online {
				log.G(ctx).WithField("metro", metro.Code).Warn("skipping offline metro")
				continue
			}

			metros = append(metros, metro.Code)
		}
	} else {
		metros = append(metros, opts.Metro)
	}

	if len(metros) != 0 && !config.G[config.KraftKit](ctx).NoPrompt {
		purgeYesNo, err := selection.Select(
			fmt.Sprintf("You are about to delete all instances, instance templates, services, certificates, volumes, volume templates, images from the following metros: %s. Proceed?",
				strings.Join(metros, ", "),
			),
			PurgeAcceptNo,
			PurgeAcceptYes,
		)
		if err != nil {
			return err
		}

		if purgeYesNo == nil || *purgeYesNo == PurgeAcceptNo {
			return nil
		}
	}

	for _, metro := range metros {
		opts.Metro = metro

		log.G(ctx).WithField("metro", metro).Info("purging")

		if err := Purge(ctx, opts); err != nil {
			if !opts.Force {
				return fmt.Errorf("could not purge metro %q: %w", metro, err)
			}

			log.G(ctx).WithField("metro", metro).WithError(err).Warn("could not fully purge")
		}
	}

	return nil
}
