// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package manifest

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"

	"kraftkit.sh/archive"
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
func pullArchive(ctx context.Context, manifest *Manifest, popts *pack.PackageOptions, ppopts *pack.PullPackageOptions) error {
	resource, cache, checksum, err := resourceCacheChecksum(manifest)
	if err != nil {
		return err
	}

	pp := &pullProgressArchive{
		onProgress: ppopts.OnProgress,
		total:      0,
		downloaded: 0,
	}

	if f, err := os.Stat(cache); !ppopts.UseCache() || err != nil || f.Size() == 0 {
		log.G(ctx).WithFields(logrus.Fields{
			"url":    resource,
			"method": "HEAD",
		}).Trace("http")

		// Get the total size of the remote resource.  Note: this fails for GitHub
		// archives as Content-Length is, for some reason, always set to 0.
		res, err := http.Head(resource)
		if err != nil {
			return fmt.Errorf("could not perform HEAD request on resource: %v", err)
		} else if res.StatusCode != http.StatusOK {
			return fmt.Errorf("received HTTP error code %d on resource", res.StatusCode)
		} else if res.ContentLength <= 0 {
			log.G(ctx).Warnf("could not determine package size before pulling")
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

		defer f.Close()

		log.G(ctx).WithFields(logrus.Fields{
			"url":    resource,
			"method": "HEAD",
		}).Trace("http")

		// Perform the request to actually retrieve the file
		res, err = http.Get(resource)
		if err != nil {
			return fmt.Errorf("could not initialize GET request to download package: %v", err)
		} else if res.StatusCode != http.StatusOK {
			return fmt.Errorf("received non-200 HTTP status code when attemptingn to download package: %v", err)
		}

		defer res.Body.Close()

		// With io.TeeReader we are able to pass in the implementing io.Writer such
		// that we are able to call the onProgress method
		_, err = io.Copy(f, io.TeeReader(res.Body, pp))
		if err != nil {
			return err
		}

		if ppopts.CalculateChecksum() {
			log.G(ctx).Debugf("calculating checksum for manifest package...")

			if len(checksum) == 0 {
				log.G(ctx).Warnf("manifest does not specify checksum!")
			} else {

				f, err := os.Open(tmpCache)
				if err != nil {
					return fmt.Errorf("could not perform checksum: %v", err)
				}
				defer f.Close()

				h := sha256.New()
				if _, err := io.Copy(h, f); err != nil {
					return fmt.Errorf("could not perform checksum: %v", err)
				}

				if checksum != string(h.Sum(nil)) {
					return fmt.Errorf("checksum of package does not match")
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
	}

	local := cache
	if len(ppopts.Workdir()) > 0 {
		local, err = unikraft.PlaceComponent(
			ppopts.Workdir(),
			manifest.Type,
			manifest.Name,
		)
		if err != nil {
			return fmt.Errorf("could not place component package: %s", err)
		}
	}

	// Unarchive the package to the given workdir
	if len(local) > 0 {
		if err := archive.Unarchive(cache, local,
			archive.StripComponents(1),
		); err != nil {
			return fmt.Errorf("could not unarchive: %v", err)
		}
	}

	return nil
}
