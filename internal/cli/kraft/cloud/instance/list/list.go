// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package list

import (
	"fmt"
	"os"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	kraftcloud "sdk.kraft.cloud"
	kcinstance "sdk.kraft.cloud/instance"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/cloud/utils"
	"kraftkit.sh/log"
)

type List struct {
	Output string `long:"output" short:"o" usage:"Set output format" default:"table"`

	metro string
}

func New() *cobra.Command {
	cmd, err := cmdfactory.New(&List{}, cobra.Command{
		Short:   "List instances",
		Use:     "ls [FLAGS]",
		Aliases: []string{"list"},
		Example: heredoc.Doc(`
			# List all instances in your account.
			$ kraft cloud instances list
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

func (opts *List) Pre(cmd *cobra.Command, _ []string) error {
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

func (opts *List) Run(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	auth, err := config.GetKraftCloudLoginFromContext(ctx)
	if err != nil {
		return fmt.Errorf("could not retrieve credentials: %w", err)
	}

	client := kcinstance.NewInstancesClient(
		kraftcloud.WithToken(auth.Token),
	)

	uuids, err := client.WithMetro(opts.metro).List(ctx)
	if err != nil {
		return fmt.Errorf("could not list instances: %w", err)
	}

	// TODO(nderjung): For now, the KraftCloud API does not support
	// returning the full details of each instance.  Temporarily request a
	// status for each instance.
	var instances []kcinstance.Instance
	for _, uuid := range uuids {
		instance, err := client.WithMetro(opts.metro).Status(ctx, uuid.UUID)
		if err != nil {
			return fmt.Errorf("could not get instance status: %w", err)
		}

		instances = append(instances, *instance)
	}

	return utils.PrintInstances(ctx, opts.Output, instances...)
}
