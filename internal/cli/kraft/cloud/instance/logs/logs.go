// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package logs

import (
	"context"
	"fmt"
	"os"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	kraftcloud "sdk.kraft.cloud"
	"sdk.kraft.cloud/instance"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"
)

type LogOptions struct {
	Tail int `local:"true" long:"tail" short:"n" usage:"Lines of recent logs to display" default:"-1"`

	metro string
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&LogOptions{}, cobra.Command{
		Short: "Get console output of an instance",
		Use:   "logs [UUID]",
		Args:  cobra.ExactArgs(1),
		Example: heredoc.Doc(`
			# Get console output of a kraftcloud instance
			$ kraft cloud inst logs 77d0316a-fbbe-488d-8618-5bf7a612477a
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

func (opts *LogOptions) Pre(cmd *cobra.Command, _ []string) error {
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

func (opts *LogOptions) Run(ctx context.Context, args []string) error {
	auth, err := config.GetKraftCloudLoginFromContext(ctx)
	if err != nil {
		return fmt.Errorf("could not retrieve credentials: %w", err)
	}

	uuid := args[0]

	client := instance.NewInstancesClient(
		kraftcloud.WithToken(auth.Token),
	)

	logs, err := client.WithMetro(opts.metro).Logs(ctx, uuid, opts.Tail, true)
	if err != nil {
		return fmt.Errorf("could not retrieve logs: %w", err)
	}

	fmt.Fprintf(iostreams.G(ctx).Out, "%s\n", logs)

	return nil
}
