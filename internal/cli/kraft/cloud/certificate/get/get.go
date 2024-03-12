// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package get

import (
	"context"
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	kraftcloud "sdk.kraft.cloud"
	kccerts "sdk.kraft.cloud/certificates"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/cloud/utils"
)

type GetOptions struct {
	Output string `long:"output" short:"o" usage:"Set output format. Options: table,yaml,json,list" default:"table"`

	metro string
	token string
}

// Status of a KraftCloud certificate.
func Get(ctx context.Context, opts *GetOptions, args ...string) error {
	if opts == nil {
		opts = &GetOptions{}
	}

	return opts.Run(ctx, args)
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&GetOptions{}, cobra.Command{
		Short:   "Retrieve the status of a certificate",
		Use:     "get [FLAGS] UUID|NAME",
		Args:    cobra.ExactArgs(1),
		Aliases: []string{"status", "info"},
		Example: heredoc.Doc(`
			# Retrieve information about a kraftcloud certificate by UUID
			$ kraft cloud certificate get fd1684ea-7970-4994-92d6-61dcc7905f2b

			# Retrieve information about a kraftcloud certificate by name
			$ kraft cloud certificate get my-certificate-431342
		`),
		Long: heredoc.Doc(`
			Retrieve the status of a certificate.
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

func (opts *GetOptions) Pre(cmd *cobra.Command, _ []string) error {
	err := utils.PopulateMetroToken(cmd, &opts.metro, &opts.token)
	if err != nil {
		return fmt.Errorf("could not populate metro and token: %w", err)
	}

	return nil
}

func (opts *GetOptions) Run(ctx context.Context, args []string) error {
	auth, err := config.GetKraftCloudAuthConfig(ctx, opts.token)
	if err != nil {
		return fmt.Errorf("could not retrieve credentials: %w", err)
	}

	client := kraftcloud.NewCertificatesClient(
		kraftcloud.WithToken(config.GetKraftCloudTokenAuthConfig(*auth)),
	)

	var certs []kccerts.GetResponseItem
	if utils.IsUUID(args[0]) {
		certs, err = client.WithMetro(opts.metro).GetByUUIDs(ctx, args[0])
	} else {
		certs, err = client.WithMetro(opts.metro).GetByNames(ctx, args[0])
	}
	if err != nil {
		return fmt.Errorf("could not get certificate: %w", err)
	}

	return utils.PrintCertificates(ctx, opts.Output, certs...)
}
