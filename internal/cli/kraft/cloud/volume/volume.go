// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package volume

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"kraftkit.sh/internal/cli/kraft/cloud/volume/attach"
	"kraftkit.sh/internal/cli/kraft/cloud/volume/create"
	"kraftkit.sh/internal/cli/kraft/cloud/volume/detach"
	"kraftkit.sh/internal/cli/kraft/cloud/volume/get"
	"kraftkit.sh/internal/cli/kraft/cloud/volume/list"
	"kraftkit.sh/internal/cli/kraft/cloud/volume/remove"

	"kraftkit.sh/cmdfactory"
)

type VolumeOptions struct{}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&VolumeOptions{}, cobra.Command{
		Short:   "Manage persistent volumes on KraftCloud",
		Use:     "volume SUBCOMMAND",
		Aliases: []string{"volumes", "vol"},
		Hidden:  true,
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "kraftcloud-vol",
		},
	})
	if err != nil {
		panic(err)
	}

	cmd.AddCommand(attach.NewCmd())
	cmd.AddCommand(detach.NewCmd())
	cmd.AddCommand(create.NewCmd())
	cmd.AddCommand(list.NewCmd())
	cmd.AddCommand(remove.NewCmd())
	cmd.AddCommand(get.NewCmd())

	return cmd
}

func (opts *VolumeOptions) Run(_ context.Context, _ []string) error {
	return pflag.ErrHelp
}
