// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2025, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package set

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/log"
)

type Set struct{}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&Set{}, cobra.Command{
		Short: "Set a KraftKit configuration option",
		Use:   "set KEY=VALUE",
		Args:  cobra.MinimumNArgs(1),
		Example: heredoc.Doc(`
			# Change the default log level and log type
			$ kraft system set log.level=debug log.type=basic

			# Enable anonymous telemetry
			$ kraft system set collect_anonymous_telemetry=true

			# Set a toolchain variable
			$ kraft system set toolchain.CC=clang
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "misc",
		},
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *Set) Run(ctx context.Context, args []string) error {
	for _, arg := range args {
		key, val, ok := strings.Cut(arg, "=")
		if !ok {
			return errors.New("invalid argument: expected KEY=VALUE")
		}

		log.G(ctx).
			WithField(key, val).
			Info("setting")

		if err := config.M[config.KraftKit](ctx).Set(key, val); err != nil {
			return fmt.Errorf("could not set configuration option: %w", err)
		}
	}

	return config.M[config.KraftKit](ctx).Write(true)
}
