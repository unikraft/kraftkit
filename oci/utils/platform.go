// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package utils

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// PlatformChecksum accepts an input manifest and generates a
// checksum based on the platform
func PlatformChecksum(seed string, manifest *ocispec.Platform) (string, error) {
	b, err := json.Marshal(manifest)
	if err != nil {
		return "", err
	}

	h := sha256.New()
	h.Write([]byte(seed))
	h.Write(b)
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
