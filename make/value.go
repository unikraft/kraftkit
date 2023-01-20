// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package make

// ConditionalValue represents an expression in a Makefile in the form of
// `VARIABLE-$(CONDITION) += VALUE`.  This is a typical convention in C-based
// projects where the `$(CONDITION)` variable either resolves itself to a
// variable or value, e.g. `y`.
type ConditionalValue struct {
	DependsOn *string
	Value     string
}
