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

	networkapi "kraftkit.sh/api/network/v1alpha1"
	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/machine/network"
)

type RemoveOptions struct {
	driver string
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
	opts.driver = cmd.Flag("driver").Value.String()
	return nil
}

func (opts *RemoveOptions) Run(ctx context.Context, args []string) error {
	var err error

	strategy, ok := network.Strategies()[opts.driver]
	if !ok {
		return fmt.Errorf("unsupported network driver strategy: %v (contributions welcome!)", opts.driver)
	}

	controller, err := strategy.NewNetworkV1alpha1(ctx)
	if err != nil {
		return err
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
