// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package firecracker

// FirecrackerConfig is a subset of the Firecracker's Go SDK structure of the
// same format.  We use this subset because these are the only attribute
// necessary and additionally, gob cannot register some of the embedded types.
type FirecrackerConfig struct {
	SocketPath string `json:"socketPath,omitempty"`
	BootArgs   string `json:"bootArgs,omitempty"`
	LogPath    string `json:"logPath,omitempty"`

	// TODO(craciunouc): This is a temporary solution until we have proper
	// un/marshalling of the resources (and all structures).
	Memory string `json:"memory,omitempty"`
}
