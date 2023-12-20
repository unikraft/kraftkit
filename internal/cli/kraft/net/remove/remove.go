// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package remove

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	machineapi "kraftkit.sh/api/machine/v1alpha1"
	networkapi "kraftkit.sh/api/network/v1alpha1"
	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"
	"kraftkit.sh/machine/network"
	mplatform "kraftkit.sh/machine/platform"
)

type RemoveOptions struct {
	Driver string `noattribute:"true"`
	Force  bool   `long:"force" short:"f" usage:"Force removal of the network" default:"false"`
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
		Short:   "Remove a network",
		Use:     "rm",
		Aliases: []string{"remove", "delete", "del"},
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

func (opts *RemoveOptions) Pre(cmd *cobra.Command, _ []string) error {
	opts.Driver = cmd.Flag("driver").Value.String()
	return nil
}

func (opts *RemoveOptions) Run(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("expected exactly one network to remove, got %d", len(args))
	}

	var err error

	strategy, ok := network.Strategies()[opts.Driver]
	if !ok {
		return fmt.Errorf("unsupported network driver strategy: %v (contributions welcome!)", opts.Driver)
	}

	controller, err := strategy.NewNetworkV1alpha1(ctx)
	if err != nil {
		return err
	}

	machineController, err := mplatform.NewMachineV1alpha1ServiceIterator(ctx)
	if err != nil {
		return err
	}

	machines, err := machineController.List(ctx, &machineapi.MachineList{})
	if err != nil {
		return err
	}

	for _, machine := range machines.Items {
		if machine.Status.State != machineapi.MachineStateRunning {
			continue
		}
		for _, network := range machine.Spec.Networks {
			if network.IfName == args[0] {
				if !opts.Force {
					return fmt.Errorf("network %s is in use by machine %s. Use --force to remove it anyway", args[0], machine.Name)
				} else {
					log.G(ctx).Warnf("network '%s' is in use by machine '%s'", args[0], machine.Name)
				}
			}
		}
	}

	if opts.Force {
		// This update ensures that all the interfaces are removed from the NetworkSpec
		if _, err := controller.Update(ctx, &networkapi.Network{
			ObjectMeta: metav1.ObjectMeta{
				Name: args[0],
			},
		}); err != nil {
			return err
		}
	}

	if _, err := controller.Delete(ctx, &networkapi.Network{
		ObjectMeta: metav1.ObjectMeta{
			Name: args[0],
		},
	}); err != nil {
		return err
	}

	fmt.Fprintln(iostreams.G(ctx).Out, args[0])

	return nil
}
