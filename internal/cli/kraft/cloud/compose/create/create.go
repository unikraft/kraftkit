// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package create

import (
	"context"
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/internal/cli/kraft/cloud/compose/up"
	"kraftkit.sh/internal/cli/kraft/cloud/utils"
	"kraftkit.sh/packmanager"
)

type CreateOptions struct {
	Metro       string `noattribute:"true"`
	Composefile string `noattribute:"true"`
	Token       string `noattribute:"true"`
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&CreateOptions{}, cobra.Command{
		Short:   "Create a deployment from a Compose project on Unikraft Cloud",
		Use:     "create [FLAGS] [COMPONENT]",
		Args:    cobra.ArbitraryArgs,
		Aliases: []string{"cr"},
		Example: heredoc.Doc(`
			# Create a deployment
			$ kraft cloud compose create
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "cloud-compose",
		},
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *CreateOptions) Pre(cmd *cobra.Command, args []string) error {
	err := utils.PopulateMetroToken(cmd, &opts.Metro, &opts.Token)
	if err != nil {
		return fmt.Errorf("could not populate metro and token: %w", err)
	}

	ctx, err := packmanager.WithDefaultUmbrellaManagerInContext(cmd.Context())
	if err != nil {
		return err
	}

	cmd.SetContext(ctx)

	if cmd.Flag("file").Changed {
		opts.Composefile = cmd.Flag("file").Value.String()
	}

	return nil
}

func (opts *CreateOptions) Run(ctx context.Context, args []string) error {
	return up.Up(ctx, &up.UpOptions{
		Detach:  true,
		Metro:   opts.Metro,
		NoStart: true,
		Token:   opts.Token,
	}, args...)
}
