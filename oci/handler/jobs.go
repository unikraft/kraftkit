// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package handler

import (
	"sync"

	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// jobs provides a way of identifying the download keys for a particular task
// encountering during the pull walk.
//
// This is very minimal and will probably be replaced with something more
// featured.
type jobs struct {
	name     string
	added    map[digest.Digest]struct{}
	descs    []ocispec.Descriptor
	mu       sync.Mutex
	resolved bool
}

// newJobs creates a new instance of the job status tracker
func newJobs(name string) *jobs {
	return &jobs{
		name:  name,
		added: map[digest.Digest]struct{}{},
	}
}

// Add adds a descriptor to be tracked
func (j *jobs) Add(desc ocispec.Descriptor) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.resolved = true

	if _, ok := j.added[desc.Digest]; ok {
		return
	}
	j.descs = append(j.descs, desc)
	j.added[desc.Digest] = struct{}{}
}

// Jobs returns a list of all tracked descriptors
func (j *jobs) Jobs() []ocispec.Descriptor {
	j.mu.Lock()
	defer j.mu.Unlock()

	var descs []ocispec.Descriptor
	return append(descs, j.descs...)
}

// IsResolved checks whether a descriptor has been resolved
func (j *jobs) IsResolved() bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.resolved
}
