// SPDX-License-Identifier: MIT
//
// Copyright (c) 2019 GitHub Inc.
//               2022 Unikraft GmbH.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

//go:build windows

package config

import (
	"log"
	"os"
)

// sudoUserInfo contains information about the original user when running under sudo.
type sudoUserInfo struct {
	HomeDir string
	UID     int
	GID     int
}

// getSudoUserInfo returns information about the original user when running under sudo.
// On Windows, sudo is not applicable so this always returns nil.
func getSudoUserInfo() *sudoUserInfo {
	return nil
}

// getHomeDir returns the home directory for kraftkit configuration.
// On Windows, sudo is not applicable, so this simply returns os.UserHomeDir().
func getHomeDir() string {
	// Dummy call for interface consistency with Unix implementation
	_ = getSudoUserInfo()

	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Printf("warning: could not determine home directory: %v", err)
	}
	return homeDir
}

// ChownToUser is a no-op on Windows as sudo/chown concepts don't apply.
func ChownToUser(path string) error {
	return nil
}

// ChownToUserRecursive is a no-op on Windows as sudo/chown concepts don't apply.
func ChownToUserRecursive(path string) error {
	return nil
}
