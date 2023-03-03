// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package setup

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/internal/setup"
)

type Setup struct {
	WithOS   string `long:"with-os" usage:"Force the host OS"`
	WithArch string `long:"with-arch" usage:"Force the architecture of the host"`
	WithVMM  string `long:"with-vmm" usage:"Force the use of a specific VMM on the host"`
	WithPM   string `long:"with-pm" usage:"Force the use of a specific package manager on the host"`
}

func New() *cobra.Command {
	return cmdfactory.New(&Setup{}, cobra.Command{
		Short:   "Setup the working environment for building and running unikernels",
		Use:     "setup",
		Aliases: []string{"sup"},
		Args:    cmdfactory.MaxDirArgs(0),
		Long: heredoc.Doc(`
		Setup the working environment for building and running unikernels`),
		Example: heredoc.Doc(`
			# Setup the environment
			$ kraft setup`),
		Annotations: map[string]string{
			"help:group": "build",
		},
	})
}

func (opts *Setup) Run(cmd *cobra.Command, args []string) error {
	var err error
	var sopts []setup.SetupOption

	ctx := cmd.Context()

	if opts.WithOS != "" {
		sopts = append(sopts, setup.WithOS(opts.WithOS))
	} else {
		sopts = append(sopts, setup.WithDetectHostOS())
	}

	if opts.WithArch != "" {
		sopts = append(sopts, setup.WithArch(opts.WithArch))
	} else {
		sopts = append(sopts, setup.WithDetectArch())
	}

	if opts.WithVMM != "" {
		sopts = append(sopts, setup.WithVMM(opts.WithVMM))
	} else {
		sopts = append(sopts, setup.WithDetectVMM())
	}

	if opts.WithPM != "" {
		sopts = append(sopts, setup.WithPM(opts.WithPM))
	} else {
		sopts = append(sopts, setup.WithDetectPM())
	}

	if err := setup.DoSetup(ctx, sopts...); err != nil {
		return err
	}

	return err
}
