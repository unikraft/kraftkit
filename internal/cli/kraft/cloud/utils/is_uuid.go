// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package utils

import "github.com/google/uuid"

// IsUUID returns whether the provided the string is a UUID or not.
func IsUUID(arg string) bool {
	_, uuidErr := uuid.Parse(arg)
	return uuidErr == nil
}
