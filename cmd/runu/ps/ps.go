// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package ps

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	libcontainer "kraftkit.sh/libmocktainer"
	"kraftkit.sh/log"
)

const (
	flagRoot = "root"
)

const formatJSON = "json"

// Ps implements the runc "ps" command.
type Ps struct {
	Format string `long:"format" short:"f" usage:"format of the output (table or json)" default:"table"`
}

func New(cfg *config.ConfigManager[config.KraftKit]) *cobra.Command {
	cmd, err := cmdfactory.New(&Ps{}, cobra.Command{
		Short: "Displays the VMM process of a unikernel",
		Args:  cobra.MinimumNArgs(1),
		Use:   "ps <unikernel-id> [ps options]",
		Long:  "The ps command displays the VMM process that runs a unikernel.",
	}, cfg)
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *Ps) Run(cmd *cobra.Command, args []string, cfgMgr *config.ConfigManager[config.KraftKit]) (retErr error) {
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
