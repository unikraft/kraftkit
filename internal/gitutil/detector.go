// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package gitutil

import (
	"net/url"
	"strings"
)

// IsGitSource returns true if the source string appears to be a git repository
// URL rather than an OCI image reference. It handles:
// - SSH URLs: ssh://git@host/path, git@host:path, git+ssh://, ssh+git://
// - HTTPS URLs: https://host/path.git, https://github.com/owner/repo
// - Bare paths: github.com/owner/repo, gitlab.com/owner/repo
func IsGitSource(source string) bool {
	if source == "" {
		return false
	}

	// Check for explicit SSH URL schemes
	for _, prefix := range []string{"ssh://", "ssh+git://", "git+ssh://", "git://"} {
		if strings.HasPrefix(source, prefix) {
			return true
		}
	}

	// Check for SCP-style SSH: git@host:path
	if strings.HasPrefix(source, "git@") {
		return true
	}

	// Try to parse as URL to distinguish from OCI refs
	// Normalize common patterns first
	normalized := source
	if strings.HasPrefix(source, "github.com/") ||
		strings.HasPrefix(source, "gitlab.com/") ||
		strings.HasPrefix(source, "bitbucket.org/") {
		normalized = "https://" + source
	}

	// Parse the URL
	u, err := url.Parse(normalized)
	if err != nil {
		// If it ends with .git but can't be parsed, it's likely a git source
		return strings.HasSuffix(source, ".git")
	}

	// Check for known git hosting platforms
	if isKnownGitHost(u.Host) {
		return true
	}

	// Check for .git suffix
	// Git URLs have .git at the end of the repository name
	if strings.HasSuffix(source, ".git") {
		if u.Scheme == "http" || u.Scheme == "https" {
			return true
		}
		// If no scheme and it contains a known git host, it's likely git
		if isKnownGitHost(u.Host) {
			return true
		}
		if strings.Contains(source, ":") || strings.Contains(source, "@") {
			// Check if .git comes before : or @
			gitIdx := strings.LastIndex(source, ".git")
			colonIdx := strings.Index(source, ":")
			atIdx := strings.Index(source, "@")

			// If there's a : or @ after .git, it's an OCI ref with tag/digest
			if (colonIdx > gitIdx && colonIdx != -1) || (atIdx > gitIdx && atIdx != -1) {
				return false
			}
		}
		// Otherwise, .git suffix suggests git source
		return true
	}

	return false
}

// isKnownGitHost checks if the hostname is a known git hosting service
func isKnownGitHost(host string) bool {
	// Remove port if present
	if idx := strings.Index(host, ":"); idx != -1 {
		host = host[:idx]
	}

	knownHosts := []string{
		"github.com",
		"gitlab.com",
		"bitbucket.org",
	}

	for _, known := range knownHosts {
		if strings.EqualFold(host, known) || strings.HasSuffix(host, "."+known) {
			return true
		}
	}

	return false
}
