// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package cache

import (
	"sync"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

var (
	imageCache   map[string]v1.Image
	imageCacheMu sync.Mutex
)

// RemoteImage is a wrapper for v1.Image which caches requests in-memory for
// previously requested image.  This aids in reducing the number of lookup calls
// to the same images.  Since v1.WithPlatform is not respected, a valid lookup
// will have had any additional options, such as WithTransport and WithAuth,
// fully satisfied.
func RemoteImage(ref name.Reference, options ...remote.Option) (v1.Image, error) {
	name := ref.Name()

	if imageCache == nil {
		imageCache = make(map[string]v1.Image)
		goto lookup
	}

	if image, ok := imageCache[name]; ok {
		return image, nil
	}

lookup:
	v1Image, err := remote.Image(ref, options...)
	if err != nil {
		return nil, err
	}

	imageCacheMu.Lock()
	imageCache[name] = v1Image
	imageCacheMu.Unlock()

	return v1Image, nil
}
