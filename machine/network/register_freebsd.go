// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package network

var defaultStrategyName = ""

// hostSupportedStrategies returns the map of known supported drivers for the
// given host.
// Currently stubbed out for FreeBSD.
func hostSupportedStrategies() map[string]*Strategy {
	return map[string]*Strategy{}
}
