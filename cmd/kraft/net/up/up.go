// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package up

import (
	"fmt"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	networkapi "kraftkit.sh/api/network/v1alpha1"
	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/machine/network"
)

type Up struct {
	driver string
}

func New() *cobra.Command {
	cmd, err := cmdfactory.New(&Up{}, cobra.Command{
		Short:   "Bring a network online",
		Use:     "up",
		Aliases: []string{"start"},
		Args:    cobra.ExactArgs(1),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "net",
		},
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *Up) Pre(cmd *cobra.Command, _ []string) error {
	opts.driver = cmd.Flag("driver").Value.String()
	return nil
}

func (opts *Up) Run(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	strategy, ok := network.Strategies()[opts.driver]
	if !ok {
		return fmt.Errorf("unsupported network driver strategy: %v (contributions welcome!)", opts.driver)
	}

	controller, err := strategy.NewNetworkV1alpha1(ctx)
	if err != nil {
		return err
	}

	network, err := controller.Start(ctx, &networkapi.Network{
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
