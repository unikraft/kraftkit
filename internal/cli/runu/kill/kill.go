// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package kill

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"

	"kraftkit.sh/cmdfactory"
	libcontainer "kraftkit.sh/libmocktainer"
	"kraftkit.sh/log"
)

const (
	flagRoot = "root"
)

// KillOptions implements the OCI "kill" command.
type KillOptions struct {
	// This flag is being deprecated (opencontainers/runc#3864) but needs to be
	// retained for backwards compatibility with containerd's CRI implementation
	// (Kubernetes).
	All bool `long:"all" short:"a" usage:"send the specified signal to all processes"`

	rootDir string
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&KillOptions{}, cobra.Command{
		Short: "Send a signal to a unikernel",
		Args:  cobra.RangeArgs(1, 2),
		Use:   "kill <unikernel-id> [signal]",
		Long:  "The kill command sends a signal to a unikernel.  If the signal is not specified, SIGTERM is sent.",
		Example: heredoc.Doc(`
			# Send a SIGTERM to a unikernel
			$ runu kill my-unikernel
		`),
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *KillOptions) Pre(cmd *cobra.Command, args []string) error {
	opts.rootDir = cmd.Flag(flagRoot).Value.String()
	if opts.rootDir == "" {
		return fmt.Errorf("state directory (--%s flag) is not set", flagRoot)
	}

	return nil
}

func (opts *KillOptions) Run(ctx context.Context, args []string) (retErr error) {
	defer func() {
		// Make sure the error is written to the configured log destination, so
		// that the message gets propagated through the caller (e.g. containerd-shim)
		if retErr != nil {
			log.G(ctx).Error(retErr)
		}
	}()

	cID := args[0]

	sig := unix.SIGTERM
	if len(args) == 2 {
		var err error
		if sig, err = parseSignal(args[1]); err != nil {
			return err
		}
	}

	c, err := libcontainer.Load(opts.rootDir, cID)
	if err != nil {
		return fmt.Errorf("loading container from saved state: %w", err)
	}

	err = c.Signal(sig)
	switch {
	case errors.Is(err, libcontainer.ErrNotRunning) && opts.All:
		// no op
	case err != nil:
		return fmt.Errorf("signaling machine process: %w", err)
	}

	return nil
}

/*
Copyright 2014 Docker, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
func parseSignal(rawSignal string) (unix.Signal, error) {
	s, err := strconv.Atoi(rawSignal)
	if err == nil {
		return unix.Signal(s), nil
	}
	sig := strings.ToUpper(rawSignal)
	if !strings.HasPrefix(sig, "SIG") {
		sig = "SIG" + sig
	}
	signal := unix.SignalNum(sig)
	if signal == 0 {
		return -1, fmt.Errorf("unknown signal %q", rawSignal)
	}
	return signal, nil
}
