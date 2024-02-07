// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package quotas

import (
	"context"
	"fmt"
	"os"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/cloud/utils"
	"kraftkit.sh/log"

	kraftcloud "sdk.kraft.cloud"
)

type QuotasOptions struct {
	Output string `local:"true" long:"output" short:"o" usage:"Set output format. Options: table,yaml,json,full" default:"table"`

	metro string
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&QuotasOptions{}, cobra.Command{
		Short:   "View your resource quota on KraftCloud",
		Use:     "quotas",
		Aliases: []string{"q", "quota"},
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "kraftcloud",
		},
		Example: heredoc.Doc(`
			# View your resource quota on KraftCloud
			$ kraft cloud quota

			# View your resource quota on KraftCloud in JSON format
			$ kraft cloud quota -o json
		`),
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *QuotasOptions) Pre(cmd *cobra.Command, _ []string) error {
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

func (opts *QuotasOptions) Run(ctx context.Context, _ []string) error {
	auth, err := config.GetKraftCloudAuthConfigFromContext(ctx)
	if err != nil {
		return fmt.Errorf("could not retrieve credentials: %w", err)
	}

	client := kraftcloud.NewUsersClient(
		kraftcloud.WithToken(config.GetKraftCloudTokenAuthConfig(*auth)),
	)

	quotas, err := client.WithMetro(opts.metro).Quotas(ctx)
	if err != nil {
		return fmt.Errorf("could not get quotas: %w", err)
	}

	return utils.PrintQuotas(ctx, opts.Output, *quotas)
}
