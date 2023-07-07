// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package create

import (
	"fmt"
	"net"

	"github.com/juju/errors"
	"github.com/spf13/cobra"
	"github.com/vishvananda/netlink"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	networkapi "kraftkit.sh/api/network/v1alpha1"
	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/machine/network"
)

type Create struct {
	driver  string
	Network string `long:"network" short:"n" usage:"Set the gateway IP address and the subnet of the network in CIDR format."`
}

func New() *cobra.Command {
	cmd, err := cmdfactory.New(&Create{}, cobra.Command{
		Short:   "Create a new machine network",
		Use:     "create [FLAGS] NETWORK",
		Aliases: []string{"add"},
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

func (opts *Create) Pre(cmd *cobra.Command, _ []string) error {
	opts.driver = cmd.Flag("driver").Value.String()

	// TODO(nderjung): A future implementation can list existing networks and
	// generate new subnet and gateway appropriately.  Simply calculate a new
	// subnet which is out of bounds from all existing subnets for the given
	// driver.  Additionally, a gateway can be determined by selecting the first
	// allocatable IP of a given subnet, i.e. 0.0.0.1.  The subnet and gateway
	// can therefore be expressed in the form X.Y.Z.1/N.
	// if opts.Subnet == "" {
	// 	return fmt.Errorf("cannot create network without subnet")
	// }
	if opts.Network == "" {
		return errors.New("cannot create network without gateway and subnet in CIDR format")
	}

	return nil
}

func (opts *Create) Run(cmd *cobra.Command, args []string) error {
	var err error

	ctx := cmd.Context()

	strategy, ok := network.Strategies()[opts.driver]
	if !ok {
		return errors.Errorf("unsupported network driver strategy: %v (contributions welcome!)", opts.driver)
	}

	controller, err := strategy.NewNetworkV1alpha1(ctx)
	if err != nil {
		return err
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
