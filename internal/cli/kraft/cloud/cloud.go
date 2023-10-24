// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package cloud

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"kraftkit.sh/internal/cli/kraft/cloud/img"
	"kraftkit.sh/internal/cli/kraft/cloud/instance"

	"kraftkit.sh/cmdfactory"
)

type CloudOptions struct {
	Metro string `long:"metro" env:"KRAFTCLOUD_METRO" usage:"Set the KraftCloud metro."`
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&CloudOptions{}, cobra.Command{
		Short:  "KraftCloud",
		Use:    "cloud [FLAGS] [SUBCOMMAND|DIR]",
		Hidden: true,
		Long: heredoc.Docf(`
		Manage resources on KraftCloud: The Millisecond Platform.

		Learn more & sign up for the beta at https://kraft.cloud
	
		Quickly switch between metros using the %[1]s--metro%[1]s flag or use the
		%[1]sKRAFTCLOUD_METRO%[1]s environmental variable.
		
		Set authentication by using %[1]skraft login%[1]s or set
		%[1]sKRAFTCLOUD_TOKEN%[1]s environmental variable.`, "`"),
		Example: heredoc.Doc(`
		# List all images in your account
		$ kraft cloud img ls

		# List all instances in Frankfurt
		$ kraft cloud --metro fra0 instance ls

		# Create a new NGINX instance in Frankfurt and start it immediately
		$ kraft cloud --metro fra0 instance create \
			--start \
			--port 80:443 \
			unikraft.io/$KRAFTCLOUD_USER/nginx:latest -- nginx -c /usr/local/nginx/conf/nginx.conf

		# Get the status of an instance based on its UUID and output as JSON
		$ kraft cloud --metro fra0 instance status -o json UUID

		# Stop an instance based on its UUID
		$ kraft cloud instance stop UUID

		# Start an instance based on its UUID
		$ kraft cloud instance start UUID

		# Get logs of an instance based on its UUID
		$ kraft cloud instance logs UUID

		# Delete an instance based on its UUID
		$ kraft cloud instance rm UUID`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "kraftcloud",
		},
	})
	if err != nil {
		panic(err)
	}

	cmd.AddGroup(&cobra.Group{ID: "kraftcloud-img", Title: "IMAGE COMMANDS"})
	cmd.AddCommand(img.NewCmd())

	cmd.AddGroup(&cobra.Group{ID: "kraftcloud-instance", Title: "INSTANCE COMMANDS"})
	cmd.AddCommand(instance.NewCmd())

	return cmd
}

func (opts *CloudOptions) Run(cmd *cobra.Command, args []string) error {
	return nil
}
