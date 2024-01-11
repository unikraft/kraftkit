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

	"github.com/charmbracelet/lipgloss"
	"github.com/dustin/go-humanize"
	"kraftkit.sh/internal/tableprinter"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"
	"kraftkit.sh/tui"

	kraftcloudinstances "sdk.kraft.cloud/instances"
	kraftcloudservices "sdk.kraft.cloud/services"
	kraftcloudusers "sdk.kraft.cloud/users"
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
		table.AddField("ENV", cs.Bold)
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
			envs := []string{}
			for k, v := range instance.Env {
				envs = append(envs, fmt.Sprintf("%s=%s", k, v))
			}
			table.AddField(strings.Join(envs, ", "), nil)

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

// PrintQuotas pretty-prints the provided set of user quotas or returns
// an error if unable to send to stdout via the provided context.
func PrintQuotas(ctx context.Context, format string, quotas ...kraftcloudusers.Quotas) error {
	if err := iostreams.G(ctx).StartPager(); err != nil {
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

	if format != "table" {
		table.AddField("UUID", cs.Bold)
	}

	table.AddField("NAME", cs.Bold)
	table.AddField("LIVE INSTANCES", cs.Bold)
	table.AddField("TOTAL INSTANCES", cs.Bold)
	table.AddField("MAX TOTAL INSTANCES", cs.Bold)
	table.AddField("LIVE MEMORY", cs.Bold)
	table.AddField("SERVICE GROUPS", cs.Bold)
	table.AddField("SERVICES", cs.Bold)
	table.AddField("TOTAL VOLUME SIZE", cs.Bold)
	table.AddField("VOLUMES", cs.Bold)
	table.EndRow()

	for _, quota := range quotas {
		if format != "table" {
			table.AddField(quota.UUID, nil)
		}
		table.AddField(quota.Name, nil)
		table.AddField(fmt.Sprintf("%d/%d", quota.Used.LiveInstances, quota.Hard.LiveInstances), nil)
		table.AddField(fmt.Sprintf("%d/%d", quota.Used.Instances, quota.Hard.Instances), nil)
		table.AddField(fmt.Sprintf("%s/%s",
			humanize.IBytes(uint64(quota.Used.LiveMemoryMb)*humanize.MiByte),
			humanize.IBytes(uint64(quota.Hard.LiveMemoryMb)*humanize.MiByte),
		), nil)
		table.AddField(fmt.Sprintf("%d/%d", quota.Used.ServiceGroups, quota.Hard.ServiceGroups), nil)
		table.AddField(fmt.Sprintf("%d/%d", quota.Used.Services, quota.Hard.Services), nil)
		table.AddField(fmt.Sprintf("%s/%s",
			humanize.IBytes(uint64(quota.Used.TotalVolumeMb)*humanize.MiByte),
			humanize.IBytes(uint64(quota.Hard.TotalVolumeMb)*humanize.MiByte),
		), nil)
		table.AddField(fmt.Sprintf("%d/%d", quota.Used.Volumes, quota.Hard.Volumes), nil)
		table.EndRow()
	}

	return table.Render(iostreams.G(ctx).Out)
}

// PrettyPrintInstance outputs a single instance and information about it.
func PrettyPrintInstance(ctx context.Context, instance *kraftcloudinstances.Instance, autoStart bool) {
	out := iostreams.G(ctx).Out
	fqdn := instance.FQDN

	if len(fqdn) > 0 {
		for _, port := range instance.ServiceGroup.Services {
			if port.Port == 443 {
				fqdn = "https://" + fqdn
				break
			}
		}
	}

	var color func(...string) string
	if instance.State == "running" || instance.State == "starting" {
		color = tui.TextGreen
	} else if instance.State == "stopped" {
		color = tui.TextRed
	} else {
		color = tui.TextYellow
	}

	fmt.Fprintf(out, "\n%s%s%s %s\n", tui.TextLightGray("["), color("●"), tui.TextLightGray("]"), instance.UUID)
	fmt.Fprintf(out, "             %s: %s\n", tui.TextLightGray("name"), instance.Name)
	fmt.Fprintf(out, "            %s: %s\n", tui.TextLightGray("state"), color(instance.State))
	if len(fqdn) > 0 {
		if strings.HasPrefix(fqdn, "https://") {
			fmt.Fprintf(out, "              %s: %s\n", tui.TextLightGray("url"), fqdn)
		} else {
			fmt.Fprintf(out, "             %s: %s\n", tui.TextLightGray("fqdn"), fqdn)
		}
	}
	fmt.Fprintf(out, "            %s: %s\n", tui.TextLightGray("image"), instance.Image)
	fmt.Fprintf(out, "        %s: %.2f ms\n", tui.TextLightGray("boot time"), float64(instance.BootTimeUS)/1000)
	fmt.Fprintf(out, "           %s: %d MiB\n", tui.TextLightGray("memory"), instance.MemoryMB)
	if len(instance.ServiceGroup.Name) > 0 {
		fmt.Fprintf(out, "    %s: %s\n", tui.TextLightGray("service group"), instance.ServiceGroup.Name)
	}
	if len(instance.Args) > 0 {
		fmt.Fprintf(out, "             %s: %s\n", tui.TextLightGray("args"), strings.Join(instance.Args, " "))
	}

	fmt.Fprintf(out, "\n")

	if instance.State != "running" && instance.State != "starting" && autoStart {
		log.G(ctx).Info("it looks like the instance did not come online, to view logs run:")
		fmt.Fprintf(out, "\n    kraft cloud instance logs %s\n\n", instance.Name)
	}
}
