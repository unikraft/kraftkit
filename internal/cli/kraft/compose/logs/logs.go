// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package logs

import (
	"context"
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	machineapi "kraftkit.sh/api/machine/v1alpha1"

	"github.com/spf13/cobra"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/compose"
	kernellogs "kraftkit.sh/internal/cli/kraft/logs"
	"kraftkit.sh/log"

	mplatform "kraftkit.sh/machine/platform"

	"kraftkit.sh/packmanager"
)

type LogsOptions struct {
	Follow bool `long:"follow" usage:"Follow log output"`

	Composefile string `noattribute:"true"`
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&LogsOptions{}, cobra.Command{
		Short: "Print the logs of services in a project",
		Use:   "logs",
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "compose",
		},
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *LogsOptions) Pre(cmd *cobra.Command, _ []string) error {
	ctx, err := packmanager.WithDefaultUmbrellaManagerInContext(cmd.Context())
	if err != nil {
		return err
	}

	cmd.SetContext(ctx)

	if cmd.Flag("file").Changed {
		opts.Composefile = cmd.Flag("file").Value.String()
	}

	log.G(cmd.Context()).WithField("composefile", opts.Composefile).Debug("using")

	return nil
}

func (opts *LogsOptions) Run(ctx context.Context, args []string) error {
	workdir, err := os.Getwd()
	if err != nil {
		return err
	}

	project, err := compose.NewProjectFromComposeFile(ctx, workdir, opts.Composefile)
	if err != nil {
		return err
	}

	if err := project.Validate(ctx); err != nil {
		return err
	}

	controller, err := mplatform.NewMachineV1alpha1ServiceIterator(ctx)
	if err != nil {
		return err
	}

	services, err := project.GetServices(args...)
	if err != nil {
		return err
	}

	machinesToLog := []string{}
	for _, service := range services {
		if len(args) == 0 && service.Attach != nil && !*service.Attach {
			continue
		}
		machine, _ := controller.Get(ctx, &machineapi.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Name: service.Name,
			},
		})
		if machine != nil {
			machinesToLog = append(machinesToLog, machine.Name)
		}
	}

	logOptions := kernellogs.LogOptions{
		Follow:   opts.Follow,
		Platform: "auto",
	}

	return logOptions.Run(ctx, machinesToLog)
}
