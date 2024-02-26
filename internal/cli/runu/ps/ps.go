// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package ps

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"kraftkit.sh/cmdfactory"
	libcontainer "kraftkit.sh/libmocktainer"
	"kraftkit.sh/log"
)

const (
	flagRoot = "root"
)

const formatJSON = "json"

// PsOptions implements the runc "ps" command.
type PsOptions struct {
	Format string `long:"format" short:"f" usage:"Set output format. Options: table,yaml,json,list" default:"table"`

	rootDir string
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&PsOptions{}, cobra.Command{
		Short: "Displays the VMM process of a unikernel",
		Args:  cobra.MinimumNArgs(1),
		Use:   "ps <unikernel-id> [ps options]",
		Long:  "The ps command displays the VMM process that runs a unikernel.",
		Example: heredoc.Doc(`
			# Display the VMM process of a unikernel
			$ runu ps my-unikernel
		`),
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *PsOptions) Pre(cmd *cobra.Command, args []string) error {
	opts.rootDir = cmd.Flag(flagRoot).Value.String()
	if opts.rootDir == "" {
		return fmt.Errorf("state directory (--%s flag) is not set", flagRoot)
	}

	return nil
}

func (opts *PsOptions) Run(ctx context.Context, args []string) (retErr error) {
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

	status, err := c.Status()
	if err != nil {
		return fmt.Errorf("getting container status: %w", err)
	}

	state, err := c.State()
	if err != nil {
		return fmt.Errorf("getting container state: %w", err)
	}

	var pids []int
	if status != libcontainer.Stopped {
		pids = append(pids, state.BaseState.InitProcessPid)
	}

	if opts.Format == formatJSON {
		return json.NewEncoder(os.Stdout).Encode(pids)
	}
	return printTable(pids, args[1:])
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

func printTable(pids []int, psArgs []string) error {
	if len(psArgs) == 0 {
		psArgs = []string{"-ef"}
	}

	cmd := exec.Command("ps", psArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, output)
	}

	lines := strings.Split(string(output), "\n")
	pidIndex, err := getPidIndex(lines[0])
	if err != nil {
		return err
	}

	fmt.Println(lines[0])
	for _, line := range lines[1:] {
		if len(line) == 0 {
			continue
		}
		fields := strings.Fields(line)
		p, err := strconv.Atoi(fields[pidIndex])
		if err != nil {
			return fmt.Errorf("unable to parse pid: %w", err)
		}

		for _, pid := range pids {
			if pid == p {
				fmt.Println(line)
				break
			}
		}
	}

	return nil
}

func getPidIndex(title string) (int, error) {
	titles := strings.Fields(title)

	pidIndex := -1
	for i, name := range titles {
		if name == "PID" {
			return i, nil
		}
	}

	return pidIndex, errors.New("couldn't find PID field in ps output")
}
