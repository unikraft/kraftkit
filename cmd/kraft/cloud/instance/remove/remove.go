// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package remove

import (
	"fmt"
	"os"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	kraftcloud "sdk.kraft.cloud"
	kcinstance "sdk.kraft.cloud/instance"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/log"
)

type Remove struct {
	Output string `long:"output" short:"o" usage:"Set output format" default:"table"`
	All    bool   `long:"all" usage:"Stop all instances"`

	metro string
}

func New() *cobra.Command {
	cmd, err := cmdfactory.New(&Remove{}, cobra.Command{
		Short:   "Delete an instance",
		Use:     "delete UUID",
		Aliases: []string{"del", "delete", "rm"},
		Args:    cobra.ArbitraryArgs,
		Long: heredoc.Doc(`
			Delete a KraftCloud instance.
		`),
		Example: heredoc.Doc(`
			# Delete a KraftCloud instance
			$ kraft cloud instance delete fd1684ea-7970-4994-92d6-61dcc7905f2b
	`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "kraftcloud-instance",
		},
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *Remove) Pre(cmd *cobra.Command, args []string) error {
	if !opts.All && len(args) == 0 {
		return fmt.Errorf("either specify an instance UUID or --all flag")
	}

	opts.metro = cmd.Flag("metro").Value.String()
	if opts.metro == "" {
		opts.metro = os.Getenv("KRAFTCLOUD_METRO")
	}
	if opts.metro == "" {
		return fmt.Errorf("kraftcloud metro is unset")
	}
	log.G(cmd.Context()).WithField("metro", opts.metro).Debug("using")
	return nil
}

func (opts *Remove) Run(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	auth, err := config.GetKraftCloudLoginFromContext(ctx)
	if err != nil {
		return fmt.Errorf("could not retrieve credentials: %w", err)
	}

	client := kcinstance.NewInstancesClient(
		kraftcloud.WithToken(auth.Token),
	)

	if opts.All {
		instances, err := client.WithMetro(opts.metro).List(ctx)
		if err != nil {
			return fmt.Errorf("could not get list of all instances: %w", err)
		}

		for _, instance := range instances {
			log.G(ctx).Infof("removing %s", instance.UUID)

			if err := client.WithMetro(opts.metro).Delete(ctx, instance.UUID); err != nil {
				log.G(ctx).Error("could not stop instance: %w", err)
			}
		}

		return nil
	}

	for _, uuid := range args {
		log.G(ctx).Infof("removing %s", uuid)

		if err := client.WithMetro(opts.metro).Delete(ctx, uuid); err != nil {
			return fmt.Errorf("could not create instance: %w", err)
		}
	}

	return nil
}
