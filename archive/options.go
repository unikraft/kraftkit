// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package archive

type UnarchiveOptions struct {
	stripComponents int
}

type UnarchiveOption func(uo *UnarchiveOptions) error

func StripComponents(sc int) UnarchiveOption {
	return func(uo *UnarchiveOptions) error {
		if sc < 0 {
			sc = 0
		}

		uo.stripComponents = sc
		return nil
	}
}
