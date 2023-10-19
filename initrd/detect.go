// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package initrd

import (
	"context"
	"fmt"
)

// New attempts to return the builder for a supplied path which
// will allow the provided ...
func New(ctx context.Context, path string, opts ...InitrdOption) (Initrd, error) {
	if builder, err := NewFromDockerfile(ctx, path, opts...); err == nil {
		return builder, nil
	} else if builder, err := NewFromDirectory(ctx, path, opts...); err == nil {
		return builder, nil
	} else if builder, err := NewFromFile(ctx, path, opts...); err == nil {
		return builder, nil
	}

	return nil, fmt.Errorf("could not determine how to build initrd from: %s", path)
}
