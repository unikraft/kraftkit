// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package utils

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/dustin/go-humanize"

	"kraftkit.sh/config"
	"kraftkit.sh/internal/fancymap"
	"kraftkit.sh/internal/tableprinter"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"
	"kraftkit.sh/tui"

	kccerts "sdk.kraft.cloud/certificates"
	kcclient "sdk.kraft.cloud/client"
	kcimages "sdk.kraft.cloud/images"
	kcinstances "sdk.kraft.cloud/instances"
	kcservices "sdk.kraft.cloud/services"
	kcautoscale "sdk.kraft.cloud/services/autoscale"
	kcusers "sdk.kraft.cloud/users"
	kcvolumes "sdk.kraft.cloud/volumes"
)

type colorFunc func(string) string

var (
	instanceStateColor = map[kcinstances.InstanceState]colorFunc{
		kcinstances.InstanceStateDraining: iostreams.Yellow,
		kcinstances.InstanceStateRunning:  iostreams.Green,
		kcinstances.InstanceStateStandby:  iostreams.Cyan,
		kcinstances.InstanceStateStarting: iostreams.Green,
		kcinstances.InstanceStateStopped:  iostreams.Red,
		kcinstances.InstanceStateStopping: iostreams.Yellow,
	}
	instanceStateColorNil = map[kcinstances.InstanceState]colorFunc{
		kcinstances.InstanceStateDraining: nil,
		kcinstances.InstanceStateRunning:  nil,
		kcinstances.InstanceStateStandby:  nil,
		kcinstances.InstanceStateStarting: nil,
		kcinstances.InstanceStateStopped:  nil,
		kcinstances.InstanceStateStopping: nil,
	}
)

var (
	certStateColor = map[string]colorFunc{
		"error":   iostreams.Red,
		"pending": iostreams.Yellow,
		"valid":   iostreams.Green,
	}
	certStateColorNil = map[string]colorFunc{
		"error":   nil,
		"pending": nil,
		"valid":   nil,
	}
)

func parseTime(dateTime, format, uuid string) (string, error) {
	if len(dateTime) > 0 {
		createdTime, err := time.Parse(time.RFC3339, dateTime)
		if err != nil {
			return "", fmt.Errorf("could not parse time for '%s': %w", uuid, err)
		}
		if format != "table" {
			return dateTime, nil
		} else {
			return humanize.Time(createdTime), nil
		}
	}

	return "", nil
}

// PrintInstances pretty-prints the provided set of instances or returns
// an error if unable to send to stdout via the provided context.
func PrintInstances(ctx context.Context, format string, resp kcclient.ServiceResponse[kcinstances.GetResponseItem]) error {
	if format == "raw" {
		printRaw(ctx, resp)
		return nil
	}

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

	// Header row
	if format != "table" {
		table.AddField("UUID", cs.Bold)
	}
	table.AddField("NAME", cs.Bold)
	table.AddField("FQDN", cs.Bold)
	if format != "table" {
		table.AddField("PRIVATE FQDN", cs.Bold)
		table.AddField("PRIVATE IP", cs.Bold)
	}
	table.AddField("STATE", cs.Bold)
	if format == "table" {
		table.AddField("STATUS", cs.Bold)
	} else {
		table.AddField("CREATED", cs.Bold)
		table.AddField("STARTED", cs.Bold)
		table.AddField("STOPPED", cs.Bold)
		table.AddField("START COUNT", cs.Bold)
		table.AddField("RESTART COUNT", cs.Bold)
		table.AddField("RESTART ATTEMPTS", cs.Bold)
		table.AddField("NEXT RESTART", cs.Bold)
		table.AddField("RESTART POLICY", cs.Bold)
		table.AddField("STOP ORIGIN", cs.Bold)
		table.AddField("STOP REASON", cs.Bold)
		table.AddField("APP EXIT CODE", cs.Bold)
	}
	table.AddField("IMAGE", cs.Bold)
	table.AddField("MEMORY", cs.Bold)
	table.AddField("ARGS", cs.Bold)
	if format != "table" {
		table.AddField("ENV", cs.Bold)
		table.AddField("VOLUMES", cs.Bold)
		table.AddField("SERVICE", cs.Bold)
		table.AddField("SNAPSHOT", cs.Bold)
	}
	table.AddField("BOOT TIME", cs.Bold)
	if format != "table" {
		table.AddField("UP TIME", cs.Bold)
	}
	table.EndRow()

	if config.G[config.KraftKit](ctx).NoColor {
		instanceStateColor = instanceStateColorNil
	}

	for _, instance := range resp.Data.Entries {
		if instance.Message != "" {
			// Header row
			if format != "table" {
				table.AddField(instance.UUID, nil)
			}
			table.AddField(instance.Name, nil)
			table.AddField("", nil) // FQDN
			if format != "table" {
				table.AddField("", nil) // PRIVATE FQDN
				table.AddField("", nil) // PRIVATE IP
			}
			table.AddField("", cs.Bold) // STATE
			if format == "table" {
				table.AddField(instance.Message, nil)
			} else {
				table.AddField("", cs.Bold) // CREATED
				table.AddField("", cs.Bold) // STARTED
				table.AddField("", cs.Bold) // STOPPED
				table.AddField("", cs.Bold) // START COUNT
				table.AddField("", cs.Bold) // RESTART COUNT
				table.AddField("", cs.Bold) // RESTART ATTEMPTS
				table.AddField("", cs.Bold) // RESTART NEXT
				table.AddField("", cs.Bold) // RESTART POLICY
				table.AddField("", cs.Bold) // STOP ORIGIN
				table.AddField("", cs.Bold) // APP EXIT CODE
			}
			table.AddField("", nil) // IMAGE
			table.AddField("", nil) // MEMORY
			table.AddField("", nil) // ARGS
			if format != "table" {
				table.AddField("", nil) // ENV
				table.AddField("", nil) // VOLUMES
				table.AddField("", nil) // SERVICE
				table.AddField("", nil) // SNAPSHOT
			}
			table.AddField("", nil) // BOOT TIME
			if format != "table" {
				table.AddField("", nil) // UP TIME
			}
			table.EndRow()

			continue
		}

		var createdAt string
		var stoppedAt string
		var startedAt string
		var restartNextAt string

		createdAt, err = parseTime(instance.CreatedAt, format, instance.UUID)
		if err != nil {
			return err
		}
		startedAt, err = parseTime(instance.StartedAt, format, instance.UUID)
		if err != nil {
			return err
		}
		stoppedAt, err = parseTime(instance.StoppedAt, format, instance.UUID)
		if err != nil {
			return err
		}
		if instance.Restart != nil {
			restartNextAt, err = parseTime(instance.Restart.NextAt, format, instance.UUID)
			if err != nil {
				return err
			}
		} else {
			restartNextAt = ""
		}

		if format != "table" {
			table.AddField(instance.UUID, nil)
		}

		table.AddField(instance.Name, nil)

		var fqdn string
		if instance.ServiceGroup != nil && len(instance.ServiceGroup.Domains) > 0 {
			fqdn = instance.ServiceGroup.Domains[0].FQDN
		}
		table.AddField(fqdn, nil)

		if format != "table" {
			table.AddField(instance.PrivateFQDN, nil)
			table.AddField(instance.PrivateIP, nil)
		}

		table.AddField(string(instance.State), instanceStateColor[instance.State])
		if format == "table" {
			table.AddField(instance.DescribeStatus(), nil)
		} else {
			table.AddField(createdAt, nil)
			table.AddField(startedAt, nil)
			table.AddField(stoppedAt, nil)
			table.AddField(fmt.Sprintf("%d", instance.StartCount), nil)
			table.AddField(fmt.Sprintf("%d", instance.RestartCount), nil)
			if instance.Restart != nil {
				table.AddField(fmt.Sprintf("%d", instance.Restart.Attempt), nil)
				table.AddField(restartNextAt, nil)
			} else {
				table.AddField("", nil)
				table.AddField("", nil)
			}
			table.AddField(string(instance.RestartPolicy), nil)
			if instance.State == kcinstances.InstanceStateStopped {
				table.AddField(fmt.Sprintf("%s (%s)", instance.DescribeStopOrigin(), instance.StopOriginCode()), nil)
				stopReason := instance.DescribeStopReason()
				switch stopReason {
				case "shutdown":
					stopReason = "Successful shutdown."
				case "assertion error":
					stopReason = "Execution failed due to an unexpected state. Check instance logs."
				case "out of memory":
					stopReason = "Out of memory. Try increasing instance's memory (see -M flag)."
				case "illegal memory access", "segmentation fault":
					stopReason = "Illegal memory access. Check instance logs."
				case "page fault":
					stopReason = "Paging error. Check instance logs."
				case "arithmetic error":
					stopReason = "Arithmetic error. Check instance logs."
				case "instruction error":
					stopReason = "Invalid CPU instruction or instruction error. Check instance logs."
				case "hardware error":
					stopReason = "Hardware reported error. Check instance logs."
				case "security violation":
					stopReason = "Security violation. Check instance logs."
				default:
					stopReason = "Unexpected error Check instance logs."
				}
				table.AddField(fmt.Sprintf("%s (%s)", stopReason, instance.StopReasonCode()), nil)
			} else {
				table.AddField("", nil)
				table.AddField("", nil)
			}
			if instance.ExitCode != nil {
				table.AddField(fmt.Sprintf("%d", *instance.ExitCode), nil)
			} else {
				table.AddField("", nil)
			}
		}
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
			if instance.ServiceGroup != nil {
				table.AddField(instance.ServiceGroup.UUID, nil)
			} else {
				table.AddField("", nil)
			}

			if instance.Snapshot != nil {
				table.AddField("present", nil)
			} else {
				table.AddField("", nil)
			}
		}

		table.AddField(fmt.Sprintf("%.2f ms", float64(instance.BootTimeUs)/1000), nil)

		if format != "table" {
			duration, err := time.ParseDuration(fmt.Sprintf("%dms", instance.UptimeMs))
			if err != nil {
				return fmt.Errorf("could not parse uptime for '%s': %w", instance.UUID, err)
			}
			table.AddField(duration.String(), nil)
		}

		table.EndRow()
	}

	return table.Render(iostreams.G(ctx).Out)
}

// PrintVolumes pretty-prints the provided set of volumes or returns
// an error if unable to send to stdout via the provided context.
func PrintVolumes(ctx context.Context, format string, resp kcclient.ServiceResponse[kcvolumes.GetResponseItem]) error {
	if format == "raw" {
		printRaw(ctx, resp)
		return nil
	}

	volumes, err := resp.AllOrErr()
	if err != nil {
		return err
	}

	if err = iostreams.G(ctx).StartPager(); err != nil {
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
	table.AddField("MOUNTED BY", cs.Bold)
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
			if format != "table" {
				createdAt = volume.CreatedAt
			} else {
				createdAt = humanize.Time(createdTime)
			}
		}

		if format != "table" {
			table.AddField(volume.UUID, nil)
		}

		table.AddField(volume.Name, nil)
		table.AddField(createdAt, nil)
		table.AddField(humanize.IBytes(uint64(volume.SizeMB)*humanize.MiByte), nil)

		var attachedTo []string
		for _, attch := range volume.AttachedTo {
			if attch.Name != "" {
				attachedTo = append(attachedTo, attch.Name)
			} else {
				attachedTo = append(attachedTo, attch.UUID)
			}
		}
		table.AddField(strings.Join(attachedTo, ","), nil)

		var mountedBy []string
		for _, mnt := range volume.MountedBy {
			mounted := ""
			if mnt.Name != "" {
				mounted = mnt.Name
			} else {
				mounted = mnt.UUID
			}
			if mnt.ReadOnly {
				mounted = mounted + ":ro"
			}
			mountedBy = append(mountedBy, mounted)
		}
		table.AddField(strings.Join(mountedBy, ","), nil)

		table.AddField(string(volume.State), nil)
		table.AddField(fmt.Sprintf("%t", volume.Persistent), nil)

		table.EndRow()
	}

	return table.Render(iostreams.G(ctx).Out)
}

// PrintAutoscaleConfiguration pretty-prints the provided autoscale configuration or returns
// an error if unable to send to stdout via the provided context.
func PrintAutoscaleConfiguration(ctx context.Context, format string, resp kcclient.ServiceResponse[kcautoscale.GetResponseItem]) error {
	if format == "raw" {
		printRaw(ctx, resp)
		return nil
	}

	aconf, err := resp.FirstOrErr()
	if err != nil {
		return err
	}

	if err = iostreams.G(ctx).StartPager(); err != nil {
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
	table.AddField("ENABLED", cs.Bold)
	var minSizeStr string
	if aconf.MinSize != nil {
		table.AddField("MIN SIZE", cs.Bold)
		minSizeStr = strconv.Itoa(*aconf.MinSize)
	}
	var maxSizeStr string
	if aconf.MaxSize != nil {
		table.AddField("MAX SIZE", cs.Bold)
		maxSizeStr = strconv.Itoa(*aconf.MaxSize)
	}
	var warmupStr string
	if aconf.WarmupTimeMs != nil {
		table.AddField("WARMUP (MS)", cs.Bold)
		warmupStr = strconv.Itoa(*aconf.WarmupTimeMs)
	}
	var cooldownStr string
	if aconf.CooldownTimeMs != nil {
		table.AddField("COOLDOWN (MS)", cs.Bold)
		cooldownStr = strconv.Itoa(*aconf.CooldownTimeMs)
	}
	var masterStr string
	if aconf.Master != nil {
		table.AddField("MASTER", cs.Bold)
		if aconf.Master.UUID != "" {
			masterStr = aconf.Master.UUID
		} else if aconf.Master.Name != "" {
			masterStr = aconf.Master.Name
		}
	}
	table.AddField("POLICIES", cs.Bold)
	table.EndRow()

	if format != "table" {
		table.AddField(aconf.UUID, nil)
	}

	table.AddField(aconf.Name, nil)
	table.AddField(fmt.Sprint(aconf.Enabled), nil)
	if aconf.MinSize != nil {
		table.AddField(fmt.Sprint(minSizeStr), nil)
	}
	if aconf.MaxSize != nil {
		table.AddField(fmt.Sprint(maxSizeStr), nil)
	}
	if aconf.WarmupTimeMs != nil {
		table.AddField(fmt.Sprint(warmupStr), nil)
	}
	if aconf.CooldownTimeMs != nil {
		table.AddField(fmt.Sprint(cooldownStr), nil)
	}
	if aconf.Master != nil {
		table.AddField(fmt.Sprint(masterStr), nil)
	}

	var policies []string
	for _, policy := range aconf.Policies {
		name := "<unknown>"
		switch policy.Type() {
		case kcautoscale.PolicyTypeStep:
			name = policy.(*kcautoscale.StepPolicy).Name
		}
		policies = append(policies, name)
	}

	table.AddField(strings.Join(policies, ";"), nil)

	table.EndRow()

	return table.Render(iostreams.G(ctx).Out)
}

// PrintServices pretty-prints the provided set of service or returns
// an error if unable to send to stdout via the provided context.
func PrintServices(ctx context.Context, format string, resp kcclient.ServiceResponse[kcservices.GetResponseItem]) error {
	if format == "raw" {
		printRaw(ctx, resp)
		return nil
	}

	services, err := resp.AllOrErr()
	if err != nil {
		return err
	}

	if err = iostreams.G(ctx).StartPager(); err != nil {
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

	for _, sg := range services {
		if format != "table" {
			table.AddField(sg.UUID, nil)
		}

		table.AddField(sg.Name, nil)

		var fqdn string
		if len(sg.Domains) > 0 {
			fqdn = sg.Domains[0].FQDN
		}
		table.AddField(fqdn, nil)

		var services []string
		for _, service := range sg.Services {
			var handlers []string
			for _, handler := range service.Handlers {
				handlers = append(handlers, string(handler))
			}

			services = append(services, fmt.Sprintf("%d:%d/%s", service.Port, service.DestinationPort, strings.Join(handlers, "+")))
		}

		table.AddField(strings.Join(services, " "), nil)

		var sgInstances []string
		for _, instance := range sg.Instances {
			if instance.Name != "" {
				sgInstances = append(sgInstances, instance.Name)
			} else {
				sgInstances = append(sgInstances, instance.UUID)
			}
		}
		table.AddField(strings.Join(sgInstances, " "), nil)

		var createdAt string
		if len(sg.CreatedAt) > 0 {
			createdTime, err := time.Parse(time.RFC3339, sg.CreatedAt)
			if err != nil {
				return fmt.Errorf("could not parse time for '%s': %w", sg.UUID, err)
			}
			if format != "table" {
				createdAt = sg.CreatedAt
			} else {
				createdAt = humanize.Time(createdTime)
			}
		}

		table.AddField(createdAt, nil)
		table.AddField(fmt.Sprintf("%v", sg.Persistent), nil)

		table.EndRow()
	}

	return table.Render(iostreams.G(ctx).Out)
}

// An internal utility method for printing a bar based on the provided progress
// and max values and the width of the bar.
func printBar(cs *iostreams.ColorScheme, progress, max int) string {
	width := 36

	var ret strings.Builder

	percent := math.Floor(float64(progress) / float64(max) * float64(width))

	color := "green"
	if percent > 30 { // ~83%
		color = "red"
	} else if percent > 20 { // ~56%
		color = "yellow"
	}

	ret.WriteString(cs.ColorFromString(color)(strings.Repeat("█", int(percent))))
	ret.WriteString(cs.ColorFromString(":243")(strings.Repeat(" ", width-int(percent))))

	return ret.String()
}

// PrintQuotas pretty-prints the provided set of user quotas or returns
// an error if unable to send to stdout via the provided context.
func PrintQuotas(ctx context.Context, auth config.AuthConfig, format string, resp kcclient.ServiceResponse[kcusers.QuotasResponseItem], imageResp *kcimages.QuotasResponseItem) error {
	if format == "raw" {
		printRaw(ctx, resp)
		return nil
	}

	quota, err := resp.FirstOrErr()
	if err != nil {
		return err
	}

	cs := iostreams.G(ctx).ColorScheme()
	table, err := tableprinter.NewTablePrinter(ctx,
		tableprinter.WithMaxWidth(iostreams.G(ctx).TerminalWidth()),
		tableprinter.WithOutputFormatFromString(format),
	)
	if err != nil {
		return err
	}

	if format != "table" {
		table.AddField("USER UUID", cs.Bold)
	}

	table.AddField("USER NAME", cs.Bold)

	// Blank line on list view
	if format == "list" {
		table.AddField("", nil)
	}

	if imageResp != nil && imageResp.Hard > 0 {
		table.AddField("IMAGE STORAGE", cs.Bold)

		// Blank line on list view
		if format == "list" {
			table.AddField("", nil)
		}
	}

	table.AddField("ACTIVE INSTANCES", cs.Bold)
	table.AddField("TOTAL INSTANCES", cs.Bold)

	// Blank line on list view
	if format == "list" {
		table.AddField("", nil)
	}

	table.AddField("ACTIVE USED MEMORY", cs.Bold)
	table.AddField("MEMORY SIZE LIMITS", cs.Bold)

	// Blank line on list view
	if format == "list" {
		table.AddField("", nil)
	}

	table.AddField("EXPOSED SERVICES", cs.Bold)
	table.AddField("SERVICES", cs.Bold)

	// Blank line on list view
	if format == "list" {
		table.AddField("", nil)
	}

	table.AddField("VOLUMES", cs.Bold)
	if quota.Limits.MaxVolumeMb > 0 {
		table.AddField("ACTIVE VOLUMES", cs.Bold)
		table.AddField("USED VOLUME SPACE", cs.Bold)
		table.AddField("VOLUME SIZE LIMITS", cs.Bold)
	}

	// Blank line on list view
	if format == "list" {
		table.AddField("", nil)
	}

	table.AddField("AUTOSCALE", cs.Bold)
	if quota.Limits.MaxAutoscaleSize > 1 {
		table.AddField("AUTOSCALE LIMIT", cs.Bold)
	}
	table.AddField("SCALE-TO-ZERO", cs.Bold)

	table.EndRow()

	if format != "table" {
		// USER UUID
		table.AddField(quota.UUID, nil)
	}

	// USER NAME
	table.AddField(strings.TrimSuffix(strings.TrimPrefix(auth.User, "robot$"), ".users.kraftcloud"), nil)

	// Blank line on list view
	if format == "list" {
		table.AddField("", nil)
	}

	// IMAGE STORAGE
	if imageResp != nil && imageResp.Hard > 0 {
		var storedImages string
		if format == "list" {
			storedImages = printBar(cs, int(imageResp.Used), int(imageResp.Hard)) + " "
		}
		storedImages += fmt.Sprintf("%s/%s",
			humanize.IBytes(uint64(imageResp.Used)*humanize.Byte),
			humanize.IBytes(uint64(imageResp.Hard)*humanize.Byte),
		)
		table.AddField(storedImages, nil)

		// Blank line on list view
		if format == "list" {
			table.AddField("", nil)
		}
	}

	// ACTIVE INSTANCES
	var activeInstances string
	if format == "list" {
		activeInstances = printBar(cs, quota.Used.LiveInstances, quota.Hard.LiveInstances) + " "
	}
	activeInstances += fmt.Sprintf("%d/%d", quota.Used.LiveInstances, quota.Hard.LiveInstances)
	table.AddField(activeInstances, nil)

	// TOTAL INSTANCES
	var totalInstances string
	if format == "list" {
		totalInstances = printBar(cs, quota.Used.Instances, quota.Hard.Instances) + " "
	}
	totalInstances += fmt.Sprintf("%d/%d", quota.Used.Instances, quota.Hard.Instances)
	table.AddField(totalInstances, nil)

	// Blank line on list view
	if format == "list" {
		table.AddField("", nil)
	}

	// ACTIVE USED MEMORY
	var activeUsedMemory string
	if format == "list" {
		activeUsedMemory = printBar(cs, quota.Used.LiveMemoryMb, quota.Hard.LiveMemoryMb) + " "
	}
	activeUsedMemory += fmt.Sprintf("%s/%s",
		humanize.IBytes(uint64(quota.Used.LiveMemoryMb)*humanize.MiByte),
		humanize.IBytes(uint64(quota.Hard.LiveMemoryMb)*humanize.MiByte),
	)
	table.AddField(activeUsedMemory, nil)

	// MEMORY SIZE LIMITS
	table.AddField(
		fmt.Sprintf("%s-%s",
			humanize.IBytes(uint64(quota.Limits.MinMemoryMb)*humanize.MiByte),
			humanize.IBytes(uint64(quota.Limits.MaxMemoryMb)*humanize.MiByte),
		), nil,
	)

	// Blank line on list view
	if format == "list" {
		table.AddField("", nil)
	}

	// EXPOSED SERVICES
	var exposedServices string
	if format == "list" {
		exposedServices = printBar(cs, quota.Used.Services, quota.Hard.Services) + " "
	}
	exposedServices += fmt.Sprintf("%d/%d", quota.Used.Services, quota.Hard.Services)
	table.AddField(exposedServices, nil)

	// SERVICES
	var services string
	if format == "list" {
		services = printBar(cs, quota.Used.ServiceGroups, quota.Hard.ServiceGroups) + " "
	}
	services += fmt.Sprintf("%d/%d", quota.Used.ServiceGroups, quota.Hard.ServiceGroups)
	table.AddField(services, nil)

	// Blank line on list view
	if format == "list" {
		table.AddField("", nil)
	}

	// VOLUMES
	if quota.Limits.MaxVolumeMb == 0 {
		table.AddField("disabled", cs.Gray)
	} else {
		table.AddField("enabled", cs.Green)

		// ACTIVE VOLUMES
		var activeVolumes string
		if format == "list" {
			activeVolumes = printBar(cs, quota.Used.Volumes, quota.Hard.Volumes) + " "
		}
		activeVolumes += fmt.Sprintf("%d/%d", quota.Used.Volumes, quota.Hard.Volumes)
		table.AddField(activeVolumes, nil)

		// USED VOLUME SPACE
		var usedVolumeSpace string
		if format == "list" {
			usedVolumeSpace = printBar(cs, quota.Used.TotalVolumeMb, quota.Hard.TotalVolumeMb) + " "
		}
		usedVolumeSpace += fmt.Sprintf("%s/%s",
			humanize.IBytes(uint64(quota.Used.TotalVolumeMb)*humanize.MiByte),
			humanize.IBytes(uint64(quota.Hard.TotalVolumeMb)*humanize.MiByte),
		)
		table.AddField(usedVolumeSpace, nil)

		// VOLUME SIZE LIMITS
		table.AddField(
			fmt.Sprintf("%s-%s",
				humanize.IBytes(uint64(quota.Limits.MinVolumeMb)*humanize.MiByte),
				humanize.IBytes(uint64(quota.Limits.MaxVolumeMb)*humanize.MiByte),
			), nil,
		)
	}

	// Blank line on list view
	if format == "list" {
		table.AddField("", nil)
	}

	// AUTOSCALE
	if quota.Limits.MaxAutoscaleSize == 1 {
		table.AddField("disabled", cs.Gray)
	} else {
		table.AddField("enabled", cs.Green)

		// AUTOSCALE LIMIT
		table.AddField(fmt.Sprintf("%d-%d",
			quota.Limits.MinAutoscaleSize,
			quota.Limits.MaxAutoscaleSize,
		), nil)
	}

	// SCALE-TO-ZERO
	table.AddField("enabled", cs.Green)

	table.EndRow()

	return table.Render(iostreams.G(ctx).Out)
}

// PrintCertificates pretty-prints the provided set of certificates or returns
// an error if unable to send to stdout via the provided context.
func PrintCertificates(ctx context.Context, format string, resp kcclient.ServiceResponse[kccerts.GetResponseItem]) error {
	if format == "raw" {
		printRaw(ctx, resp)
		return nil
	}

	certs, err := resp.AllOrErr()
	if err != nil {
		return err
	}

	if err = iostreams.G(ctx).StartPager(); err != nil {
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
	table.AddField("STATE", cs.Bold)
	if format != "table" {
		table.AddField("VALIDATION ATTEMPTS", cs.Bold)
		table.AddField("NEXT ATTEMPT", cs.Bold)
	}
	table.AddField("COMMON NAME", cs.Bold)
	if format != "table" {
		table.AddField("SUBJECT", cs.Bold)
		table.AddField("ISSUER", cs.Bold)
		table.AddField("SERIAL NUMBER", cs.Bold)
		table.AddField("NOT BEFORE", cs.Bold)
		table.AddField("NOT AFTER", cs.Bold)
	}
	table.AddField("CREATED AT", cs.Bold)
	if format != "table" {
		table.AddField("SERVICES", cs.Bold)
	}
	table.EndRow()

	if config.G[config.KraftKit](ctx).NoColor {
		certStateColor = certStateColorNil
	}

	for _, cert := range certs {
		var createdAt string

		if len(cert.CreatedAt) > 0 {
			createdTime, err := time.Parse(time.RFC3339, cert.CreatedAt)
			if err != nil {
				return fmt.Errorf("could not parse time for '%s': %w", cert.UUID, err)
			}
			if format != "table" {
				createdAt = cert.CreatedAt
			} else {
				createdAt = humanize.Time(createdTime)
			}
		}

		if format != "table" {
			table.AddField(cert.UUID, nil)
		}

		table.AddField(cert.Name, nil)
		table.AddField(string(cert.State), certStateColor[cert.State])

		if format != "table" {
			var validationAttempt string
			var validationNext string
			if cert.Validation != nil {
				validationAttempt = strconv.Itoa(cert.Validation.Attempt)
				validationNext = cert.Validation.Next
			}
			table.AddField(validationAttempt, nil)
			table.AddField(validationNext, nil)
		}

		table.AddField(cert.CommonName, nil)

		if format != "table" {
			table.AddField(cert.Subject, nil)
			table.AddField(cert.Issuer, nil)
			table.AddField(cert.SerialNumber, nil)
			table.AddField(cert.NotBefore, nil)
			table.AddField(cert.NotAfter, nil)
		}

		table.AddField(createdAt, nil)

		if format != "table" {
			sgs := make([]string, 0, len(cert.ServiceGroups))
			for _, sg := range cert.ServiceGroups {
				sgs = append(sgs, sg.Name)
			}
			table.AddField(strings.Join(sgs, ", "), nil)
		}

		table.EndRow()
	}

	return table.Render(iostreams.G(ctx).Out)
}

// PrettyPrintInstance outputs a single instance and information about it.
func PrettyPrintInstance(ctx context.Context, instance kcinstances.GetResponseItem, service *kcservices.GetResponseItem, autoStart bool) {
	out := iostreams.G(ctx).Out

	var title string
	var color func(...string) string
	if instance.State == "running" || instance.State == "starting" {
		title = "Deployed successfully!"
		color = tui.TextGreen
	} else if instance.State == "standby" {
		title = "Deployed but instance is on standby!"
		color = tui.TextLightBlue
	} else if instance.State == "stopped" {
		title = "Deployed but instance is not online!"
		color = tui.TextRed
	} else {
		title = "Deployed but unknown status"
		color = tui.TextYellow
	}

	entries := []fancymap.FancyMapEntry{
		{
			Key:   "name",
			Value: instance.Name,
		},
		{
			Key:   "uuid",
			Value: instance.UUID,
		},
		{
			Key:   "state",
			Value: color(string(instance.State)),
		},
	}

	if instance.State == "stopped" {
		entries = append(entries, fancymap.FancyMapEntry{
			Key:   "stop reason",
			Value: instance.DescribeStopReason(),
		})
	}

	if service != nil {
		for _, domain := range service.Domains {
			fqdn := domain.FQDN
			for _, port := range service.Services {
				if port.Port == 443 {
					fqdn = "https://" + fqdn
					break
				}
			}

			entries = append(entries, fancymap.FancyMapEntry{
				Key:   "domain",
				Value: fqdn,
			})
		}
	}

	entries = append(entries, fancymap.FancyMapEntry{
		Key:   "image",
		Value: instance.Image,
	})

	if instance.State != "starting" {
		entries = append(entries, fancymap.FancyMapEntry{
			Key:   "boot time",
			Value: fmt.Sprintf("%.2f ms", float64(instance.BootTimeUs)/1000),
		})
	}

	entries = append(entries, fancymap.FancyMapEntry{
		Key:   "memory",
		Value: fmt.Sprintf("%d MiB", instance.MemoryMB),
	})

	if instance.ServiceGroup != nil && len(instance.ServiceGroup.Name) > 0 {
		entries = append(entries, []fancymap.FancyMapEntry{
			{
				Key:   "service",
				Value: instance.ServiceGroup.Name,
			},
		}...)
	}

	if len(instance.PrivateFQDN) > 0 {
		entries = append(entries, []fancymap.FancyMapEntry{
			{
				Key:   "private fqdn",
				Value: instance.PrivateFQDN,
			},
		}...)
	}

	if len(instance.PrivateIP) > 0 {
		entries = append(entries, []fancymap.FancyMapEntry{
			{
				Key:   "private ip",
				Value: instance.PrivateIP,
			},
		}...)
	}

	if len(instance.Args) > 0 {
		entries = append(entries, []fancymap.FancyMapEntry{
			{
				Key:   "args",
				Value: strings.Join(instance.Args, " "),
			},
		}...)
	}

	fancymap.PrintFancyMap(
		out,
		color,
		title,
		entries...,
	)

	if instance.State != "running" && instance.State != "starting" && autoStart {
		fmt.Fprintf(out, "\n")
		log.G(ctx).Info("it looks like the instance did not come online, to view logs run:")
		fmt.Fprintf(out, "\n    kraft cloud instance logs %s\n\n", instance.Name)
	}
}

func printRaw[T kcclient.APIResponseDataEntry](ctx context.Context, resps ...kcclient.ServiceResponse[T]) {
	for _, resp := range resps {
		fmt.Fprint(iostreams.G(ctx).Out, string(resp.RawBody()))
	}
}

func IsValidOutputFormat(format string) bool {
	return format == "json" ||
		format == "table" ||
		format == "yaml" ||
		format == "list" ||
		format == "raw" ||
		format == ""
}
