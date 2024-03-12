// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package list

import (
	"context"
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	kraftcloud "sdk.kraft.cloud"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/cloud/utils"
)

type ListOptions struct {
	Output string `long:"output" short:"o" usage:"Set output format. Options: table,yaml,json,list" default:"table"`

	metro string
	token string
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&ListOptions{}, cobra.Command{
		Short:   "List certificates",
		Use:     "list [FLAGS]",
		Args:    cobra.NoArgs,
		Aliases: []string{"ls"},
		Example: heredoc.Doc(`
			# List all TLS certificates in your account.
			$ kraft cloud certificate list
		`),
		Long: heredoc.Doc(`
			List all TLS certificates in your account.
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "kraftcloud-certificate",
		},
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *ListOptions) Pre(cmd *cobra.Command, _ []string) error {
	err := utils.PopulateMetroToken(cmd, &opts.metro, &opts.token)
	if err != nil {
		return fmt.Errorf("could not populate metro and token: %w", err)
	}

	return nil
}

func (opts *ListOptions) Run(ctx context.Context, args []string) error {
	auth, err := config.GetKraftCloudAuthConfig(ctx, opts.token)
	if err != nil {
		return fmt.Errorf("could not retrieve credentials: %w", err)
	}

	client := kraftcloud.NewCertificatesClient(
		kraftcloud.WithToken(config.GetKraftCloudTokenAuthConfig(*auth)),
	)

	certListResp, err := client.WithMetro(opts.metro).List(ctx)
	if err != nil {
		return fmt.Errorf("could not list certificates: %w", err)
	}
	if len(certListResp) == 0 {
		return nil
	}

	uuids := make([]string, 0, len(certListResp))
	for _, certItem := range certListResp {
		uuids = append(uuids, certItem.UUID)
	}
	certificates, err := client.WithMetro(opts.metro).GetByUUIDs(ctx, uuids...)
	if err != nil {
		return fmt.Errorf("getting details of %d certificate(s): %w", len(certListResp), err)
	}

	return utils.PrintCertificates(ctx, opts.Output, certificates...)
}
