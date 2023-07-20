// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package manifest

import (
	"kraftkit.sh/packmanager"
)

// FIXME(antoineco): avoid init, initialize things where needed
func init() {
	// Register a new pack.Package type
	_ = packmanager.RegisterPackageManager(ManifestFormat, NewManifestManager)
}
