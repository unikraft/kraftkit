// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package create

import (
	"context"
	"fmt"
	"net"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"github.com/vishvananda/netlink"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	networkapi "kraftkit.sh/api/network/v1alpha1"
	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/machine/network"
)

type CreateOptions struct {
	Driver  string `noattribute:"true"`
	Network string `long:"network" short:"n" usage:"Set the gateway IP address and the subnet of the network in CIDR format."`
}

// Create a new local machine network.
func Create(ctx context.Context, opts *CreateOptions, args ...string) error {
	if opts == nil {
		opts = &CreateOptions{}
	}

	return opts.Run(ctx, args)
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&CreateOptions{}, cobra.Command{
		Short:   "Create a new machine network",
		Use:     "create [FLAGS] NETWORK",
		Aliases: []string{"add"},
		Args:    cobra.ExactArgs(1),
		Long:    "Create a new machine network.",
		Example: heredoc.Doc(`
			# Create a new machine network
			$ kraft network create my-network --network 133.37.0.1/12
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

func (opts *CreateOptions) Pre(cmd *cobra.Command, _ []string) error {
	opts.Driver = cmd.Flag("driver").Value.String()

	return nil
}

func (opts *CreateOptions) Run(ctx context.Context, args []string) error {
	var err error

	strategy, ok := network.Strategies()[opts.Driver]
	if !ok {
		return fmt.Errorf("unsupported network driver strategy: %v (contributions welcome!)", opts.Driver)
	}

	controller, err := strategy.NewNetworkV1alpha1(ctx)
	if err != nil {
		return err
	}

	if opts.Network == "" {
		existingNetworks, err := controller.List(ctx, &networkapi.NetworkList{})
		if err != nil {
			return err
		}

		freeNetwork, err := network.FindFreeNetwork(network.DefaultNetworkPool, existingNetworks)
		if err != nil {
			return err
		}

		opts.Network = freeNetwork.String()
	}

	addr, err := netlink.ParseAddr(opts.Network)
	if err != nil {
		return err
	}

	if _, err := controller.Create(ctx, &networkapi.Network{
		ObjectMeta: metav1.ObjectMeta{
			Name: args[0],
		},
		Spec: networkapi.NetworkSpec{
			Gateway: addr.IP.String(),
			Netmask: net.IP(addr.Mask).String(),
		},
	}); err != nil {
		return err
	}

	fmt.Fprintln(iostreams.G(ctx).Out, args[0])

	return nil
}
