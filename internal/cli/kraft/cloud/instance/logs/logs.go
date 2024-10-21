// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package logs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	kraftcloud "sdk.kraft.cloud"
	kcinstances "sdk.kraft.cloud/instances"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/cloud/utils"
	"kraftkit.sh/internal/cli/kraft/logs"
	"kraftkit.sh/internal/waitgroup"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"
)

type LogOptions struct {
	Auth     *config.AuthConfig    `noattribute:"true"`
	Client   kraftcloud.KraftCloud `noattribute:"true"`
	Follow   bool                  `local:"true" long:"follow" short:"f" usage:"Follow the logs of the instance every half second" default:"false"`
	Metro    string                `noattribute:"true"`
	NoPrefix bool                  `long:"no-prefix" usage:"When logging multiple machines, do not prefix each log line with the name"`
	Tail     int                   `local:"true" long:"tail" short:"n" usage:"Show the last given lines from the logs" default:"-1"`
	Token    string                `noattribute:"true"`
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
		Short:   "Get console output of instances",
		Use:     "logs [FLAG] UUID|NAME",
		Args:    cobra.MinimumNArgs(1),
		Aliases: []string{"log"},
		Example: heredoc.Doc(`
			# Get all console output of a instance by UUID
			$ kraft cloud instance logs 77d0316a-fbbe-488d-8618-5bf7a612477a

			# Get all console output of a instance by name
			$ kraft cloud instance logs my-instance-431342

			# Get the last 20 lines of a instance by name
			$ kraft cloud instance logs my-instance-431342 --tail 20

			# Get the last lines of a instance by name continuously
			$ kraft cloud instance logs my-instance-431342 --follow

			# Get the last 10 lines of a instance by name continuously
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
	err := utils.PopulateMetroToken(cmd, &opts.Metro, &opts.Token)
	if err != nil {
		return fmt.Errorf("could not populate metro and token: %w", err)
	}

	if opts.Tail < -1 {
		return fmt.Errorf("invalid value for --tail: %d, should be -1 for all logs, or positive for length of truncated logs", opts.Tail)
	}

	return nil
}

func (opts *LogOptions) Run(ctx context.Context, args []string) error {
	return Logs(ctx, opts, args...)
}

func Logs(ctx context.Context, opts *LogOptions, args ...string) error {
	var err error

	if opts.Auth == nil {
		opts.Auth, err = config.GetKraftCloudAuthConfig(ctx, opts.Token)
		if err != nil {
			return fmt.Errorf("could not retrieve credentials: %w", err)
		}
	}

	if opts.Client == nil {
		opts.Client = kraftcloud.NewClient(
			kraftcloud.WithToken(config.GetKraftCloudTokenAuthConfig(*opts.Auth)),
		)
	}

	longestName := 0

	if len(args) > 1 && !opts.NoPrefix {
		for _, instance := range args {
			if len(instance) > longestName {
				longestName = len(instance)
			}
		}
	} else {
		opts.NoPrefix = true
	}

	var errGroup []error
	observations := waitgroup.WaitGroup[string]{}

	for _, instance := range args {
		instance := instance
		prefix := ""
		if !opts.NoPrefix {
			prefix = instance + strings.Repeat(" ", longestName-len(instance))
		}

		consumer, err := logs.NewColorfulConsumer(iostreams.G(ctx), !config.G[config.KraftKit](ctx).NoColor, prefix)
		if err != nil {
			errGroup = append(errGroup, err)
		}

		logChan, errChan, err := opts.Client.Instances().WithMetro(opts.Metro).TailLogs(ctx, instance, opts.Follow, opts.Tail, 500*time.Millisecond)
		if err != nil {
			return fmt.Errorf("initializing log tailing: %w", err)
		}

		observations.Add(instance)

		var inst *kcinstances.GetResponseItem

		// Continuously check the state in a separate thread every 1 second.
		go func() {
			for {
				resp, err := opts.Client.Instances().WithMetro(opts.Metro).Get(ctx, instance)
				if err != nil {
					// Likely there was an issue performing the request; so we'll just
					// skip and attempt to retrieve more logs.
					if !errors.Is(err, io.EOF) {
						log.G(ctx).Error(err)
					}

					continue
				}

				inst, err = resp.FirstOrErr()
				if err != nil {
					errGroup = append(errGroup, err)
				}

				if len(observations.Items()) == 0 {
					return
				}

				time.Sleep(time.Second)
			}
		}()

		go func() {
			defer observations.Done(instance)

			for {
				select {
				case <-ctx.Done():
					return
				case err := <-errChan:
					if err != nil && !strings.Contains(err.Error(), "operation timed out") && !errors.Is(err, io.EOF) {
						errGroup = append(errGroup, err)
						return
					}
				case line, ok := <-logChan:
					if ok {
						consumer.Consume(line)
					} else {
						if inst != nil && inst.State == kcinstances.InstanceStateStopped {
							consumer.Consume(
								"",
								fmt.Sprintf("The instance has exited (%s).", inst.DescribeStopReason()),
								"",
								"To see more details about why, run:",
								"",
								fmt.Sprintf("\tkraft cloud instance get %s", inst.Name),
								"",
							)
						}
						return
					}
				case <-time.After(time.Second):
					// If we have not received anything after 1 second through any of the
					// other channels, check if the instance has stopped and exit if it
					// has.
					if inst != nil && inst.State == kcinstances.InstanceStateStopped {
						consumer.Consume(
							"",
							fmt.Sprintf("The instance has exited (%s).", inst.DescribeStopReason()),
							"",
							"To see more details about why, run:",
							"",
							fmt.Sprintf("\tkraft cloud instance get %s", inst.Name),
							"",
						)

						return
					}
				}
			}
		}()
	}

	observations.Wait()

	return errors.Join(errGroup...)
}
