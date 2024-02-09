// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package start

import (
	"context"
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"kraftkit.sh/cmdfactory"
	libcontainer "kraftkit.sh/libmocktainer"
	"kraftkit.sh/log"
)

const (
	flagRoot = "root"
)

// StartOptions implements the OCI "start" command.
type StartOptions struct {
	rootDir string
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&StartOptions{}, cobra.Command{
		Short: "Start a unikernel",
		Args:  cobra.ExactArgs(1),
		Use:   "start <unikernel-id>",
		Long:  "The start command starts a created unikernel.",
		Example: heredoc.Doc(`
			# Start a unikernel
			$ runu start my-unikernel
		`),
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *StartOptions) Pre(cmd *cobra.Command, args []string) error {
	opts.rootDir = cmd.Flag(flagRoot).Value.String()
	if opts.rootDir == "" {
		return fmt.Errorf("state directory (--%s flag) is not set", flagRoot)
	}

	return nil
}

func (opts *StartOptions) Run(ctx context.Context, args []string) (retErr error) {
	defer func() {
		// Make sure the error is written to the configured log destination, so
		// that the message gets propagated through the caller (e.g. containerd-shim)
		if retErr != nil {
			log.G(ctx).Error(retErr)
		}
	}()

	cID := args[0]

	c, err := libcontainer.Load(opts.rootDir, cID)
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
