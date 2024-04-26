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
// for previously requested indexes.  This aids in reducing the number of lookup
// calls to the same index.  Since v1.WithPlatform is not respected, a valid
// lookup will have had any additional options, such as WithTransport and
// WithAuth, fully satisfied.
func RemoteIndex(ref name.Reference, options ...remote.Option) (v1.ImageIndex, error) {
	name := ref.Name()

	if indexCache == nil {
		indexCacheMu.Lock()
		indexCache = make(map[string]v1.ImageIndex)
		indexCacheMu.Unlock()
		goto lookup
	}

	indexCacheMu.Lock()
	if index, ok := indexCache[name]; ok {
		indexCacheMu.Unlock()
		return index, nil
	}
	indexCacheMu.Unlock()

lookup:
	v1ImageIndex, err := remote.Index(ref, options...)
	if err != nil {
		return nil, err
	}

	indexCacheMu.Lock()
	indexCache[name] = v1ImageIndex
	indexCacheMu.Unlock()

	return v1ImageIndex, nil
}
