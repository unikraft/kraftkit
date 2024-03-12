// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package certificate

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"kraftkit.sh/cmdfactory"

	"kraftkit.sh/internal/cli/kraft/cloud/certificate/get"
	"kraftkit.sh/internal/cli/kraft/cloud/certificate/list"
	"kraftkit.sh/internal/cli/kraft/cloud/certificate/remove"
)

type CertificateOptions struct{}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&CertificateOptions{}, cobra.Command{
		Short:   "Manage KraftCloud TLS certificates",
		Use:     "certificate SUBCOMMAND",
		Aliases: []string{"certificates", "cert", "certs", "crt", "crts"},
		Long:    "Manage KraftCloud TLS certificates.",
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup:  "kraftcloud-certificate",
			cmdfactory.AnnotationHelpHidden: "true",
		},
	})
	if err != nil {
		panic(err)
	}

	cmd.AddCommand(list.NewCmd())
	cmd.AddCommand(remove.NewCmd())
	cmd.AddCommand(get.NewCmd())

	return cmd
}

func (opts *CertificateOptions) Run(_ context.Context, _ []string) error {
	return pflag.ErrHelp
}
