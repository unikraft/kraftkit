// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package manifest

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"kraftkit.sh/unikraft"
)

func TestArchiveCachePath(t *testing.T) {
	m := &Manifest{
		Name: "lib-musl",
		Type: unikraft.ComponentTypeLib,
		mopts: &ManifestOptions{
			cacheDir: "/tmp/kraft-cache",
		},
	}

	got := archiveCachePath(m, "https://github.com/unikraft/lib-musl/archive/refs/heads/stable.tar.gz")
	want := filepath.Join("/tmp/kraft-cache", "libs", "lib-musl", "unikraft-lib-musl-archive-refs-heads-stable.tar.gz")
	if got != want {
		t.Errorf("archiveCachePath() = %q, want %q", got, want)
	}

	// Branches with slashes must produce distinct cache paths.
	gotA := archiveCachePath(m, "https://github.com/unikraft/lib-musl/archive/refs/heads/feature/new-api.tar.gz")
	gotB := archiveCachePath(m, "https://github.com/unikraft/lib-musl/archive/refs/heads/bugfix/new-api.tar.gz")
	if gotA == gotB {
		t.Errorf("branches with different slash-prefixed names collide: %q", gotA)
	}
}

func TestFetchRemoteETag(t *testing.T) {
	const testETag = `"abc123"`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead {
			t.Errorf("expected HEAD request, got %s", r.Method)
		}
		w.Header().Set("ETag", testETag)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	m := &Manifest{mopts: &ManifestOptions{}}
	etag, err := fetchRemoteETag(t.Context(), m, srv.URL+"/archive.tar.gz")
	if err != nil {
		t.Fatalf("fetchRemoteETag() error: %v", err)
	}
	if etag != testETag {
		t.Errorf("fetchRemoteETag() = %q, want %q", etag, testETag)
	}
}

func TestWriteAndReadArchiveCacheETag(t *testing.T) {
	tmp := t.TempDir()
	m := &Manifest{
		Name: "unikraft",
		Type: unikraft.ComponentTypeCore,
		mopts: &ManifestOptions{
			cacheDir: tmp,
		},
	}

	resource := "https://github.com/unikraft/unikraft/archive/refs/heads/staging.tar.gz"
	etag := `"deadbeef"`

	if err := writeArchiveCacheETag(t.Context(), m, resource, etag); err != nil {
		t.Fatalf("writeArchiveCacheETag() error: %v", err)
	}

	cache := archiveCachePath(m, resource)
	etagFile := cache + ".etag"

	data, err := os.ReadFile(etagFile)
	if err != nil {
		t.Fatalf("could not read etag sidecar: %v", err)
	}

	if got := string(data); got != etag+"\n" {
		t.Errorf("etag sidecar content = %q, want %q", got, etag+"\n")
	}
}
