// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

//go:build xen
// +build xen

package xen

import (
	"encoding/gob"

	"xenbits.xenproject.org/git-http/xen.git/tools/golang/xenlight"
)

func init() {
	gob.Register(xenlight.Domid(0))
}
