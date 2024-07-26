// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package inspect

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	volumeapi "kraftkit.sh/api/volume/v1alpha1"
	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/machine/volume"
)

type Inspect struct {
	Driver string `noattribute:"true"`
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&Inspect{}, cobra.Command{
		Short:   "Inspect a machine volume",
		Use:     "inspect VOLUME",
		Aliases: []string{"get"},
		Args:    cobra.ExactArgs(1),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "vol",
		},
		Example: heredoc.Doc(`
			# Inspect a volume
			$ kraft volume inspect my-volume
		`),
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *Inspect) Pre(cmd *cobra.Command, _ []string) error {
	opts.Driver = cmd.Flag("driver").Value.String()
	return nil
}

func (opts *Inspect) Run(ctx context.Context, args []string) error {
	var err error

	strategy, ok := volume.Strategies()[opts.Driver]
	if !ok {
		return fmt.Errorf("unsupported network driver strategy: %v (contributions welcome!)", opts.Driver)
	}

	controller, err := strategy.NewVolumeV1alpha1(ctx)
	if err != nil {
		return err
	}

	volume, err := controller.Get(ctx, &volumeapi.Volume{
		ObjectMeta: v1.ObjectMeta{
			Name: args[0],
		},
	})
	if err != nil {
		return err
	}

	ret, err := json.Marshal(volume)
	if err != nil {
		return err
	}

	fmt.Fprintf(iostreams.G(ctx).Out, "%s\n", ret)

	return nil
}
