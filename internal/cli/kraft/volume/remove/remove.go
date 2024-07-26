// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package remove

import (
	"context"
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	volumeapi "kraftkit.sh/api/volume/v1alpha1"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/machine/volume"
)

type RemoveOptions struct {
	Driver string `noattribute:"true"`
}

// Remove a local machine network.
func Remove(ctx context.Context, opts *RemoveOptions, args ...string) error {
	if opts == nil {
		opts = &RemoveOptions{}
	}

	return opts.Run(ctx, args)
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&RemoveOptions{}, cobra.Command{
		Short:   "Remove a volume",
		Use:     "remove",
		Aliases: []string{"rm", "delete", "del"},
		Args:    cobra.ExactArgs(1),
		Long:    "Remove a volume.",
		Example: heredoc.Doc(`
			# Remove a volume 
			$ kraft volume remove my-volume
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "vol",
		},
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *RemoveOptions) Pre(cmd *cobra.Command, _ []string) error {
	opts.Driver = cmd.Flag("driver").Value.String()
	return nil
}

func (opts *RemoveOptions) Run(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("expected exactly one volume to remove, got %d", len(args))
	}

	var err error

	strategy, ok := volume.Strategies()[opts.Driver]
	if !ok {
		return fmt.Errorf("unsupported network driver strategy: %v (contributions welcome!)", opts.Driver)
	}

	controller, err := strategy.NewVolumeV1alpha1(ctx)
	if err != nil {
		return err
	}

	volumes, err := controller.List(ctx, &volumeapi.VolumeList{})
	if err != nil {
		return err
	}

	for _, v := range volumes.Items {
		if v.Name == args[0] || string(v.UID) == args[0] {
			_, err = controller.Delete(ctx, &v)
			if err != nil {
				return err
			}
		}
	}

	fmt.Fprintln(iostreams.G(ctx).Out, args[0])

	return nil
}
