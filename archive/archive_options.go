// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package archive

type ArchiveOptions struct {
	stripTimes bool
	gzip       bool
}

type ArchiveOption func(*ArchiveOptions) error

// WithStripTimes indicates that during the archival process that any file times
// should be removed.
func WithStripTimes(stripTimes bool) ArchiveOption {
	return func(ao *ArchiveOptions) error {
		ao.stripTimes = stripTimes
		return nil
	}
}

// WithGzip indicates that when archiving occurs that the resulting artifact
// should be gzip compressed.
func WithGzip(gzip bool) ArchiveOption {
	return func(ao *ArchiveOptions) error {
		ao.gzip = gzip
		return nil
	}
}
