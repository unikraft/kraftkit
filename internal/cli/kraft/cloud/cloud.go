// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package cloud

import (
	"context"
	"errors"

	"github.com/spf13/cobra"

	"kraftkit.sh/cmdfactory"
)

// removedMsg tells users where the cloud commands are now.
const removedMsg = "`kraft cloud` has been removed, switch to the unikraft CLI: https://unikraft.com/docs/cli"

type CloudOptions struct{}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&CloudOptions{}, cobra.Command{
		Short:   "Removed, use the unikraft CLI instead",
		Use:     "cloud",
		Aliases: []string{"cl"},
		Long:    removedMsg,
		Hidden:  true,
		// Accept all arguments and flags, so that all old calls show the message.
		Args:               cobra.ArbitraryArgs,
		DisableFlagParsing: true,
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpHidden: "true",
		},
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *CloudOptions) Run(_ context.Context, _ []string) error {
	return errors.New(removedMsg)
}
