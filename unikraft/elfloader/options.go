// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package elfloader

import (
	"kraftkit.sh/unikraft/app"
)

type ELFLoaderOption func(*ELFLoader) error

func WithApplicationOptions(aopts ...app.ApplicationOption) ELFLoaderOption {
	return func(ef *ELFLoader) error {
		ef.aopts = append(ef.aopts, aopts...)
		return nil
	}
}
