// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
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

	networkapi "kraftkit.sh/api/network/v1alpha1"
	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/machine/network"
)

type InspectOptions struct {
	Driver string `noattribute:"true"`
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&InspectOptions{}, cobra.Command{
		Short:   "Inspect a machine network",
		Use:     "inspect NETWORK",
		Aliases: []string{"list"},
		Args:    cobra.ExactArgs(1),
		Long:    "Inspect a machine network.",
		Example: heredoc.Doc(`
			# Inspect a machine network
			$ kraft network inspect my-network
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

func (opts *InspectOptions) Pre(cmd *cobra.Command, _ []string) error {
	opts.Driver = cmd.Flag("driver").Value.String()
	return nil
}

func (opts *InspectOptions) Run(ctx context.Context, args []string) error {
	var err error

	strategy, ok := network.Strategies()[opts.Driver]
	if !ok {
		return fmt.Errorf("unsupported network driver strategy: %v (contributions welcome!)", opts.Driver)
	}

	controller, err := strategy.NewNetworkV1alpha1(ctx)
	if err != nil {
		return err
	}

	network, err := controller.Get(ctx, &networkapi.Network{
		ObjectMeta: v1.ObjectMeta{
			Name: args[0],
		},
	})
	if err != nil {
		return err
	}

	ret, err := json.Marshal(network)
	if err != nil {
		return err
	}

	fmt.Fprintf(iostreams.G(ctx).Out, "%s\n", ret)

	return nil
}
