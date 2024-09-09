// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package metrics

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"

	kraftcloud "sdk.kraft.cloud"
	kcinstances "sdk.kraft.cloud/instances"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/cloud/utils"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"
	"kraftkit.sh/tui/pstable"
)

type MetricsOptions struct {
	Auth   *config.AuthConfig    `noattribute:"true"`
	Client kraftcloud.KraftCloud `noattribute:"true"`
	Metro  string                `noattribute:"true"`
	Token  string                `noattribute:"true"`
	Output string                `long:"output" short:"o" usage:"Set output format. Options: top,table,yaml,json,list"`
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&MetricsOptions{}, cobra.Command{
		Short:   "Show instance metrics",
		Use:     "top [FLAGS] [UUID|NAME [UUID|NAME]...]",
		Aliases: []string{"metrics", "metric", "m", "meter"},
		Args:    cobra.ArbitraryArgs,
		Example: heredoc.Doc(`
			# Return metrics for all instances
			$ kraft cloud instance top

			# Return metrics for an instance by UUID
			$ kraft cloud instance top fd1684ea-7970-4994-92d6-61dcc7905f2b

			# Return metrics for an instance by name
			$ kraft cloud instance top my-instance-431342
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

func (opts *MetricsOptions) printTop(ctx context.Context) error {
	instanceStateColor := utils.InstanceStateColor
	if config.G[config.KraftKit](ctx).NoColor {
		instanceStateColor = utils.InstanceStateColorNil
	}

	previousMetrics := make(map[string]kcinstances.MetricsResponseItem)
	table, err := pstable.NewPSTable(ctx,
		"kraft cloud instance top",
		[]pstable.Column{
			{Title: "NAME"},
			{Title: "STATE"},
			{Title: "UPTIME"},
			{Title: "CPU"},
			{Title: "MEM"},
			{Title: "NET RX / TX"},
		},
		func(ctx context.Context) ([]pstable.Row, error) {
			instListResp, err := opts.Client.Instances().WithMetro(opts.Metro).List(ctx)
			if err != nil {
				return nil, fmt.Errorf("could not list instances: %w", err)
			}

			instList, err := instListResp.AllOrErr()
			if err != nil {
				return nil, fmt.Errorf("could not list instances: %w", err)
			}

			if len(instList) == 0 {
				return nil, nil
			}

			var uuids []string
			for _, instItem := range instList {
				uuids = append(uuids, instItem.UUID)
			}

			metricsReps, err := opts.Client.Instances().WithMetro(opts.Metro).Metrics(ctx, uuids...)
			if err != nil {
				return nil, fmt.Errorf("getting metrics of %d instance(s): %w", len(instList), err)
			}

			metrics, err := metricsReps.AllOrErr()
			if err != nil {
				return nil, fmt.Errorf("getting metrics of %d instance(s): %w", len(instList), err)
			}

			instanceResps, err := opts.Client.Instances().WithMetro(opts.Metro).Get(ctx, uuids...)
			if err != nil {
				return nil, fmt.Errorf("getting details of %d instance(s): %w", len(instList), err)
			}

			instances, err := instanceResps.AllOrErr()
			if err != nil {
				return nil, fmt.Errorf("getting details of %d instance(s): %w", len(instList), err)
			}

			rows := make(map[string]pstable.Row)
			instancesMap := make(map[string]kcinstances.GetResponseItem)

			for _, instance := range instances {
				instancesMap[instance.UUID] = instance
				if _, ok := previousMetrics[instance.UUID]; !ok {
					previousMetrics[instance.UUID] = kcinstances.MetricsResponseItem{}
				}
				rows[instance.UUID] = make([]pstable.Cell, 6)
				rows[instance.UUID][0] = pstable.StringCell(instance.Name)
				rows[instance.UUID][1] = pstable.StringCell(instanceStateColor[instance.State](string(instance.State)))
				duration, err := time.ParseDuration(fmt.Sprintf("%dms", instance.UptimeMs))
				if err != nil {
					return nil, fmt.Errorf("could not parse uptime for '%s': %w", instance.UUID, err)
				}
				timeParsed := duration.Seconds()
				timeUnit := "s"
				if timeParsed > 60 {
					timeParsed = duration.Minutes()
					timeUnit = "m"
				}
				if timeParsed > 60 {
					timeParsed = duration.Hours()
					timeUnit = "h"
				}
				rows[instance.UUID][2] = pstable.StringCell(iostreams.Bold(fmt.Sprintf("%.2f%s", timeParsed, timeUnit)))
			}

			for _, metric := range metrics {
				if _, ok := rows[metric.UUID]; !ok {
					continue
				}

				rows[metric.UUID][3] = pstable.GuageCell{
					Cs:      iostreams.G(ctx).ColorScheme(),
					Current: float64(metric.CPUTimeMs - previousMetrics[metric.UUID].CPUTimeMs),
					Max:     1000,
					Width:   10,
				}

				rows[metric.UUID][4] = pstable.GuageCell{
					Cs:      iostreams.G(ctx).ColorScheme(),
					Current: float64(metric.RSS),
					Max:     float64(instancesMap[metric.UUID].MemoryMB) * 1024 * 1024,
					Width:   10,
				}
				rows[metric.UUID][5] = pstable.StringCell(iostreams.Bold(fmt.Sprintf("%s / %s", humanize.Bytes(metric.RxBytes), humanize.Bytes(metric.TxBytes))))

				previousMetrics[metric.UUID] = metric
			}

			var keys []string
			for key := range rows {
				keys = append(keys, key)
			}

			// Sort ret by keys
			sort.Strings(keys)
			var ret []pstable.Row
			for _, key := range keys {
				ret = append(ret, rows[key])
			}

			return ret, nil
		})
	if err != nil {
		return fmt.Errorf("could not create table: %w", err)
	}

	return table.Start(ctx)
}

func (opts *MetricsOptions) Pre(cmd *cobra.Command, _ []string) error {
	err := utils.PopulateMetroToken(cmd, &opts.Metro, &opts.Token)
	if err != nil {
		return fmt.Errorf("could not populate metro and token: %w", err)
	}

	if !utils.IsValidOutputFormat(opts.Output) {
		return fmt.Errorf("invalid output format: %s", opts.Output)
	}

	return nil
}

func (opts *MetricsOptions) Run(ctx context.Context, args []string) error {
	auth, err := config.GetKraftCloudAuthConfig(ctx, opts.Token)
	if err != nil {
		return fmt.Errorf("could not retrieve credentials: %w", err)
	}

	opts.Client = kraftcloud.NewClient(
		kraftcloud.WithToken(config.GetKraftCloudTokenAuthConfig(*auth)),
	)

	instances := args
	if len(instances) == 0 {
		resp, err := opts.Client.Instances().WithMetro(opts.Metro).List(ctx)
		if err != nil {
			return fmt.Errorf("could not list instances: %w", err)
		}

		insts, err := resp.AllOrErr()
		if err != nil {
			return fmt.Errorf("could not list instances: %w", err)
		}

		for _, inst := range insts {
			instances = append(instances, inst.UUID)
		}
	}

	if len(instances) > 0 {
		if opts.Output == "" {
			opts.Output = "list"
			if log.LoggerTypeFromString(config.G[config.KraftKit](ctx).Log.Type) == log.FANCY {
				opts.Output = "top"
			} else if len(instances) > 1 {
				opts.Output = "table"
			}
		}

		if opts.Output == "top" {
			return opts.printTop(ctx)
		}

		resp, err := opts.Client.Instances().WithMetro(opts.Metro).Metrics(ctx, instances...)
		if err != nil {
			return fmt.Errorf("could not get instance %s: %w", instances, err)
		}
		return utils.PrintMetrics(ctx, opts.Output, *resp)
	}

	return fmt.Errorf("no instances found")
}
