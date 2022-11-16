// SPDX-License-Identifier: BSD-3-Clause
//
// Authors: Alexander Jung <alex@unikraft.io>
//
// Copyright (c) 2022, Unikraft GmbH.  All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions
// are met:
//
// 1. Redistributions of source code must retain the above copyright
//    notice, this list of conditions and the following disclaimer.
// 2. Redistributions in binary form must reproduce the above copyright
//    notice, this list of conditions and the following disclaimer in the
//    documentation and/or other materials provided with the distribution.
// 3. Neither the name of the copyright holder nor the names of its
//    contributors may be used to endorse or promote products derived from
//    this software without specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
// AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
// IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
// ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE
// LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR
// CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF
// SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS
// INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN
// CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE)
// ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE
// POSSIBILITY OF SUCH DAMAGE.

package run

import (
	machinedriver "kraftkit.sh/machine/driver"
	"kraftkit.sh/unikraft/app"

	"kraftkit.sh/internal/cmdfactory"
	"kraftkit.sh/internal/cmdutil"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
)

func RunCmd(f *cmdfactory.Factory) *cobra.Command {
	cmd, err := cmdutil.NewCmd(f, "run",
		cmdutil.WithSubcmds(),
	)
	if err != nil {
		panic("could not initialize 'kraft run' command")
	}

	application := &app.CommandOptions{
		PackageManager: f.PackageManager,
		ConfigManager:  f.ConfigManager,
		Logger:         f.Logger,
		IO:             f.IOStreams,
	}

	args := &app.CommandRunArgs{}

	cmd.Short = "Run a unikernel"
	cmd.Use = "run [FLAGS] [PROJECT|KERNEL] [ARGS]"
	cmd.Aliases = []string{"launch", "r"}
	cmd.Long = heredoc.Doc(`
		Launch a unikernel`)
	cmd.Example = heredoc.Doc(`
		# Run a unikernel kernel image
		kraft run path/to/kernel-x86_64-kvm

		# Run a project which only has one target
		kraft run path/to/project
	`)
	cmd.RunE = func(cmd *cobra.Command, extraArgs []string) error {
		args.Hypervisor = cmd.Flag("hypervisor").Value.String()

		return application.Run(args, extraArgs...)
	}

	cmd.Flags().BoolVarP(
		&args.Detach,
		"detach", "d",
		false,
		"Run unikernel in background.",
	)

	cmd.Flags().BoolVar(
		&args.WithKernelDbg,
		"symbolic",
		false,
		"Use the debuggable (symbolic) unikernel.",
	)

	cmd.Flags().BoolVarP(
		&args.DisableAccel,
		"disable-acceleration", "W",
		false,
		"Disable acceleration of CPU (usually enables TCG).",
	)

	cmd.Flags().IntVarP(
		&args.Memory,
		"memory", "M",
		64,
		"Assign MB memory to the unikernel.",
	)

	cmd.Flags().StringVarP(
		&args.Target,
		"target", "t",
		"",
		"Explicitly use the defined project target.",
	)

	cmd.Flags().VarP(
		cmdutil.NewEnumFlag(machinedriver.DriverNames(), "auto"),
		"hypervisor",
		"H",
		"Set the hypervisor machine driver.",
	)

	cmd.Flags().StringVar(
		&args.Architecture,
		"arch",
		"",
		"Filter the creation of the package by architecture of known targets",
	)

	cmd.Flags().StringVar(
		&args.Platform,
		"plat",
		"",
		"Filter the creation of the package by platform of known targets",
	)

	cmd.Flags().BoolVar(
		&args.NoMonitor,
		"no-monitor",
		false,
		"Do not spawn a (or attach to an existing) KraftKit unikernel monitor",
	)

	cmd.Flags().BoolVar(
		&args.Remove,
		"rm",
		false,
		"Automatically remove the unikernel when it shutsdown",
	)

	return cmd
}
