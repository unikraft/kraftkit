// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package logs

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	kraftcloud "sdk.kraft.cloud"
	kcinstances "sdk.kraft.cloud/instances"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/cloud/utils"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"
)

const (
	defaultPageSize  = 4096
	maxPageSize      = defaultPageSize*4 - 1
	maxPossibleBytes = 1024 * 1024 * 1024
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
		return fmt.Errorf("invalid value for --tail: %d, should be -1, or positive", opts.Tail)
	}

	if opts.Follow && opts.Tail == -1 {
		return fmt.Errorf("cannot use --follow without --tail")
	}

	return nil
}

func (opts *LogOptions) logsFetchDecode(ctx context.Context, client kcinstances.InstancesService, image string, offset, limit int) ([]byte, *kcinstances.LogResponseItem, error) {
	var err error
	var resp *kcinstances.LogResponseItem

	if utils.IsUUID(image) {
		resp, err = client.WithMetro(opts.metro).LogByUUID(ctx, image, offset, limit)
	} else {
		resp, err = client.WithMetro(opts.metro).LogByName(ctx, image, offset, limit)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("could not retrieve logs: %w", err)
	}

	outputPart, err := base64.StdEncoding.DecodeString(resp.Output)
	if err != nil {
		return nil, nil, fmt.Errorf("decoding base64 console output: %w", err)
	}

	return outputPart, resp, nil
}

func (opts *LogOptions) Run(ctx context.Context, args []string) error {
	var offset, limit int

	auth, err := config.GetKraftCloudAuthConfig(ctx, opts.token)
	if err != nil {
		return fmt.Errorf("could not retrieve credentials: %w", err)
	}

	client := kraftcloud.NewInstancesClient(
		kraftcloud.WithToken(config.GetKraftCloudTokenAuthConfig(*auth)),
	)

	if opts.Follow {
		offset = -defaultPageSize
		limit = maxPageSize

		for {
			output, resp, err := opts.logsFetchDecode(ctx, client, args[0], offset, limit)
			if err != nil {
				return err
			}

			// Split the output by line and print each line separately to
			// be able to limit the output to the last N lines.
			// Truncate the last line if it's not a full line.
			if len(output) > 0 {
				split := strings.Split(string(output), "\n")
				linesToSkip := len(split) - opts.Tail
				for i, line := range split {
					if i < linesToSkip {
						continue
					}

					if i == len(split)-1 && output[len(output)-1] != '\n' {
						offset = resp.Range.End - len(line)
					} else {
						if i == len(split)-1 {
							offset = resp.Range.End
						}
						fmt.Fprintf(iostreams.G(ctx).Out, "%s\n", line)
					}
				}
			}

			time.Sleep(500 * time.Millisecond)
		}
	}

	var output []byte
	if opts.Tail >= 0 {
		lines := 0
		for i := 1; lines < opts.Tail; i++ {
			offset = i * -defaultPageSize
			limit = defaultPageSize

			outputPart, resp, err := opts.logsFetchDecode(ctx, client, args[0], offset, limit)
			if err != nil {
				return err
			}

			outputPart = append(outputPart, output...)
			output = outputPart

			if resp.Range.Start == resp.Available.Start {
				break
			}

			lines += strings.Count(string(outputPart), "\n")
		}

		split := strings.Split(string(output), "\n")
		start := len(split) - opts.Tail
		if start < 0 {
			start = 0
		}
		for i := start; i < len(split); i++ {
			fmt.Fprintf(iostreams.G(ctx).Out, "%s\n", split[i])
		}
	} else {
		// The same as above, but fetch the max possible amount of logs.
		// Stop only when there are no more logs to fetch.
		for i := 4; ; i = i + 4 {
			offset = i * -defaultPageSize
			limit = maxPageSize

			outputPart, resp, err := opts.logsFetchDecode(ctx, client, args[0], offset, limit)
			if err != nil {
				return err
			}

			outputPart = append(outputPart, output...)
			output = outputPart

			if resp.Range.Start == resp.Available.Start {
				break
			}

			if len(output) > maxPossibleBytes {
				log.G(ctx).Warnf("The maximum amount of logs has been reached. Stopping.")
				fmt.Fprintf(iostreams.G(ctx).Out, "%s\n", output)
				break
			}
		}

		fmt.Fprintf(iostreams.G(ctx).Out, "%s\n", output)
	}

	return nil
}
