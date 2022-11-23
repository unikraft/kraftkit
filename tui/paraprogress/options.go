// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package paraprogress

import (
	"kraftkit.sh/log"
)

type ParaProgressOption func(md *ParaProgress) error

func WithLogger(l log.Logger) ParaProgressOption {
	return func(md *ParaProgress) error {
		md.log = l
		return nil
	}
}

func WithRenderer(norender bool) ParaProgressOption {
	return func(md *ParaProgress) error {
		md.norender = norender
		return nil
	}
}

func IsParallel(parallel bool) ParaProgressOption {
	return func(md *ParaProgress) error {
		md.parallel = parallel
		return nil
	}
}

func WithFailFast(failFast bool) ParaProgressOption {
	return func(pp *ParaProgress) error {
		pp.failFast = failFast
		return nil
	}
}
