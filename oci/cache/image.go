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
// to the same images.  For tag-based references, a lightweight HEAD request is
// performed to resolve the current digest, which is then used as the cache key.
// This ensures stale content is never returned when a tag changes, while still
// benefiting from caching when the tag points to the same digest.  Since
// v1.WithPlatform is not respected, a valid lookup will have had any additional
// options, such as WithTransport and WithAuth, fully satisfied.
func RemoteImage(ref name.Reference, options ...remote.Option) (v1.Image, error) {
	key := ref.String()
	cacheable := true

	// For tag-based references (mutable), resolve to the current digest via
	// a lightweight HEAD request so we can cache by the immutable digest.
	// If the HEAD request fails we cannot trust any previously cached entry
	// keyed by the mutable tag, so bypass the cache entirely for this call.
	if _, isDigest := ref.(name.Digest); !isDigest {
		desc, err := remote.Head(ref, options...)
		if err != nil {
			cacheable = false
		} else {
			key = ref.Context().Digest(desc.Digest.String()).String()
		}
	}

	if cacheable {
		imageCacheMu.Lock()
		if imageCache == nil {
			imageCache = make(map[string]v1.Image)
		}
		if image, ok := imageCache[key]; ok {
			imageCacheMu.Unlock()
			return image, nil
		}
		imageCacheMu.Unlock()
	}

	v1Image, err := remote.Image(ref, options...)
	if err != nil {
		return nil, err
	}

	if cacheable {
		imageCacheMu.Lock()
		imageCache[key] = v1Image
		imageCacheMu.Unlock()
	}

	return v1Image, nil
}
