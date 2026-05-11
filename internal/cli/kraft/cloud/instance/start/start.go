// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package start

import (
	"context"
	"fmt"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	kraftcloud "sdk.kraft.cloud"
	kcinstances "sdk.kraft.cloud/instances"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/cloud/utils"
	"kraftkit.sh/log"
)

type StartOptions struct {
	AllowInsecure bool                  `noattribute:"true"`
	All           bool                  `long:"all" short:"a" usage:"Start all instances"`
	Auth          *config.AuthConfig    `noattribute:"true"`
	Client        kraftcloud.KraftCloud `noattribute:"true"`
	Metro         string                `noattribute:"true"`
	Token         string                `noattribute:"true"`
	Wait          time.Duration         `local:"true" long:"wait" short:"w" usage:"Timeout to wait for the instance to start (ms/s/m/h)" default:"5m"`
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&StartOptions{}, cobra.Command{
		Short:   "Start instances",
		Use:     "start [FLAGS] [UUID|NAME [UUID|NAME]...]",
		Args:    cobra.ArbitraryArgs,
		Aliases: []string{"str"},
		Example: heredoc.Doc(`
			# Start an instance by UUID
			$ kraft cloud instance start 77d0316a-fbbe-488d-8618-5bf7a612477a

			# Start an instance by name
			$ kraft cloud instance start my-instance-431342

			# Start multiple instances
			$ kraft cloud instance start my-instance-431342 my-instance-other-2313
		`),
		Long: heredoc.Doc(`
			Start an instance on KraftCloud from a stopped instance.
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

func (opts *StartOptions) Pre(cmd *cobra.Command, _ []string) error {
	err := utils.PopulateMetroToken(cmd, &opts.Metro, &opts.Token, &opts.AllowInsecure)
	if err != nil {
		return fmt.Errorf("could not populate Metro and Token: %w", err)
	}

	return nil
}

func (opts *StartOptions) Run(ctx context.Context, args []string) error {
	return Start(ctx, opts, args...)
}

// Start KraftCloud instance(s).
func Start(ctx context.Context, opts *StartOptions, args ...string) error {
	var err error

	if opts.Auth == nil {
		opts.Auth, err = config.GetKraftCloudAuthConfig(ctx, opts.Token)
		if err != nil {
			return fmt.Errorf("could not retrieve credentials: %w", err)
		}
	}

	if opts.Client == nil {
		opts.Client = kraftcloud.NewClient(
			kraftcloud.WithAllowInsecure(opts.AllowInsecure),
			kraftcloud.WithToken(config.GetKraftCloudTokenAuthConfig(*opts.Auth)),
		)
	}

	if opts.Wait < time.Millisecond && opts.Wait != 0 {
		return fmt.Errorf("wait timeout must be greater than 1ms")
	}

	waitMs := 0
	if opts.Wait > 10*time.Second {
		waitMs = int(10 * time.Second.Milliseconds())
	} else if opts.Wait < 10*time.Second && opts.Wait != 0 {
		waitMs = int(opts.Wait.Milliseconds())
	}

	if opts.All {
		args = []string{}

		instListResp, err := opts.Client.Instances().WithMetro(opts.Metro).List(ctx)
		if err != nil {
			return fmt.Errorf("could not list instances: %w", err)
		}

		instList, err := instListResp.AllOrErr()
		if err != nil {
			return fmt.Errorf("could not list instances: %w", err)
		}
		if len(instList) == 0 {
			log.G(ctx).Info("no instances found")
			return nil
		}

		for _, instItem := range instList {
			args = append(args, instItem.UUID)
		}
	}

	log.G(ctx).Infof("starting %d instance(s)", len(args))

	resp, err := opts.Client.Instances().WithMetro(opts.Metro).Start(ctx, waitMs, args...)
	if err != nil {
		return fmt.Errorf("starting instance: %w", err)
	}
	startResponses, startErr := resp.AllOrErr()

	totalStarted := 0
	for _, started := range startResponses {
		if started.Status == "success" {
			totalStarted++
		}
	}

	log.G(ctx).Infof("started %d instance(s)", totalStarted)

	if opts.Wait < 10*time.Second {
		if startErr != nil {
			return fmt.Errorf("starting %d instance(s): %w", len(args), startErr)
		}
		return nil
	}

	deadline := time.Now().Add(opts.Wait)

	var pendingUUIDs []string
	for _, started := range startResponses {
		if started.UUID != "" && kcinstances.State(started.State) != kcinstances.StateRunning {
			pendingUUIDs = append(pendingUUIDs, started.UUID)
		}
	}

	if len(pendingUUIDs) == 0 {
		if startErr != nil {
			return fmt.Errorf("starting %d instance(s): %w", len(args), startErr)
		}
		return nil
	}

	for len(pendingUUIDs) > 0 {
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for %d instance(s) to start", len(pendingUUIDs))
		}

		_, waitErr := opts.Client.Instances().WithMetro(opts.Metro).Wait(ctx, kcinstances.StateRunning, int(min(10*time.Second, time.Until(deadline)).Milliseconds()), pendingUUIDs...)
		if waitErr == nil {
			if startErr != nil {
				return fmt.Errorf("starting %d instance(s): %w", len(args), startErr)
			}
			return nil
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for instance(s) to start: %w", waitErr)
		}

		getResp, getErr := opts.Client.Instances().WithMetro(opts.Metro).Get(ctx, pendingUUIDs...)
		if getErr == nil {
			getItems, _ := getResp.AllOrErr()
			pendingUUIDs = pendingUUIDs[:0]
			for _, item := range getItems {
				switch item.State {
				case kcinstances.InstanceStateRunning,
					kcinstances.InstanceStateStarting:
					pendingUUIDs = append(pendingUUIDs, item.UUID)
				default:
					return fmt.Errorf("instance %s transitioned to unexpected state %q while waiting to start", item.UUID, item.State)
				}
			}
		}
	}

	if startErr != nil {
		return fmt.Errorf("starting %d instance(s): %w", len(args), startErr)
	}
	return nil
}
