// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package hyperlight

import (
	"encoding/gob"

	"kraftkit.sh/cmdfactory"
)

const (
	FlagStack       = "hyperlight-stack"
	FlagQuiet       = "hyperlight-quiet"
	FlagEnableTools = "hyperlight-enable-tools"
	FlagNetAllow    = "hyperlight-net-allow"
	FlagNetBlock    = "hyperlight-net-block"
	FlagRepeat      = "hyperlight-repeat"
	FlagExec        = "hyperlight-exec"
	FlagMount       = "hyperlight-mount"
	FlagNet         = "hyperlight-net"
)

var (
	hyperlightStack       string
	hyperlightQuiet       bool
	hyperlightEnableTools bool
	hyperlightNetAllow    []string
	hyperlightNetBlock    []string
	hyperlightRepeat      int
	hyperlightExec        string
	hyperlightMounts      []string
	hyperlightNet         bool
)

func init() {
	gob.Register(HyperlightConfig{})
}

func RegisterFlags() {
	cmdfactory.RegisterFlag(
		"kraft run",
		cmdfactory.StringVar(
			&hyperlightStack,
			FlagStack,
			"",
			"Set the Hyperlight guest stack size",
		),
	)

	cmdfactory.RegisterFlag(
		"kraft run",
		cmdfactory.BoolVar(
			&hyperlightQuiet,
			FlagQuiet,
			false,
			"Suppress hyperlight-unikraft host-side status messages",
		),
	)

	cmdfactory.RegisterFlag(
		"kraft run",
		cmdfactory.BoolVar(
			&hyperlightEnableTools,
			FlagEnableTools,
			false,
			"Enable Hyperlight tool dispatch via __dispatch host function",
		),
	)

	cmdfactory.RegisterFlag(
		"kraft run",
		cmdfactory.StringArrayVar(
			&hyperlightNetAllow,
			FlagNetAllow,
			nil,
			"Restrict Hyperlight guest networking to the listed hosts/IPs",
		),
	)

	cmdfactory.RegisterFlag(
		"kraft run",
		cmdfactory.StringArrayVar(
			&hyperlightNetBlock,
			FlagNetBlock,
			nil,
			"Block Hyperlight guest networking to the listed hosts/IPs",
		),
	)

	cmdfactory.RegisterFlag(
		"kraft run",
		cmdfactory.IntVar(
			&hyperlightRepeat,
			FlagRepeat,
			0,
			"Run the Hyperlight application N additional times",
		),
	)

	cmdfactory.RegisterFlag(
		"kraft run",
		cmdfactory.StringVar(
			&hyperlightExec,
			FlagExec,
			"",
			"Run an inline code snippet through the guest interpreter",
		),
	)

	cmdfactory.RegisterFlag(
		"kraft run",
		cmdfactory.StringArrayVar(
			&hyperlightMounts,
			FlagMount,
			nil,
			"Preopen a host directory for the Hyperlight guest filesystem",
		),
	)

	cmdfactory.RegisterFlag(
		"kraft run",
		cmdfactory.BoolVar(
			&hyperlightNet,
			FlagNet,
			false,
			"Enable guest networking for hyperlight machine. Without this flag, the guest has no network access",
		),
	)
}

func applyRegisteredRunConfig(cfg *HyperlightConfig) {
	if cfg == nil {
		return
	}

	if hyperlightStack != "" {
		cfg.Stack = hyperlightStack
	}
	if hyperlightQuiet {
		cfg.Quiet = true
	}
	if hyperlightEnableTools {
		cfg.EnableTools = true
	}
	if hyperlightNet {
		cfg.Net = true
	}
	if len(hyperlightNetAllow) > 0 {
		cfg.NetAllow = append(cfg.NetAllow, hyperlightNetAllow...)
	}
	if len(hyperlightNetBlock) > 0 {
		cfg.NetBlock = append(cfg.NetBlock, hyperlightNetBlock...)
	}
	if hyperlightRepeat != 0 {
		cfg.Repeat = hyperlightRepeat
	}
	if hyperlightExec != "" {
		cfg.Exec = hyperlightExec
	}
	if len(hyperlightMounts) > 0 {
		cfg.Mounts = append(cfg.Mounts, hyperlightMounts...)
	}
}
