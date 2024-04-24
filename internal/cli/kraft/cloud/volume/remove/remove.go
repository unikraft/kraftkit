// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package remove

import (
	"context"
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	kraftcloud "sdk.kraft.cloud"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/cloud/utils"
	"kraftkit.sh/log"
)

type RemoveOptions struct {
	All    bool                  `long:"all" short:"a" usage:"Remove all volumes"`
	Auth   *config.AuthConfig    `noattribute:"true"`
	Client kraftcloud.KraftCloud `noattribute:"true"`
	Metro  string                `noattribute:"true"`
	Token  string                `noattribute:"true"`
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&RemoveOptions{}, cobra.Command{
		Short:   "Permanently delete a persistent volume",
		Use:     "remove UUID [UUID [...]]",
		Args:    cobra.MinimumNArgs(1),
		Aliases: []string{"rm"},
		Long: heredoc.Doc(`
			Permanently delete a persistent volume.
		`),
		Example: heredoc.Doc(`
			# Delete three persistent volumes
			$ kraft cloud volume rm UUID1 UUID2 UUID3
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "kraftcloud-vol",
		},
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *RemoveOptions) Pre(cmd *cobra.Command, _ []string) error {
	err := utils.PopulateMetroToken(cmd, &opts.Metro, &opts.Token)
	if err != nil {
		return fmt.Errorf("could not populate metro and token: %w", err)
	}

	return nil
}

// Remove a KraftCloud volume.
func Remove(ctx context.Context, opts *RemoveOptions, args ...string) error {
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
		)
	}

	if opts.All {
		volListResp, err := opts.Client.Volumes().WithMetro(opts.Metro).List(ctx)
		if err != nil {
			return fmt.Errorf("listing volumes: %w", err)
		}

		volList, err := volListResp.AllOrErr()
		if err != nil {
			return fmt.Errorf("listing volumes: %w", err)
		}

		if len(volList) == 0 {
			log.G(ctx).Info("no volumes found")
			return nil
		}

		vols := make([]string, len(volList))
		for i, vol := range volList {
			vols[i] = vol.UUID
		}

		log.G(ctx).Infof("Deleting %d volume(s)", len(volList))

		delResp, err := opts.Client.Volumes().WithMetro(opts.Metro).Delete(ctx, vols...)
		if err != nil {
			return fmt.Errorf("deleting %d volume(s): %w", len(volList), err)
		}

		if _, err = delResp.AllOrErr(); err != nil {
			return fmt.Errorf("deleting %d volume(s): %w", len(volList), err)
		}
	} else {
		log.G(ctx).Infof("Deleting %d volume(s)", len(args))

		delResp, err := opts.Client.Volumes().WithMetro(opts.Metro).Delete(ctx, args...)
		if err != nil {
			return fmt.Errorf("deleting %d volume(s): %w", len(args), err)
		}
		if _, err = delResp.AllOrErr(); err != nil {
			return fmt.Errorf("deleting %d volume(s): %w", len(args), err)
		}
	}

	return nil
}

func (opts *RemoveOptions) Run(ctx context.Context, args []string) error {
	return Remove(ctx, opts, args...)
}
