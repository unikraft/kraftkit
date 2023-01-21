// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package manifest

import (
	"context"
	"fmt"

	"github.com/libgit2/git2go/v31"

	"kraftkit.sh/log"
	"kraftkit.sh/pack"
	"kraftkit.sh/unikraft"
)

// pullGit is used internally to pull a specific Manifest resource using if the
// Manifest has the repo defined within.
func pullGit(ctx context.Context, manifest *Manifest, popts *pack.PackageOptions, ppopts *pack.PullPackageOptions) error {
	if len(ppopts.Workdir()) == 0 {
		return fmt.Errorf("cannot Git clone manifest package without working directory")
	}

	log.G(ctx).Infof("using git to pull manifest package %s", manifest.Name)

	if len(manifest.Origin) == 0 {
		return fmt.Errorf("requesting Git with empty repository in manifest")
	}

	copts := &git.CloneOptions{
		FetchOptions: &git.FetchOptions{
			RemoteCallbacks: git.RemoteCallbacks{
				TransferProgressCallback: func(stats git.TransferProgress) git.ErrorCode {
					ppopts.OnProgress(float64(stats.IndexedObjects) / float64(stats.TotalObjects))
					return 0
				},
			},
		},
		CheckoutOpts: &git.CheckoutOptions{
			Strategy: git.CheckoutSafe,
		},
		CheckoutBranch: popts.Version,
		Bare:           false,
	}

	// TODO: Authetication.  This needs to be handled via the authentication
	// callback provided by CloneOptions.
	// Attribute any supplied authentication, supplied with hostname as key
	// u, err := url.Parse(manifest.Origin)
	// if err != nil {
	// 	return fmt.Errorf("could not parse Git repository: %v", err)
	// }

	// if auth, ok := manifest.auths[u.Host]; ok {
	// 	...
	// }

	local, err := unikraft.PlaceComponent(
		ppopts.Workdir(),
		manifest.Type,
		manifest.Name,
	)
	if err != nil {
		return fmt.Errorf("could not place component package: %s", err)
	}

	log.G(ctx).Infof("cloning %s into %s", manifest.Origin, local)

	_, err = git.Clone(manifest.Origin, local, copts)
	if err != nil {
		return fmt.Errorf("could not clone repository: %v", err)
	}

	log.G(ctx).Infof("successfulyl cloned %s into %s", manifest.Origin, local)

	return nil
}
