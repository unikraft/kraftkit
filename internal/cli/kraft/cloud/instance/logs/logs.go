// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package logs

import (
	"context"
	"fmt"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	kraftcloud "sdk.kraft.cloud"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/cloud/utils"
	"kraftkit.sh/iostreams"
)

type LogOptions struct {
	Follow bool `local:"true" long:"follow" short:"f" usage:"Follow the logs of the instance every half second" default:"false"`
	Tail   int  `local:"true" long:"tail" short:"n" usage:"Show the last given lines from the logs" default:"-1"`

	metro string
	token string
}

// Log retrieves the console output from a KraftCloud instance.
func Log(ctx context.Context, opts *LogOptions, args ...string) error {
	if opts == nil {
		opts = &LogOptions{}
	}

	return opts.Run(ctx, args)
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&LogOptions{}, cobra.Command{
		Short:   "Get console output of an instance",
		Use:     "logs [FLAG] UUID|NAME",
		Args:    cobra.ExactArgs(1),
		Aliases: []string{"log"},
		Example: heredoc.Doc(`
			# Get all console output of a kraftcloud instance by UUID
			$ kraft cloud instance logs 77d0316a-fbbe-488d-8618-5bf7a612477a

			# Get all console output of a kraftcloud instance by name
			$ kraft cloud instance logs my-instance-431342

			# Get the last 20 lines of a kraftcloud instance by name
			$ kraft cloud instance logs my-instance-431342 --tail 20

			# Get the last lines of a kraftcloud instance by name continuously
			$ kraft cloud instance logs my-instance-431342 --follow

			# Get the last 10 lines of a kraftcloud instance by name continuously
			$ kraft cloud instance logs my-instance-431342 --follow --tail 10
		`),
		Long: heredoc.Doc(`
			Get console output of an instance.
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
	err := utils.PopulateMetroToken(cmd, &opts.metro, &opts.token)
	if err != nil {
		return fmt.Errorf("could not populate metro and token: %w", err)
	}

	if opts.Tail < -1 {
		return fmt.Errorf("invalid value for --tail: %d, should be -1 for all logs, or positive for length of truncated logs", opts.Tail)
	}

	return nil
}

func (opts *LogOptions) Run(ctx context.Context, args []string) error {
	auth, err := config.GetKraftCloudAuthConfig(ctx, opts.token)
	if err != nil {
		return fmt.Errorf("could not retrieve credentials: %w", err)
	}

	client := kraftcloud.NewInstancesClient(
		kraftcloud.WithToken(config.GetKraftCloudTokenAuthConfig(*auth)),
	)

	logChan, errChan, err := client.TailLogs(ctx, args[0], opts.Follow, opts.Tail, 500*time.Millisecond)
	if err != nil {
		return fmt.Errorf("initializing log tailing: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-errChan:
			return err
		case line, ok := <-logChan:
			if ok {
				fmt.Fprintf(iostreams.G(ctx).Out, "%s\n", line)
			}
		}
	}
}
