//go:build tools

// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package hack

// These imports ensure that build tools are included in Go modules so that we
// can `go install` them in module-aware mode.
// See https://github.com/golang/go/wiki/Modules#how-can-i-track-tool-dependencies-for-a-module
import _ "github.com/onsi/ginkgo/v2/ginkgo"
