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

//go:build !windows

package config

import (
	"log"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"
)

// sudoUserInfo contains information about the original user when running under sudo.
type sudoUserInfo struct {
	HomeDir string
	UID     int
	GID     int
}

// getSudoUserInfo returns information about the original user when running under sudo.
// Returns nil if not running under sudo or if the original user cannot be determined.
func getSudoUserInfo() *sudoUserInfo {
	// Only use sudo detection when actually running as root
	if os.Getuid() != 0 {
		return nil
	}

	sudoUser := os.Getenv("SUDO_USER")
	if sudoUser == "" {
		return nil
	}

	// Try to get UID/GID from environment first (more reliable)
	uidStr := os.Getenv("SUDO_UID")
	gidStr := os.Getenv("SUDO_GID")

	var uid, gid int
	var homeDir string

	// Look up the user to get their home directory
	if u, err := user.Lookup(sudoUser); err == nil {
		homeDir = u.HomeDir
		// Use looked up UID/GID as fallback
		if uidStr == "" {
			uidStr = u.Uid
		}
		if gidStr == "" {
			gidStr = u.Gid
		}
	}

	if homeDir == "" {
		return nil
	}

	// Parse UID
	if uidStr != "" {
		if parsedUID, err := strconv.Atoi(uidStr); err == nil {
			uid = parsedUID
		}
	}

	// Parse GID
	if gidStr != "" {
		if parsedGID, err := strconv.Atoi(gidStr); err == nil {
			gid = parsedGID
		}
	}

	return &sudoUserInfo{
		HomeDir: homeDir,
		UID:     uid,
		GID:     gid,
	}
}

// getHomeDir returns the appropriate home directory for kraftkit configuration.
// When running under sudo, this returns the original user's home directory
// to maintain consistent config paths between privileged and unprivileged commands.
func getHomeDir() string {
	if info := getSudoUserInfo(); info != nil {
		// Verify the home directory exists and is accessible before using it
		if _, err := os.Stat(info.HomeDir); err == nil {
			return info.HomeDir
		}
		// Fall back to os.UserHomeDir() if sudo user's home is not accessible
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Printf("warning: could not determine home directory: %v", err)
	}
	return homeDir
}

// ChownToUser changes ownership of a path to the original user when running under sudo.
// This ensures that files created by sudo commands are accessible by the regular user.
// If not running under sudo, this is a no-op.
func ChownToUser(path string) error {
	info := getSudoUserInfo()
	if info == nil || info.UID == 0 {
		// Not running under sudo or couldn't determine original user
		return nil
	}

	return os.Chown(path, info.UID, info.GID)
}

// ChownToUserRecursive changes ownership of a path and all its contents to the original user.
// This is useful for directories created by sudo commands.
// Only files/dirs owned by root are chowned to avoid unnecessary syscalls.
func ChownToUserRecursive(path string) error {
	info := getSudoUserInfo()
	if info == nil || info.UID == 0 {
		return nil
	}

	return filepath.Walk(path, func(name string, fileInfo os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if stat, ok := fileInfo.Sys().(*syscall.Stat_t); ok {
			if stat.Uid != 0 {
				return nil
			}
		}
		return os.Chown(name, info.UID, info.GID)
	})
}
