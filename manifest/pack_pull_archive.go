// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package manifest

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"

	"kraftkit.sh/archive"
	"kraftkit.sh/internal/version"
	"kraftkit.sh/log"
	"kraftkit.sh/pack"
	"kraftkit.sh/unikraft"
)

type pullProgressArchive struct {
	total      int
	downloaded int
	onProgress func(float64)
}

func (pp *pullProgressArchive) Write(p []byte) (n int, err error) {
	pp.downloaded += len(p)

	if pp.total > 0 && pp.onProgress != nil {
		pp.onProgress(float64(pp.downloaded) / float64(pp.total))
	}

	return len(p), nil
}

// pullArchive is used internally to pull a specific Manifest resource using the
// conventional archive.
func pullArchive(ctx context.Context, manifest *Manifest, opts ...pack.PullOption) error {
	popts, err := pack.NewPullOptions(opts...)
	if err != nil {
		return err
	}

	resource, cache, checksum, err := resourceCacheChecksum(manifest)
	if err != nil {
		return err
	}

	pp := &pullProgressArchive{
		onProgress: popts.OnProgress,
		total:      0,
		downloaded: 0,
	}

	if f, err := os.Stat(cache); !popts.UseCache() || err != nil || f.Size() == 0 {
		u, err := url.Parse(resource)
		if err != nil {
			return err
		}

		authHeader := ""
		authenticated := false

		if auth := popts.Auths(u.Host); auth != nil {
			if len(auth.User) > 0 {
				authenticated = true
				authHeader = "Basic " + base64.StdEncoding.
					EncodeToString([]byte(auth.User+":"+auth.Token))
			} else if len(auth.Token) > 0 {
				authenticated = true
				authHeader = "Bearer " + auth.Token
			}
		}

		client := &http.Client{}

		head, err := http.NewRequestWithContext(ctx, "HEAD", resource, nil)
		if err != nil {
			return err
		}

		head.Header.Set("User-Agent", version.UserAgent())
		if authenticated {
			head.Header.Set("Authorization", authHeader)
		}

		log.G(ctx).WithFields(logrus.Fields{
			"url":           resource,
			"method":        "HEAD",
			"authenticated": authenticated,
		}).Trace("http")

		// Get the total size of the remote resource.  Note: this fails for GitHub
		// archives as Content-Length is, for some reason, always set to 0.
		res, err := client.Do(head)
		if err != nil {
			return fmt.Errorf("could not perform HEAD request on resource: %v", err)
		} else if res.StatusCode != http.StatusOK {
			return fmt.Errorf("received HTTP error code %d on resource", res.StatusCode)
		} else if res.ContentLength <= 0 {
			log.G(ctx).Tracef("could not determine package size before pulling")
			pp.total = 0
		} else {
			pp.total = int(res.ContentLength)
		}

		// Create a temporary partial of the destination path of the resource
		tmpCache := cache + ".part"
		if err := os.MkdirAll(filepath.Dir(tmpCache), 0o755); err != nil {
			return fmt.Errorf("could not create parent directorires: %v", err)
		}

		f, err := os.OpenFile(tmpCache, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o755)
		if err != nil {
			return fmt.Errorf("could not create cache file: %v", err)
		}

		get, err := http.NewRequestWithContext(ctx, "GET", resource, nil)
		if err != nil {
			return err
		}

		get.Header.Set("User-Agent", version.UserAgent())
		if authenticated {
			head.Header.Set("Authorization", authHeader)
		}

		log.G(ctx).WithFields(logrus.Fields{
			"url":           resource,
			"method":        "GET",
			"authenticated": authenticated,
		}).Trace("http")

		// Perform the request to actually retrieve the file
		res, err = client.Do(get)
		if err != nil {
			return fmt.Errorf("could not initialize GET request to download package: %v", err)
		} else if res.StatusCode != http.StatusOK {
			return fmt.Errorf("received non-200 HTTP status code when attempting to download package: %v", err)
		}

		defer res.Body.Close()

		// With io.TeeReader we are able to pass in the implementing io.Writer such
		// that we are able to call the onProgress method
		_, err = io.Copy(f, io.TeeReader(res.Body, pp))
		if err != nil {
			return err
		}

		err = f.Close()
		if err != nil {
			return fmt.Errorf("could not close file '%s' %s", tmpCache, err)
		}

		if popts.CalculateChecksum() {
			log.G(ctx).Debugf("calculating checksum for manifest package...")

			if len(checksum) == 0 {
				log.G(ctx).Warnf("manifest does not specify checksum!")
			} else {

				f, err := os.Open(tmpCache)
				if err != nil {
					return fmt.Errorf("could not perform checksum: %v", err)
				}

				h := sha256.New()
				if _, err := io.Copy(h, f); err != nil {
					return fmt.Errorf("could not perform checksum: %v", err)
				}

				if checksum != string(h.Sum(nil)) {
					return fmt.Errorf("checksum of package does not match")
				}

				err = f.Close()
				if err != nil {
					return fmt.Errorf("could not close file '%s' %s", tmpCache, err)
				}

				log.G(ctx).WithFields(logrus.Fields{
					"url":      resource,
					"checksum": checksum,
				}).Debug("checksum OK")
			}
		}

		// Copy the completed download to the local cache path
		if err := os.Rename(tmpCache, cache); err != nil {
			return fmt.Errorf("could not move downloaded package '%s' to destination '%s': %v", tmpCache, cache, err)
		}
	} else {
		log.G(ctx).WithFields(logrus.Fields{
			"local":  cache,
			"remote": resource,
		}).Debug("using cache")
	}

	local := cache
	if len(popts.Workdir()) > 0 {
		local, err = unikraft.PlaceComponent(
			popts.Workdir(),
			manifest.Type,
			manifest.Name,
		)
		if err != nil {
			return fmt.Errorf("could not place component package: %s", err)
		}
	}

	// Unarchive the package to the given workdir
	if len(local) > 0 {
		log.G(ctx).WithFields(logrus.Fields{
			"from": cache,
			"to":   local,
		}).Trace("unarchiving")

		if err := archive.Unarchive(cache, local,
			archive.StripComponents(1),
		); err != nil {
			return fmt.Errorf("could not unarchive: %v", err)
		}
	}

	return nil
}
