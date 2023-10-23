// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package start

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

type Start struct {
	WaitTimeoutMS int    `local:"true" long:"wait_timeout_ms" short:"w" usage:"Timeout to wait for the instance to start in milliseconds"`
	Output        string `long:"output" short:"o" usage:"Set output format" default:"table"`

	metro string
}

func New() *cobra.Command {
	cmd, err := cmdfactory.New(&Start{}, cobra.Command{
		Short: "Start an instance",
		Use:   "start [FLAGS] [PACKAGE]",
		Args:  cobra.ExactArgs(1),
		Example: heredoc.Doc(`
			# Start a KraftCloud instance
			$ kraft cloud instance start 77d0316a-fbbe-488d-8618-5bf7a612477a
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

func (opts *Start) Pre(cmd *cobra.Command, _ []string) error {
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

func (opts *Start) Run(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	auth, err := config.GetKraftCloudLoginFromContext(ctx)
	if err != nil {
		return fmt.Errorf("could not retrieve credentials: %w", err)
	}

	client := kcinstance.NewInstancesClient(
		kraftcloud.WithToken(auth.Token),
	)

	for _, arg := range args {
		log.G(ctx).Infof("starting %s", arg)

		_, err := client.WithMetro(opts.metro).Start(ctx, arg, opts.WaitTimeoutMS)
		if err != nil {
			log.G(ctx).WithError(err).Error("could not start instance")
			continue
		}
	}

	return nil
}
