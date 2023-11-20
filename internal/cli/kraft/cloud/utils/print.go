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

	kraftcloudinstances "sdk.kraft.cloud/instances"
	kraftcloudservices "sdk.kraft.cloud/services"
	kraftcloudvolumes "sdk.kraft.cloud/volumes"
)

// PrintInstances pretty-prints the provided set of instances or returns
// an error if unable to send to stdout via the provided context.
func PrintInstances(ctx context.Context, format string, instances ...kraftcloudinstances.Instance) error {
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
	table.AddField("NAME", cs.Bold)
	table.AddField("FQDN", cs.Bold)
	if format != "table" {
		table.AddField("PRIVATE IP", cs.Bold)
	}
	table.AddField("STATE", cs.Bold)
	table.AddField("CREATED AT", cs.Bold)
	table.AddField("IMAGE", cs.Bold)
	table.AddField("MEMORY", cs.Bold)
	table.AddField("ARGS", cs.Bold)
	if format != "table" {
		table.AddField("VOLUMES", cs.Bold)
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

		table.AddField(instance.Name, nil)
		table.AddField(instance.FQDN, nil)

		if format != "table" {
			table.AddField(instance.PrivateIP, nil)
		}

		table.AddField(string(instance.State), nil)
		table.AddField(createdAt, nil)
		table.AddField(instance.Image, nil)
		table.AddField(humanize.IBytes(uint64(instance.MemoryMB)*humanize.MiByte), nil)
		table.AddField(strings.Join(instance.Args, " "), nil)

		if format != "table" {
			vols := make([]string, len(instance.Volumes))
			for i, vol := range instance.Volumes {
				vols[i] = fmt.Sprintf("%s:%s", vol.Name, vol.At)
				if vol.ReadOnly {
					vols[i] += ":ro"
				}
			}
			table.AddField(strings.Join(vols, ", "), nil)
			table.AddField(instance.ServiceGroup.UUID, nil)
		}

		table.AddField(fmt.Sprintf("%dus", instance.BootTimeUS), nil)

		table.EndRow()
	}

	return table.Render(iostreams.G(ctx).Out)
}

// PrintVolumes pretty-prints the provided set of volumes or returns
// an error if unable to send to stdout via the provided context.
func PrintVolumes(ctx context.Context, format string, volumes ...kraftcloudvolumes.Volume) error {
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
	table.AddField("NAME", cs.Bold)
	table.AddField("CREATED AT", cs.Bold)
	table.AddField("SIZE", cs.Bold)
	table.AddField("ATTACHED TO", cs.Bold)
	table.AddField("STATE", cs.Bold)
	table.AddField("PERSISTENT", cs.Bold)
	table.EndRow()

	for _, volume := range volumes {
		var createdAt string
		if len(volume.CreatedAt) > 0 {
			createdTime, err := time.Parse(time.RFC3339, volume.CreatedAt)
			if err != nil {
				return fmt.Errorf("could not parse time for '%s': %w", volume.UUID, err)
			}
			createdAt = humanize.Time(createdTime)
		}

		if format != "table" {
			table.AddField(volume.UUID, nil)
		}

		table.AddField(volume.Name, nil)
		table.AddField(createdAt, nil)
		table.AddField(humanize.IBytes(uint64(volume.SizeMB)*humanize.MiByte), nil)

		var attachedTo []string
		for _, instance := range volume.AttachedTo {
			attachedTo = append(attachedTo, instance.Name)
		}

		table.AddField(strings.Join(attachedTo, ","), nil)
		table.AddField(string(volume.State), nil)
		table.AddField(fmt.Sprintf("%t", volume.Persistent), nil)

		table.EndRow()
	}

	return table.Render(iostreams.G(ctx).Out)
}

// PrintServiceGroups pretty-prints the provided set of service groups or returns
// an error if unable to send to stdout via the provided context.
func PrintServiceGroups(ctx context.Context, format string, serviceGroups ...kraftcloudservices.ServiceGroup) error {
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
	table.AddField("NAME", cs.Bold)
	table.AddField("FQDN", cs.Bold)
	table.AddField("SERVICES", cs.Bold)
	table.AddField("INSTANCES", cs.Bold)
	table.AddField("CREATED AT", cs.Bold)
	table.AddField("PERSISTENT", cs.Bold)
	table.EndRow()

	for _, sg := range serviceGroups {
		if format != "table" {
			table.AddField(sg.UUID, nil)
		}

		table.AddField(sg.Name, nil)
		table.AddField(sg.FQDN, nil)

		var services []string
		for _, service := range sg.Services {
			var handlers []string
			for _, handler := range service.Handlers {
				handlers = append(handlers, string(handler))
			}

			services = append(services, fmt.Sprintf("%d:%d/%s", service.Port, service.DestinationPort, strings.Join(handlers, "+")))
		}

		table.AddField(strings.Join(services, " "), nil)
		table.AddField(strings.Join(sg.Instances, " "), nil)

		var createdAt string
		if len(sg.CreatedAt) > 0 {
			createdTime, err := time.Parse(time.RFC3339, sg.CreatedAt)
			if err != nil {
				return fmt.Errorf("could not parse time for '%s': %w", sg.UUID, err)
			}
			createdAt = humanize.Time(createdTime)
		}

		table.AddField(createdAt, nil)
		table.AddField(fmt.Sprintf("%v", sg.Persistent), nil)

		table.EndRow()
	}

	return table.Render(iostreams.G(ctx).Out)
}
