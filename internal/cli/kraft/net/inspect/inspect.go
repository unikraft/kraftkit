// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package inspect

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	networkapi "kraftkit.sh/api/network/v1alpha1"
	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/machine/network"
)

type Inspect struct {
	Driver string `noattribute:"true"`
}

func New() *cobra.Command {
	cmd, err := cmdfactory.New(&Inspect{}, cobra.Command{
		Short:   "Inspect a machine network",
		Use:     "inspect NETWORK",
		Aliases: []string{"list"},
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

func (opts *Inspect) Pre(cmd *cobra.Command, _ []string) error {
	opts.Driver = cmd.Flag("driver").Value.String()
	return nil
}

func (opts *Inspect) Run(cmd *cobra.Command, args []string) error {
	var err error

	ctx := cmd.Context()

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
