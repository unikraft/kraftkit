// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package pack

// PushOptions contains the list of options which can be set whilst pushing a
// package.
type PushOptions struct{}

// PushOption is an option function which is used to modify PushOptions.
type PushOption func(*PushOptions)
