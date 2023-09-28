// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package platform

// hostSupportedStrategies returns the map of known supported drivers for the
// given host.
// No drivers are supported on Windows currently. Future HyperV support is possible.
func hostSupportedStrategies() map[Platform]*Strategy {
	s := map[Platform]*Strategy{}

	return s
}
