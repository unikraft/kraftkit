// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package create

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	kraftcloud "sdk.kraft.cloud"
	kcinstance "sdk.kraft.cloud/instance"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/cloud/utils"
	"kraftkit.sh/log"
)

type CreateOptions struct {
	Env      []string `local:"true" long:"env" short:"e" usage:"Environmental variables"`
	Memory   int64    `local:"true" long:"memory" short:"M" usage:"Specify the amount of memory to allocate"`
	Name     string   `local:"true" long:"name" short:"n" usage:"Specify the name of the package"`
	Output   string   `local:"true" long:"output" short:"o" usage:"Set output format" default:"table"`
	Port     string   `local:"true" long:"port" short:"p" usage:"Specify the port the app serves on locally"`
	Replicas int      `local:"true" long:"replicas" short:"R" usage:"Number of replicas of the instance" default:"1"`
	Start    bool     `local:"true" long:"start" short:"S" usage:"Immediately start the instance after creation"`

	metro string
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&CreateOptions{}, cobra.Command{
		Short:   "Create an instance",
		Use:     "create [FLAGS] IMAGE [-- ARGS]",
		Args:    cobra.MinimumNArgs(1),
		Aliases: []string{"new"},
		Example: heredoc.Doc(`
			# Create a hello world instance
			$ kraft cloud instance create -M 64 -p 80 unikraft.org/helloworld:latest

			# Create a new NGINX instance in Frankfurt and start it immediately
			$ kraft cloud --metro fra0 instance create \
				--start \
				--port 80:443 \
				unikraft.io/$KRAFTCLOUD_USER/nginx:latest -- nginx -c /usr/local/nginx/conf/nginx.conf
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "kraftcloud-instance",
		},
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *CreateOptions) Pre(cmd *cobra.Command, _ []string) error {
	opts.metro = cmd.Flag("metro").Value.String()
	if opts.metro == "" {
		opts.metro = os.Getenv("KRAFTCLOUD_METRO")
	}
	if opts.metro == "" {
		return fmt.Errorf("kraftcloud metro is unset")
	}
	log.G(cmd.Context()).WithField("metro", opts.metro).Debug("using")
	return nil
}

func (opts *CreateOptions) Run(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	image := args[0]
	auth, err := config.GetKraftCloudLoginFromContext(ctx)
	if err != nil {
		return fmt.Errorf("could not retrieve credentials: %w", err)
	}

	client := kcinstance.NewInstancesClient(
		kraftcloud.WithToken(auth.Token),
	)

	services := []kcinstance.CreateInstanceServicesRequest{}

	if len(opts.Port) > 0 {
		var internalPort int
		var externalPort int
		if strings.ContainsRune(opts.Port, ':') {
			ports := strings.Split(opts.Port, ":")
			if len(ports) != 2 {
				return fmt.Errorf("invalid --port value expected --port <internal>:<external>")
			}

			internalPort, err = strconv.Atoi(ports[0])
			if err != nil {
				return fmt.Errorf("invalid internal port: %w", err)
			}

			externalPort, err = strconv.Atoi(ports[1])
			if err != nil {
				return fmt.Errorf("invalid external port: %w", err)
			}
		} else {
			port, err := strconv.Atoi(opts.Port)
			if err != nil {
				return fmt.Errorf("could not parse port number: %w", err)
			}
			internalPort = port
			externalPort = port
		}

		services = append(services, kcinstance.CreateInstanceServicesRequest{
			Handlers:     []string{kcinstance.DefaultHandler},
			InternalPort: int(internalPort),
			Port:         int(externalPort),
		})
	}

	envs := make(map[string]string)
	for _, env := range opts.Env {
		if strings.ContainsRune(env, '=') {
			split := strings.SplitN(env, "=", 2)
			envs[split[0]] = split[1]
		} else {
			envs[env] = os.Getenv(env)
		}
	}

	instance, err := client.WithMetro(opts.metro).Create(ctx, kcinstance.CreateInstanceRequest{
		Image:     image,
		Args:      args[1:],
		MemoryMB:  opts.Memory,
		Services:  services,
		Autostart: opts.Start,
		Instances: opts.Replicas,
		Env:       envs,
	})
	if err != nil {
		return fmt.Errorf("could not create instance: %w", err)
	}

	return utils.PrintInstances(ctx, opts.Output, *instance)
}
