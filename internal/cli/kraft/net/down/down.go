// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package down

import (
	"context"
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	networkapi "kraftkit.sh/api/network/v1alpha1"
	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/machine/network"
)

type DownOptions struct {
	Driver string `noattribute:"true"`
}

// Down brings a local machine network offline.
func Down(ctx context.Context, opts *DownOptions, args ...string) error {
	if opts == nil {
		opts = &DownOptions{}
	}

	return opts.Run(ctx, args)
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&DownOptions{}, cobra.Command{
		Short:   "Bring a network offline",
		Use:     "down",
		Aliases: []string{"stop"},
		Args:    cobra.ExactArgs(1),
		Long:    "Bring a network offline.",
		Example: heredoc.Doc(`
			# Bring a network offline
			$ kraft network down my-network
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "net",
		},
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *DownOptions) Pre(cmd *cobra.Command, _ []string) error {
	opts.Driver = cmd.Flag("driver").Value.String()
	return nil
}

func (opts *DownOptions) Run(ctx context.Context, args []string) error {
	strategy, ok := network.Strategies()[opts.Driver]
	if !ok {
		return fmt.Errorf("unsupported network driver strategy: %v (contributions welcome!)", opts.Driver)
	}

	controller, err := strategy.NewNetworkV1alpha1(ctx)
	if err != nil {
		return err
	}

	network, err := controller.Stop(ctx, &networkapi.Network{
		ObjectMeta: metav1.ObjectMeta{
			Name: args[0],
		},
	})
	if err != nil {
		return err
	}

	fmt.Fprintln(iostreams.G(ctx).Out, network.Name)

	return nil
}
