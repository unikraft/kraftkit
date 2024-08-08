// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package image

import (
	"context"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"kraftkit.sh/internal/cli/kraft/cloud/image/list"
	"kraftkit.sh/internal/cli/kraft/cloud/image/remove"

	"kraftkit.sh/cmdfactory"
)

type ImageOptions struct{}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&ImageOptions{}, cobra.Command{
		Short:   "Manage images",
		Use:     "image SUBCOMMAND",
		Aliases: []string{"img, images"},
		Example: heredoc.Doc(`
			# List images in your account.
			$ kraft cloud image list

			# Delete an image from your account.
			$ kraft cloud image remove caddy@sha256:2ba5324141...
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup:  "kraftcloud-image",
			cmdfactory.AnnotationHelpHidden: "true",
		},
	})
	if err != nil {
		panic(err)
	}

	cmd.AddCommand(list.NewCmd())
	cmd.AddCommand(remove.NewCmd())

	return cmd
}

func (opts *ImageOptions) Run(_ context.Context, _ []string) error {
	return pflag.ErrHelp
}
