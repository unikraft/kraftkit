// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package qemu

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/Masterminds/semver/v3"

	"kraftkit.sh/exec"
)

// GetQemuVersionFromBin is direct method of accessing the version of the
// provided QEMU binary by executing it with the well-known flag `-version` and
// parsing its output.
func GetQemuVersionFromBin(ctx context.Context, bin string) (*semver.Version, error) {
	e, err := exec.NewExecutable(bin, QemuConfig{
		Version: true,
	})
	if err != nil {
		return nil, fmt.Errorf("could not prepare QEMU executable: %v", err)
	}

	var buf bytes.Buffer

	process, err := exec.NewProcessFromExecutable(e,
		exec.WithStdout(bufio.NewWriter(&buf)),
	)
	if err != nil {
		return nil, fmt.Errorf("could not prepare QEMU process: %v", err)
	}

	// Start and also wait for the process to be released, this ensures the
	// program is actively being executed.
	if err := process.StartAndWait(ctx); err != nil {
		return nil, fmt.Errorf("could not start and wait for QEMU process: %v", err)
	}

	// Get the first line of the returned value
	ret := strings.Split(strings.TrimSpace(buf.String()), "\n")[0]

	// Check if the returned value has the magic words
	if !strings.HasPrefix(ret, "QEMU emulator version ") {
		return nil, fmt.Errorf("malformed return value cannot parse QEMU version")
	}

	ret = strings.TrimPrefix(ret, "QEMU emulator version ")

	// Some QEMU versions include the OS distribution that it was compiled for
	// after the version number (surrounded by brackets).  In every case, just
	// split the string and gather everything before the first bracket.
	return semver.NewVersion(strings.TrimSpace(strings.Split(ret, " (")[0]))
}
