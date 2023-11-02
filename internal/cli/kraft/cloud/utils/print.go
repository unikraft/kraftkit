// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package utils

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"kraftkit.sh/internal/tableprinter"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"
	kcinstance "sdk.kraft.cloud/instance"
)

// PrintInstances pretty-prints the provided set of instances or returns
// an error if unable to send to stdout via the provided context.
func PrintInstances(ctx context.Context, format string, instances ...kcinstance.Instance) error {
	err := iostreams.G(ctx).StartPager()
	if err != nil {
		log.G(ctx).Errorf("error starting pager: %v", err)
	}

	defer iostreams.G(ctx).StopPager()

	cs := iostreams.G(ctx).ColorScheme()
	table, err := tableprinter.NewTablePrinter(ctx,
		tableprinter.WithMaxWidth(iostreams.G(ctx).TerminalWidth()),
		tableprinter.WithOutputFormatFromString(format),
	)
	if err != nil {
		return err
	}

	// Header row
	if format != "table" {
		table.AddField("UUID", cs.Bold)
	}
	table.AddField("DNS", cs.Bold)
	if format != "table" {
		table.AddField("PRIVATE IP", cs.Bold)
	}
	table.AddField("STATUS", cs.Bold)
	table.AddField("CREATED AT", cs.Bold)
	table.AddField("IMAGE", cs.Bold)
	table.AddField("MEMORY", cs.Bold)
	table.AddField("ARGS", cs.Bold)
	if format != "table" {
		table.AddField("SERVICE GROUP", cs.Bold)
	}
	table.AddField("BOOT TIME", cs.Bold)
	table.EndRow()

	for _, instance := range instances {
		var createdAt string
		if len(instance.CreatedAt) > 0 {
			createdTime, err := time.Parse(time.RFC3339, instance.CreatedAt)
			if err != nil {
				return fmt.Errorf("could not parse time for '%s': %w", instance.UUID, err)
			}
			createdAt = humanize.Time(createdTime)
		}
		if format != "table" {
			table.AddField(instance.UUID, nil)
		}
		table.AddField(instance.DNS, nil)
		if format != "table" {
			table.AddField(instance.PrivateIP, nil)
		}
		table.AddField(string(instance.Status), nil)
		table.AddField(createdAt, nil)
		table.AddField(instance.Image, nil)
		table.AddField(humanize.Bytes(uint64(instance.MemoryMB)*humanize.MiByte), nil)
		table.AddField(strings.Join(instance.Args, " "), nil)
		if format != "table" {
			table.AddField(instance.ServiceGroup, nil)
		}
		table.AddField(fmt.Sprintf("%dus", instance.BootTimeUS), nil)
		table.EndRow()
	}

	return table.Render(iostreams.G(ctx).Out)
}
