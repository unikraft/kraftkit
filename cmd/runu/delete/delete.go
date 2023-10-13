// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package delete

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	libcontainer "kraftkit.sh/libmocktainer"
	"kraftkit.sh/log"
)

const (
	flagRoot = "root"
)

// Delete implements the OCI "delete" command.
type Delete struct {
	Force bool `long:"force" short:"f" usage:"forcibly delete the unikernel if it is still running"`
}

func New(cfg *config.ConfigManager[config.KraftKit]) *cobra.Command {
	cmd, err := cmdfactory.New(&Delete{}, cobra.Command{
		Short: "Delete a unikernel",
		Args:  cobra.ExactArgs(1),
		Use:   "delete <unikernel-id>",
		Long:  "The delete command deletes any resources held by a unikernel.",
	}, cfg)
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *Delete) Run(cmd *cobra.Command, args []string, cfgMgr *config.ConfigManager[config.KraftKit]) (retErr error) {
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
	case libcontainer.Stopped:
		return destroyContainer(c)
	case libcontainer.Created:
		return killContainer(c)
	default:
		if opts.Force {
			return killContainer(c)
		}
		return fmt.Errorf("container is not stopped: %s", st)
	}
}

// destroyContainer destroys the given container.
func destroyContainer(c *libcontainer.Container) error {
	if err := c.Destroy(); err != nil {
		return fmt.Errorf("destroying container: %w", err)
	}
	return nil
}

// killContainer sends a SIGKILL to the container process.
func killContainer(c *libcontainer.Container) error {
	_ = c.Signal(unix.SIGKILL)
	for i := 0; i < 100; i++ {
		time.Sleep(100 * time.Millisecond)
		if err := c.Signal(unix.Signal(0)); err != nil {
			_ = destroyContainer(c)
			return nil
		}
	}
	return fmt.Errorf("container init still running")
}
