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
	indexCache   map[string]v1.ImageIndex
	indexCacheMu sync.Mutex
)

// RemoteIndex is a wrapper for v1.RemoteIndex which caches requests in-memory
// for previously requested indexes.  For tag-based references, a lightweight
// HEAD request is performed to resolve the current digest, which is then used
// as the cache key.  This ensures stale content is never returned when a tag
// changes, while still benefiting from caching when the tag points to the same
// digest.  Since v1.WithPlatform is not respected, a valid lookup will have had
// any additional options, such as WithTransport and WithAuth, fully satisfied.
func RemoteIndex(ref name.Reference, options ...remote.Option) (v1.ImageIndex, error) {
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
		indexCacheMu.Lock()
		if indexCache == nil {
			indexCache = make(map[string]v1.ImageIndex)
		}
		if index, ok := indexCache[key]; ok {
			indexCacheMu.Unlock()
			return index, nil
		}
		indexCacheMu.Unlock()
	}

	v1ImageIndex, err := remote.Index(ref, options...)
	if err != nil {
		return nil, err
	}

	if cacheable {
		indexCacheMu.Lock()
		indexCache[key] = v1ImageIndex
		indexCacheMu.Unlock()
	}

	return v1ImageIndex, nil
}
