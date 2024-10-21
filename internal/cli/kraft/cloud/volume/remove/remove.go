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

	cloud "sdk.kraft.cloud"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/cloud/utils"
	"kraftkit.sh/log"
)

type RemoveOptions struct {
	Auth   *config.AuthConfig `noattribute:"true"`
	Client cloud.KraftCloud   `noattribute:"true"`
	All    bool               `long:"all" short:"a" usage:"Remove all volumes that are not attached"`
	Metro  string             `noattribute:"true"`
	Token  string             `noattribute:"true"`
}

// Remove a UnikraftCloud persistent volume.
func Remove(ctx context.Context, opts *RemoveOptions, args ...string) error {
	if opts == nil {
		opts = &RemoveOptions{}
	}

	return opts.Run(ctx, args)
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&RemoveOptions{}, cobra.Command{
		Short:   "Permanently delete persistent volume(s)",
		Use:     "remove [FLAGS] [UUID|NAME [UUID|NAME]...]",
		Args:    cobra.MinimumNArgs(0),
		Aliases: []string{"rm", "delete"},
		Example: heredoc.Doc(`
			# Remove a volume by UUID
			$ kraft cloud volume remove fd1684ea-7970-4994-92d6-61dcc7905f2b

			# Remove a volume by name
			$ kraft cloud volume remove my-vol-431342

			# Remove multiple volumes
			$ kraft cloud volume remove my-vol-431342 my-vol-other-2313

			# Remove all volumes
			$ kraft cloud volume remove --all
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "cloud-vol",
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

func (opts *RemoveOptions) Run(ctx context.Context, args []string) error {
	var err error

	if opts.All && len(args) > 0 {
		return fmt.Errorf("cannot specify volumes and use '--all' flag")
	}

	if opts.Auth == nil {
		opts.Auth, err = config.GetUnikraftCloudAuthConfig(ctx, opts.Token)
		if err != nil {
			return fmt.Errorf("could not retrieve credentials: %w", err)
		}
	}

	if opts.Client == nil {
		opts.Client = cloud.NewClient(
			cloud.WithToken(config.GetUnikraftCloudTokenAuthConfig(*opts.Auth)),
		)
	}

	if opts.All {
		volListResp, err := opts.Client.Volumes().WithMetro(opts.Metro).List(ctx)
		if err != nil {
			return fmt.Errorf("could not list volumes: %w", err)
		}

		vols, err := volListResp.AllOrErr()
		if err != nil {
			return fmt.Errorf("could not list volumes: %w", err)
		}

		if len(vols) == 0 {
			log.G(ctx).Info("no volumes found")
			return nil
		}

		uuids := make([]string, 0, len(vols))
		for _, vol := range vols {
			uuids = append(uuids, vol.UUID)
		}

		log.G(ctx).Infof("removing %d volumes(s)", len(uuids))

		if _, err := opts.Client.Volumes().WithMetro(opts.Metro).Delete(ctx, uuids...); err != nil {
			return fmt.Errorf("removing %d volumes(s): %w", len(uuids), err)
		}

		return nil
	}

	log.G(ctx).Infof("removing %d volume(s)", len(args))

	delResp, err := opts.Client.Volumes().WithMetro(opts.Metro).Delete(ctx, args...)
	if err != nil {
		return fmt.Errorf("deleting %d volume(s): %w", len(args), err)
	}
	if _, err = delResp.AllOrErr(); err != nil {
		return fmt.Errorf("deleting %d volume(s): %w", len(args), err)
	}

	return nil
}
