// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2025, Unikraft GmbH and The KraftKit Authors.
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
	Auth   *config.AuthConfig    `noattribute:"true"`
	Client kraftcloud.KraftCloud `noattribute:"true"`
	All    bool                  `long:"all" short:"a" usage:"Remove all templates"`
	Metro  string                `noattribute:"true"`
	Token  string                `noattribute:"true"`
}

// Remove a KraftCloud persistent instance template.
func Remove(ctx context.Context, opts *RemoveOptions, args ...string) error {
	if opts == nil {
		opts = &RemoveOptions{}
	}

	return opts.Run(ctx, args)
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&RemoveOptions{}, cobra.Command{
		Short:   "Permanently delete instance template(s)",
		Use:     "remove [FLAGS] [UUID|NAME [UUID|NAME]...]",
		Args:    cobra.MinimumNArgs(0),
		Aliases: []string{"rm", "delete"},
		Example: heredoc.Doc(`
			# Remove a template by UUID
			$ kraft cloud instance template remove fd1684ea-7970-4994-92d6-61dcc7905f2b

			# Remove a template by name
			$ kraft cloud instance template remove my-template-1

			# Remove multiple templates
			$ kraft cloud instance template remove my-template-1 my-template-2

			# Remove all templates
			$ kraft cloud instance template remove --all
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "kraftcloud-instance-template",
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
		return fmt.Errorf("cannot specify templates and use '--all' flag")
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

	if opts.All {
		volListResp, err := opts.Client.Instances().WithMetro(opts.Metro).ListTemplate(ctx)
		if err != nil {
			return fmt.Errorf("could not list instance templates: %w", err)
		}

		vols, err := volListResp.AllOrErr()
		if err != nil {
			return fmt.Errorf("could not list instance templates: %w", err)
		}

		if len(vols) == 0 {
			log.G(ctx).Info("no instance templates found")
			return nil
		}

		uuids := make([]string, 0, len(vols))
		for _, vol := range vols {
			uuids = append(uuids, vol.UUID)
		}

		log.G(ctx).Infof("removing %d instance template(s)", len(uuids))

		if _, err := opts.Client.Instances().WithMetro(opts.Metro).DeleteTemplate(ctx, uuids...); err != nil {
			return fmt.Errorf("removing %d instance template(s): %w", len(uuids), err)
		}

		return nil
	}

	log.G(ctx).Infof("removing %d instance template(s)", len(args))

	delResp, err := opts.Client.Instances().WithMetro(opts.Metro).DeleteTemplate(ctx, args...)
	if err != nil {
		return fmt.Errorf("deleting %d instance template(s): %w", len(args), err)
	}
	if _, err = delResp.AllOrErr(); err != nil {
		return fmt.Errorf("deleting %d instance template(s): %w", len(args), err)
	}

	return nil
}
