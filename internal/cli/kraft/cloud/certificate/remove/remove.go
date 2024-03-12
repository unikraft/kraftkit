// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
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
	Output string `long:"output" short:"o" usage:"Set output format. Options: table,yaml,json,list" default:"table"`
	All    bool   `long:"all" usage:"Remove all certificates"`

	metro string
	token string
}

// Remove a KraftCloud certificate.
func Remove(ctx context.Context, opts *RemoveOptions, args ...string) error {
	if opts == nil {
		opts = &RemoveOptions{}
	}

	return opts.Run(ctx, args)
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&RemoveOptions{}, cobra.Command{
		Short:   "Remove a certificate",
		Use:     "remove [FLAGS] [UUID|NAME [UUID|NAME]...]",
		Aliases: []string{"del", "delete", "rm"},
		Args:    cobra.ArbitraryArgs,
		Example: heredoc.Doc(`
			# Remove a KraftCloud certificate by UUID
			$ kraft cloud certificate remove fd1684ea-7970-4994-92d6-61dcc7905f2b

			# Remove a KraftCloud certificate by name
			$ kraft cloud certificate remove my-certificate-431342

			# Remove multiple KraftCloud certificates
			$ kraft cloud certificate remove my-certificate-431342 my-certificate-other-2313

			# Remove all KraftCloud certificates
			$ kraft cloud certificate remove --all
		`),
		Long: heredoc.Doc(`
			Remove a KraftCloud certificate.
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

func (opts *RemoveOptions) Pre(cmd *cobra.Command, args []string) error {
	if !opts.All && len(args) == 0 {
		return fmt.Errorf("either specify a certificate name or UUID, or use the --all flag")
	}

	err := utils.PopulateMetroToken(cmd, &opts.metro, &opts.token)
	if err != nil {
		return fmt.Errorf("could not populate metro and token: %w", err)
	}

	return nil
}

func (opts *RemoveOptions) Run(ctx context.Context, args []string) error {
	auth, err := config.GetKraftCloudAuthConfig(ctx, opts.token)
	if err != nil {
		return fmt.Errorf("could not retrieve credentials: %w", err)
	}

	client := kraftcloud.NewCertificatesClient(
		kraftcloud.WithToken(config.GetKraftCloudTokenAuthConfig(*auth)),
	)

	if opts.All {
		certListResp, err := client.WithMetro(opts.metro).List(ctx)
		if err != nil {
			return fmt.Errorf("could not list certificates: %w", err)
		}

		log.G(ctx).Infof("Removing %d certificate(s)", len(certListResp))

		uuids := make([]string, 0, len(certListResp))
		for _, certItem := range certListResp {
			uuids = append(uuids, certItem.UUID)
		}

		if _, err := client.WithMetro(opts.metro).DeleteByUUIDs(ctx, uuids...); err != nil {
			return fmt.Errorf("removing %d certificate(s): %w", len(certListResp), err)
		}
		return nil
	}

	log.G(ctx).Infof("Removing %d certificate(s)", len(args))

	allUUIDs := true
	allNames := true
	for _, arg := range args {
		if utils.IsUUID(arg) {
			allNames = false
		} else {
			allUUIDs = false
		}
		if !(allUUIDs || allNames) {
			break
		}
	}

	switch {
	case allUUIDs:
		if _, err := client.WithMetro(opts.metro).DeleteByUUIDs(ctx, args...); err != nil {
			return fmt.Errorf("removing %d certificate(s): %w", len(args), err)
		}
	case allNames:
		if _, err := client.WithMetro(opts.metro).DeleteByNames(ctx, args...); err != nil {
			return fmt.Errorf("removing %d certificate(s): %w", len(args), err)
		}
	default:
		for _, arg := range args {
			log.G(ctx).Infof("Removing certificate %s", arg)

			if utils.IsUUID(arg) {
				_, err = client.WithMetro(opts.metro).DeleteByUUIDs(ctx, arg)
			} else {
				_, err = client.WithMetro(opts.metro).DeleteByNames(ctx, arg)
			}

			if err != nil {
				return fmt.Errorf("could not remove certificate %s: %w", arg, err)
			}
		}
	}

	return nil
}
