// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package manifest

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"

	"kraftkit.sh/config"
	"kraftkit.sh/internal/version"
	"kraftkit.sh/log"
	"kraftkit.sh/unikraft"
)

// archiveCachePath computes the local cache path for a manifest resource
// archive. Slashes in the URL path are replaced with dashes to produce a flat
// filename.
func archiveCachePath(manifest *Manifest, resource string) string {
	name := filepath.Base(resource)
	if u, err := url.Parse(resource); err == nil && u.Path != "" {
		name = strings.ReplaceAll(strings.TrimPrefix(u.Path, "/"), "/", "-")
	}

	cache := manifest.Name + string(filepath.Separator) + name

	if manifest.Type != unikraft.ComponentTypeCore {
		cache = manifest.Type.Plural() + string(filepath.Separator) + cache
	}

	return filepath.Join(manifest.mopts.cacheDir, cache)
}

// invalidateStaleArchiveCache checks if a cached archive is outdated by
// performing a lightweight HTTP HEAD request and comparing the ETag header
// against a locally stored ETag sidecar file. If they differ, the cached
// archive is removed so pullArchive will re-download it.
func invalidateStaleArchiveCache(ctx context.Context, manifest *Manifest, resource string) (string, error) {
	if manifest.mopts == nil || manifest.mopts.cacheDir == "" {
		return "", nil
	}

	remoteETag, err := fetchRemoteETag(ctx, manifest, resource)
	if err != nil {
		log.G(ctx).WithError(err).Debug("could not fetch remote ETag, skipping cache validation")
		return "", nil
	}

	cache := archiveCachePath(manifest, resource)

	// Nothing to invalidate if the cache file does not exist.
	if _, err := os.Stat(cache); err != nil {
		return remoteETag, nil
	}

	etagFile := cache + ".etag"
	storedETag, err := os.ReadFile(etagFile)
	if err == nil && strings.TrimSpace(string(storedETag)) == remoteETag {
		log.G(ctx).WithFields(logrus.Fields{
			"resource": resource,
			"etag":     remoteETag,
		}).Debug("cached archive is up-to-date")
		return remoteETag, nil
	}

	if err == nil {
		log.G(ctx).WithFields(logrus.Fields{
			"resource":   resource,
			"remoteETag": remoteETag,
			"cachedETag": strings.TrimSpace(string(storedETag)),
		}).Info("cached archive is stale, invalidating")
	} else {
		log.G(ctx).WithFields(logrus.Fields{
			"resource": resource,
			"etag":     remoteETag,
		}).Debug("no cached ETag sidecar found, invalidating cache to be safe")
	}

	// Remove the stale archive so pullArchive will re-download it.
	if err := os.Remove(cache); err != nil && !os.IsNotExist(err) {
		return remoteETag, fmt.Errorf("could not remove stale cache file: %w", err)
	}

	return remoteETag, nil
}

// writeArchiveCacheETag writes the ETag to a sidecar file next to the cached
// archive.
func writeArchiveCacheETag(_ context.Context, manifest *Manifest, resource string, etag string) error {
	if manifest.mopts == nil || manifest.mopts.cacheDir == "" || etag == "" {
		return nil
	}

	cache := archiveCachePath(manifest, resource)
	etagFile := cache + ".etag"

	if err := os.MkdirAll(filepath.Dir(etagFile), 0o755); err != nil {
		return fmt.Errorf("could not create directory for cache ETag: %w", err)
	}

	if err := os.WriteFile(etagFile, []byte(etag+"\n"), 0o644); err != nil {
		return fmt.Errorf("could not write cache ETag: %w", err)
	}

	return nil
}

// setRequestAuth sets the Authorization header on r using the first matching
// credentials for r's host in auths.
func setRequestAuth(r *http.Request, auths map[string]config.AuthConfig) bool {
	if auths == nil {
		return false
	}
	auth, ok := auths[r.URL.Host]
	if !ok {
		return false
	}
	if len(auth.User) > 0 {
		r.Header.Set("Authorization", "Basic "+
			base64.StdEncoding.EncodeToString([]byte(auth.User+":"+auth.Token)))
		return true
	} else if len(auth.Token) > 0 {
		r.Header.Set("Authorization", "Bearer "+auth.Token)
		return true
	}
	return false
}

// fetchRemoteETag performs an HTTP HEAD request to the resource URL and returns
// the ETag header value.
func fetchRemoteETag(ctx context.Context, manifest *Manifest, resource string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "HEAD", resource, nil)
	if err != nil {
		return "", fmt.Errorf("could not create HEAD request: %w", err)
	}

	req.Header.Set("User-Agent", version.UserAgent())

	if manifest.mopts != nil {
		setRequestAuth(req, manifest.mopts.auths)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("HEAD request failed for %s: %w", resource, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("HEAD request returned status %d for %s", resp.StatusCode, resource)
	}

	etag := resp.Header.Get("ETag")
	if etag == "" {
		return "", fmt.Errorf("no ETag header in response for %s", resource)
	}

	return etag, nil
}
