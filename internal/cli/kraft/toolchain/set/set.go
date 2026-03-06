// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package set

import (
	"context"
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
)

type SetOptions struct{}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&SetOptions{}, cobra.Command{
		Short: "Set a toolchain variable in the KraftKit configuration",
		Use:   "set KEY=VALUE [KEY=VALUE ...]",
		Args:  cobra.MinimumNArgs(1),
		Long: heredoc.Doc(`
			Set one or more toolchain variables in the global KraftKit configuration.

			Variables are passed directly to Unikraft's GNU Make build system on
			every 'kraft build' invocation. They can be overridden per-project
			using the 'toolchain' key in a Kraftfile.
		`),
		Example: heredoc.Doc(`
			# Override the C compiler
			$ kraft toolchain set CC=/usr/bin/gcc-12

			# Set multiple variables at once
			$ kraft toolchain set CC=/usr/bin/gcc-12 UK_CFLAGS="-O2 -pipe"
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "build",
		},
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *SetOptions) Run(ctx context.Context, args []string) error {
	for _, arg := range args {
		key, val, ok := strings.Cut(arg, "=")
		if !ok {
			return fmt.Errorf("invalid argument %q: expected KEY=VALUE", arg)
		}

		if key == "" {
			return fmt.Errorf("invalid argument %q: key must not be empty", arg)
		}

		if config.G[config.KraftKit](ctx).Toolchain == nil {
			config.G[config.KraftKit](ctx).Toolchain = make(map[string]string)
		}

		config.G[config.KraftKit](ctx).Toolchain[key] = val
	}

	return config.M[config.KraftKit](ctx).Write(true)
}
