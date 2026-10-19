// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package network

import (
	"context"
	"os"
	"path/filepath"

	"kraftkit.sh/config"
	"kraftkit.sh/internal/lockedfile"
)

// Lock acquires an exclusive, cross-process lock for the named network and
// returns a function that releases it.  Hold it around reading a network,
// changing its interfaces and updating it, so concurrent kraft processes do
// not overwrite each other's changes.
func Lock(ctx context.Context, name string) (func(), error) {
	dir := config.G[config.KraftKit](ctx).RuntimeDir
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	// Use only the base name so the lock file cannot be placed outside dir.
	return lockedfile.MutexAt(filepath.Join(dir,
		"networkv1alpha1-"+filepath.Base(name)+".lock")).Lock()
}
