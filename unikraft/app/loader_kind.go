// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package app

import "fmt"

// ProjectLoader identifies which Kraftfile loader path produced an Application.
type ProjectLoader string

const (
	ProjectLoaderV06 ProjectLoader = "v0.6"
	ProjectLoaderV07 ProjectLoader = "v0.7"
)

var ErrProjectMutationNotSupported = fmt.Errorf("project mutation is not supported for this Kraftfile loader")
