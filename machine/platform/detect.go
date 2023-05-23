// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package platform

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"kraftkit.sh/internal/set"
)

type SystemMode string

const (
	SystemUnknown = SystemMode("unknown")
	SystemGuest   = SystemMode("guest")
	SystemHost    = SystemMode("host")
)

// getenv retrieves the environment variable key. If it does not exist it
// returns the default.
func getenv(key string, dfault string, combineWith ...string) string {
	value := os.Getenv(key)
	if value == "" {
		value = dfault
	}

	switch len(combineWith) {
	case 0:
		return value
	case 1:
		return filepath.Join(value, combineWith[0])
	default:
		all := make([]string, len(combineWith)+1)
		all[0] = value
		copy(all[1:], combineWith)
		return filepath.Join(all...)
	}
}

// hostProc returns the provided procfs path, using environmental variable to
// allow base bath configuration.
func hostProc(path ...string) string {
	return getenv("HOST_PROC", "/proc", path...)
}

// pathExists simply returns whether the provided file path exists.
func pathExists(file string) bool {
	if _, err := os.Stat(file); err == nil {
		return true
	}
	return false
}

// readLines reads contents from a file and splits them by new lines.
// A convenience wrapper to readLinesOffsetN(file, 0, -1).
func readLines(file string) ([]string, error) {
	return readLinesOffsetN(file, 0, -1)
}

// readLinesOffsetN reads contents from file and splits them by new line.
// The offset tells at which line number to start.
// The count determines the number of lines to read (starting from offset):
// n >= 0: at most n lines
// n < 0: whole file
func readLinesOffsetN(file string, offset uint, n int) ([]string, error) {
	f, err := os.Open(file)
	if err != nil {
		return []string{""}, err
	}
	defer f.Close()

	var ret []string

	r := bufio.NewReader(f)
	for i := 0; i < n+int(offset) || n < 0; i++ {
		line, err := r.ReadString('\n')
		if err != nil {
			if err == io.EOF && len(line) > 0 {
				ret = append(ret, strings.Trim(line, "\n"))
			}
			break
		}
		if i < int(offset) {
			continue
		}
		ret = append(ret, strings.Trim(line, "\n"))
	}

	return ret, nil
}

// Detect returns the hypervisor and system mode in the context to the
// determined hypervisor or an error if not detectable.
func Detect(ctx context.Context) (Platform, SystemMode, error) {
	file := hostProc("xen")
	if pathExists(file) {
		system := PlatformXen
		role := SystemGuest // assume guest

		if pathExists(filepath.Join(file, "capabilities")) {
			contents, err := readLines(filepath.Join(file, "capabilities"))
			if err == nil {
				if set.NewStringSet(contents...).Contains("control_d") {
					role = SystemHost
				}
			}
		}

		return system, role, nil
	}

	file = hostProc("modules")
	if pathExists(file) {
		contents, err := readLines(file)
		if err == nil {
			if set.NewStringSet(contents...).Contains("kvm") {
				return PlatformKVM, SystemHost, nil
			}
		}
	}

	file = hostProc("cpuinfo")
	if pathExists(file) {
		contents, err := readLines(file)
		if err == nil {
			if set.NewStringSet(contents...).Contains("QEMU Virtual CPU") {
				return PlatformQEMU, SystemGuest, nil
			} else if set.NewStringSet(contents...).Contains("Common KVM processor") {
				return PlatformKVM, SystemGuest, nil
			} else if set.NewStringSet(contents...).Contains("Common 32-bit KVM processor") {
				return PlatformKVM, SystemGuest, nil
			}
		}
	}

	return PlatformUnknown, SystemUnknown, fmt.Errorf("could not determine hypervisor and system mode")
}
