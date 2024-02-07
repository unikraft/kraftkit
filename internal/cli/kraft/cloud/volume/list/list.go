// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package list

import (
	"context"
	"fmt"
	"os"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	kraftcloud "sdk.kraft.cloud"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/cloud/utils"
	"kraftkit.sh/log"
)

type ListOptions struct {
	Output string `long:"output" short:"o" usage:"Set output format. Options: table,yaml,json,full" default:"table"`
	Watch  bool   `long:"watch" short:"w" usage:"After listing watch for changes."`

	metro string
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&ListOptions{}, cobra.Command{
		Short:   "List all volumes at a metro for your account",
		Use:     "list",
		Aliases: []string{"ls"},
		Long: heredoc.Doc(`
			List all volumes in your account.
		`),
		Example: heredoc.Doc(`
			# List all volumes in your account.
			$ kraft cloud vol ls

			# List all volumes in your account in JSON format.
			$ kraft cloud volume list -o json
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

func (opts *ListOptions) Pre(cmd *cobra.Command, _ []string) error {
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

func (opts *ListOptions) Run(ctx context.Context, args []string) error {
	auth, err := config.GetKraftCloudAuthConfigFromContext(ctx)
	if err != nil {
		return fmt.Errorf("could not retrieve credentials: %w", err)
	}

	client := kraftcloud.NewVolumesClient(
		kraftcloud.WithToken(config.GetKraftCloudTokenAuthConfig(*auth)),
	)

	volumes, err := client.WithMetro(opts.metro).List(ctx)
	if err != nil {
		return fmt.Errorf("could not list volumes: %w", err)
	}

	return utils.PrintVolumes(ctx, opts.Output, volumes...)
}
