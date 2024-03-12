// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package cloud

import (
	"context"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"kraftkit.sh/internal/cli/kraft/cloud/certificate"
	"kraftkit.sh/internal/cli/kraft/cloud/deploy"
	"kraftkit.sh/internal/cli/kraft/cloud/img"
	"kraftkit.sh/internal/cli/kraft/cloud/instance"
	"kraftkit.sh/internal/cli/kraft/cloud/metros"
	"kraftkit.sh/internal/cli/kraft/cloud/quotas"
	"kraftkit.sh/internal/cli/kraft/cloud/scale"
	"kraftkit.sh/internal/cli/kraft/cloud/service"
	"kraftkit.sh/internal/cli/kraft/cloud/volume"

	"kraftkit.sh/cmdfactory"
)

type CloudOptions struct {
	Metro string `long:"metro" env:"KRAFTCLOUD_METRO" usage:"Set the KraftCloud metro"`
	Token string `long:"token" env:"KRAFTCLOUD_TOKEN" usage:"Set the KraftCloud token"`
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&CloudOptions{}, cobra.Command{
		Short:   "Manage resources on the KraftCloud platform",
		Use:     "cloud [FLAGS] [SUBCOMMAND|DIR]",
		Aliases: []string{"cl"},
		Long: heredoc.Docf(`
			▄▀▄▀ kraft.cloud

			Manage resources on The Millisecond Platform.

			Learn more & sign up for the beta at https://kraft.cloud

			Quickly switch between metros using the %[1]s--metro%[1]s flag or use the
			%[1]sKRAFTCLOUD_METRO%[1]s environmental variable.

			Set authentication by using %[1]skraft login%[1]s or set
			%[1]sKRAFTCLOUD_TOKEN%[1]s environmental variable.
		`, "`"),
		Example: heredoc.Doc(`
			# List all images in your account
			$ kraft cloud image list

			# List all instances in Frankfurt
			$ kraft cloud --metro fra0 instance list

			# Create a new NGINX instance in Frankfurt and start it immediately
			$ kraft cloud instance create -S \
				-p 80:443/http+redirect \
				-p 443:8080/http+tls \
				nginx:latest

			# Get the status of an instance based on its UUID and output as JSON
			$ kraft cloud --metro fra0 instance status -o json UUID

			# Stop an instance based on its UUID
			$ kraft cloud instance stop UUID

			# Start an instance based on its UUID
			$ kraft cloud instance start UUID

			# Get logs of an instance based on its UUID
			$ kraft cloud instance logs UUID

			# Delete an instance based on its UUID
			$ kraft cloud instance remove UUID
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup:  "kraftcloud",
			cmdfactory.AnnotationHelpHidden: "true",
		},
	})
	if err != nil {
		panic(err)
	}

	cmd.AddCommand(deploy.NewCmd())
	cmd.AddCommand(quotas.NewCmd())

	cmd.AddGroup(&cobra.Group{ID: "kraftcloud-img", Title: "IMAGE COMMANDS"})
	cmd.AddCommand(img.NewCmd())

	cmd.AddGroup(&cobra.Group{ID: "kraftcloud-instance", Title: "INSTANCE COMMANDS"})
	cmd.AddCommand(instance.NewCmd())

	cmd.AddGroup(&cobra.Group{ID: "kraftcloud-vol", Title: "VOLUME COMMANDS"})
	cmd.AddCommand(volume.NewCmd())

	cmd.AddGroup(&cobra.Group{ID: "kraftcloud-scale", Title: "SCALE COMMANDS"})
	cmd.AddCommand(scale.NewCmd())

	cmd.AddGroup(&cobra.Group{ID: "kraftcloud-svc", Title: "SERVICE COMMANDS"})
	cmd.AddCommand(service.NewCmd())

	cmd.AddGroup(&cobra.Group{ID: "kraftcloud-certificate", Title: "CERTIFICATE COMMANDS"})
	cmd.AddCommand(certificate.NewCmd())

	cmd.AddGroup(&cobra.Group{ID: "kraftcloud-metro", Title: "METRO COMMANDS"})
	cmd.AddCommand(metros.NewCmd())

	return cmd
}

func (opts *CloudOptions) Run(_ context.Context, args []string) error {
	return pflag.ErrHelp
}
