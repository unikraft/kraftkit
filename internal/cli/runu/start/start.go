// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package start

import (
	"fmt"

	"github.com/spf13/cobra"

	"kraftkit.sh/cmdfactory"
	libcontainer "kraftkit.sh/libmocktainer"
	"kraftkit.sh/log"
)

const (
	flagRoot = "root"
)

// StartOptions implements the OCI "start" command.
type StartOptions struct{}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&StartOptions{}, cobra.Command{
		Short: "Start a unikernel",
		Args:  cobra.ExactArgs(1),
		Use:   "start <unikernel-id>",
		Long:  "The start command starts a created unikernel.",
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *StartOptions) Run(cmd *cobra.Command, args []string) (retErr error) {
	ctx := cmd.Context()

	defer func() {
		// Make sure the error is written to the configured log destination, so
		// that the message gets propagated through the caller (e.g. containerd-shim)
		if retErr != nil {
			log.G(ctx).Error(retErr)
		}
	}()

	rootDir := cmd.Flag(flagRoot).Value.String()
	if rootDir == "" {
		return fmt.Errorf("state directory (--%s flag) is not set", flagRoot)
	}

	cID := args[0]

	c, err := libcontainer.Load(rootDir, cID)
	if err != nil {
		return fmt.Errorf("loading container from saved state: %w", err)
	}

	st, err := c.Status()
	if err != nil {
		return fmt.Errorf("getting container status: %w", err)
	}

	switch st {
	case libcontainer.Created:
		if err := c.Exec(); err != nil {
			return fmt.Errorf("starting machine: %w", err)
		}
	case libcontainer.Running:
		return fmt.Errorf("cannot start a running machine")
	case libcontainer.Stopped:
		return fmt.Errorf("cannot start a stopped machine")
	default:
		return fmt.Errorf("cannot start a machine in the %s state", st)
	}

	return nil
}
