// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package unset

import (
	"context"
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/log"
)

type Unset struct{}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&Unset{}, cobra.Command{
		Short: "Unset a KraftKit configuration option",
		Use:   "unset KEY [KEY ...]",
		Args:  cobra.MinimumNArgs(1),
		Example: heredoc.Doc(`
			# Remove a previously set toolchain variable
			$ kraft system unset toolchain.CC

			# Remove multiple keys at once
			$ kraft system unset toolchain.CC toolchain.UK_CFLAGS
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

func (opts *Unset) Run(ctx context.Context, args []string) error {
	for _, key := range args {
		log.G(ctx).
			WithField("key", key).
			Info("unsetting")

		if err := config.M[config.KraftKit](ctx).Unset(key); err != nil {
			return fmt.Errorf("could not unset configuration option: %w", err)
		}
	}

	return config.M[config.KraftKit](ctx).Write(false)
}
