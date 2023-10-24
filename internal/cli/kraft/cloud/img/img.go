// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package img

import (
	"github.com/spf13/cobra"

	"kraftkit.sh/internal/cli/kraft/cloud/img/list"

	"kraftkit.sh/cmdfactory"
)

type Img struct{}

func New() *cobra.Command {
	cmd, err := cmdfactory.New(&Img{}, cobra.Command{
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

	cmd.AddCommand(list.New())

	return cmd
}

func (opts *Img) Run(cmd *cobra.Command, args []string) error {
	return cmd.Help()
}
