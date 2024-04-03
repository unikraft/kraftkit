// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package volume

import (
	"context"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"kraftkit.sh/internal/cli/kraft/cloud/volume/attach"
	"kraftkit.sh/internal/cli/kraft/cloud/volume/create"
	"kraftkit.sh/internal/cli/kraft/cloud/volume/detach"
	"kraftkit.sh/internal/cli/kraft/cloud/volume/get"
	"kraftkit.sh/internal/cli/kraft/cloud/volume/import"
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
		Long:    "Manage persistent volumes on KraftCloud.",
		Example: heredoc.Doc(`
			# List all volumes in your account.
			$ kraft cloud volume list
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup:  "kraftcloud-vol",
			cmdfactory.AnnotationHelpHidden: "true",
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
	cmd.AddCommand(vimport.NewCmd())

	return cmd
}

func (opts *VolumeOptions) Run(_ context.Context, _ []string) error {
	return pflag.ErrHelp
}
