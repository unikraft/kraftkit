// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package utils

import (
	"context"
	"encoding/json"
	"fmt"
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
	kcinstances "sdk.kraft.cloud/instances"
	kcservices "sdk.kraft.cloud/services"
	kcautoscale "sdk.kraft.cloud/services/autoscale"
	kcusers "sdk.kraft.cloud/users"
	kcvolumes "sdk.kraft.cloud/volumes"
)

type colorFunc func(string) string

var (
	instanceStateColor = map[string]colorFunc{
		"draining": iostreams.Yellow,
		"running":  iostreams.Green,
		"standby":  iostreams.Blue,
		"starting": iostreams.Green,
		"stopped":  iostreams.Red,
		"stopping": iostreams.Yellow,
	}
	instanceStateColorNil = map[string]colorFunc{
		"draining": nil,
		"running":  nil,
		"standby":  nil,
		"starting": nil,
		"stopped":  nil,
		"stopping": nil,
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

// PrintInstances pretty-prints the provided set of instances or returns
// an error if unable to send to stdout via the provided context.
func PrintInstances(ctx context.Context, format string, instances ...kcinstances.GetResponseItem) error {
	if format == "json" {
		return printJSON(ctx, instances)
	}

	var err error

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
	if format != "table" {
		table.AddField("PRIVATE FQDN", cs.Bold)
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

	if config.G[config.KraftKit](ctx).NoColor {
		instanceStateColor = instanceStateColorNil
	}

	for _, instance := range instances {
		var createdAt string

		if len(instance.CreatedAt) > 0 {
			createdTime, err := time.Parse(time.RFC3339, instance.CreatedAt)
			if err != nil {
				return fmt.Errorf("could not parse time for '%s': %w", instance.UUID, err)
			}
			if format != "table" {
				createdAt = instance.CreatedAt
			} else {
				createdAt = humanize.Time(createdTime)
			}
		}

		if format != "table" {
			table.AddField(instance.UUID, nil)
		}

		table.AddField(instance.Name, nil)
		table.AddField(instance.FQDN, nil)

		if format != "table" {
			table.AddField(instance.PrivateFQDN, nil)
			table.AddField(instance.PrivateIP, nil)
		}

		table.AddField(string(instance.State), instanceStateColor[instance.State])
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
			if instance.ServiceGroup != nil {
				table.AddField(instance.ServiceGroup.UUID, nil)
			} else {
				table.AddField("", nil)
			}
		}

		table.AddField(fmt.Sprintf("%.2f ms", float64(instance.BootTimeUs)/1000), nil)

		table.EndRow()
	}

	return table.Render(iostreams.G(ctx).Out)
}

// PrintVolumes pretty-prints the provided set of volumes or returns
// an error if unable to send to stdout via the provided context.
func PrintVolumes(ctx context.Context, format string, volumes ...kcvolumes.GetResponseItem) error {
	if format == "json" {
		return printJSON(ctx, volumes)
	}

	var err error

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
			attachedTo = append(attachedTo, attch.Name)
		}

		table.AddField(strings.Join(attachedTo, ","), nil)
		table.AddField(string(volume.State), nil)
		table.AddField(fmt.Sprintf("%t", volume.Persistent), nil)

		table.EndRow()
	}

	return table.Render(iostreams.G(ctx).Out)
}

// PrintAutoscaleConfiguration pretty-prints the provided autoscale configuration or returns
// an error if unable to send to stdout via the provided context.
func PrintAutoscaleConfiguration(ctx context.Context, format string, aconf kcautoscale.GetResponseItem) error {
	if format == "json" {
		return printJSON(ctx, aconf)
	}

	var err error

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

// PrintServiceGroups pretty-prints the provided set of service groups or returns
// an error if unable to send to stdout via the provided context.
func PrintServiceGroups(ctx context.Context, format string, serviceGroups ...kcservices.GetResponseItem) error {
	if format == "json" {
		return printJSON(ctx, serviceGroups)
	}

	var err error

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

// PrintQuotas pretty-prints the provided set of user quotas or returns
// an error if unable to send to stdout via the provided context.
func PrintQuotas(ctx context.Context, format string, quotas ...kcusers.QuotasResponseItem) error {
	if format == "json" {
		return printJSON(ctx, quotas)
	}

	var err error

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

	if format != "table" {
		table.AddField("UUID", cs.Bold)
	}

	table.AddField("LIVE INSTANCES", cs.Bold)
	table.AddField("TOTAL INSTANCES", cs.Bold)
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

// PrintLimits pretty-prints the provided set of user limits or returns
// an error if unable to send to stdout via the provided context.
func PrintLimits(ctx context.Context, format string, quotas ...kcusers.QuotasResponseItem) error {
	if format == "json" {
		return printJSON(ctx, quotas)
	}

	var err error

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

	if format != "table" {
		table.AddField("UUID", cs.Bold)
	}

	table.AddField("MEMORY SIZE (MIN|MAX)", cs.Bold)
	table.AddField("VOLUME SIZE (MIN|MAX)", cs.Bold)
	table.AddField("AUTOSCALE SIZE (MIN|MAX)", cs.Bold)
	table.EndRow()

	for _, quota := range quotas {
		if format != "table" {
			table.AddField(quota.UUID, nil)
		}

		table.AddField(
			fmt.Sprintf("%s|%s",
				humanize.IBytes(uint64(quota.Limits.MinMemoryMb)*humanize.MiByte),
				humanize.IBytes(uint64(quota.Limits.MaxMemoryMb)*humanize.MiByte),
			), nil,
		)

		table.AddField(
			fmt.Sprintf("%s|%s",
				humanize.IBytes(uint64(quota.Limits.MinVolumeMb)*humanize.MiByte),
				humanize.IBytes(uint64(quota.Limits.MaxVolumeMb)*humanize.MiByte),
			), nil,
		)

		table.AddField(
			fmt.Sprintf("%d|%d",
				quota.Limits.MinAutoscaleSize,
				quota.Limits.MaxAutoscaleSize,
			), nil,
		)

		table.EndRow()
	}

	return table.Render(iostreams.G(ctx).Out)
}

// PrintCertificates pretty-prints the provided set of certificates or returns
// an error if unable to send to stdout via the provided context.
func PrintCertificates(ctx context.Context, format string, certs ...kccerts.GetResponseItem) error {
	if format == "json" {
		return printJSON(ctx, certs)
	}

	var err error

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
		table.AddField("SERVICE GROUPS", cs.Bold)
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
func PrettyPrintInstance(ctx context.Context, instance *kcinstances.GetResponseItem, serviceGroup *kcservices.GetResponseItem, autoStart bool) {
	out := iostreams.G(ctx).Out
	fqdn := instance.FQDN

	if serviceGroup != nil && fqdn != "" {
		for _, port := range serviceGroup.Services {
			if port.Port == 443 {
				fqdn = "https://" + fqdn
				break
			}
		}
	}

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
			Value: color(instance.State),
		},
	}

	if len(fqdn) > 0 {
		if strings.HasPrefix(fqdn, "https://") {
			entries = append(entries, fancymap.FancyMapEntry{
				Key:   "url",
				Value: fqdn,
			})
		} else {
			entries = append(entries, fancymap.FancyMapEntry{
				Key:   "fqdn",
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
				Key:   "service group",
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
		title,
		instance.State == "running" || instance.State == "starting",
		entries...,
	)

	if instance.State != "running" && instance.State != "starting" && autoStart {
		fmt.Fprintf(out, "\n")
		log.G(ctx).Info("it looks like the instance did not come online, to view logs run:")
		fmt.Fprintf(out, "\n    kraft cloud instance logs %s\n\n", instance.Name)
	}
}

func printJSON(ctx context.Context, data any) error {
	b, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("serializing data to JSON: %w", err)
	}
	fmt.Fprintln(iostreams.G(ctx).Out, string(b))
	return nil
}
