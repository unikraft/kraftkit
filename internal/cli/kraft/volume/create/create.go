// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package create

import (
	"context"
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	volumeapi "kraftkit.sh/api/volume/v1alpha1"
	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/machine/volume"
)

type CreateOptions struct {
	Driver string `noattribute:"true"`
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&CreateOptions{}, cobra.Command{
		Short: "Create a machine volume",
		Use:   "create VOLUME",
		Args:  cobra.MaximumNArgs(1),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "vol",
		},
		Example: heredoc.Doc(`
			# Create a volume with a randomly generated name
			$ kraft volume create

			# Create a volume with a specific name
			$ kraft volume create my-volume
		`),
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *CreateOptions) Pre(cmd *cobra.Command, _ []string) error {
	opts.Driver = cmd.Flag("driver").Value.String()
	return nil
}

func (opts *CreateOptions) Run(ctx context.Context, args []string) error {
	var err error

	strategy, ok := volume.Strategies()[opts.Driver]
	if !ok {
		return fmt.Errorf("unsupported network driver strategy: %v (contributions welcome!)", opts.Driver)
	}

	controller, err := strategy.NewVolumeV1alpha1(ctx)
	if err != nil {
		return err
	}

	name := ""
	if len(args) > 0 {
		name = args[0]
	}

	vol, err := controller.Get(ctx, &volumeapi.Volume{
		ObjectMeta: v1.ObjectMeta{
			Name: name,
		},
	})
	if err != nil {
		return err
	}

	if vol != nil {
		return fmt.Errorf("volume %s already exists", args[0])
	}

	if vol, err = controller.Create(ctx, &volumeapi.Volume{
		ObjectMeta: v1.ObjectMeta{
			Name: args[0],
		},
		Spec: volumeapi.VolumeSpec{
			Driver: opts.Driver,
		},
	}); err != nil {
		return err
	}

	fmt.Fprintln(iostreams.G(ctx).Out, vol.Name)
	return nil
}
