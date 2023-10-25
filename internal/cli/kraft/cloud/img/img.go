// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package img

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"kraftkit.sh/internal/cli/kraft/cloud/img/list"

	"kraftkit.sh/cmdfactory"
)

type ImgOptions struct{}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&ImgOptions{}, cobra.Command{
		Short:   "Manage images on KraftCloud",
		Use:     "img",
		Aliases: []string{"images", "image"},
		Hidden:  true,
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "kraftcloud-img",
		},
	})
	if err != nil {
		panic(err)
	}

	cmd.AddCommand(list.NewCmd())

	return cmd
}

func (opts *ImgOptions) Run(_ context.Context, _ []string) error {
	return pflag.ErrHelp
}
