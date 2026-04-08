// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package remove

import "errors"

var (
	ErrServiceAndPolicyRequired = errors.New("specify service UUID and policy name")
	ErrInvalidServiceUUID       = errors.New("specify a valid service UUID")
)
