// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package version

import (
	"fmt"

	"github.com/spf13/cobra"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/internal/version"
	"kraftkit.sh/iostreams"
)

type Version struct{}

func New() *cobra.Command {
	return cmdfactory.New(&Version{}, cobra.Command{
		Short:   "Show kraft version information",
		Use:     "version",
		Aliases: []string{"v"},
		Args:    cobra.NoArgs,
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "misc",
		},
	})
}

func (opts *Version) Run(cmd *cobra.Command, args []string) error {
	fmt.Fprintf(iostreams.G(cmd.Context()).Out, "kraft %s", version.String())
	return nil
}
