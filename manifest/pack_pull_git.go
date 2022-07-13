// SPDX-License-Identifier: BSD-3-Clause
//
// Authors: Alexander Jung <alex@unikraft.io>
//
// Copyright (c) 2022, Unikraft GmbH.  All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions
// are met:
//
// 1. Redistributions of source code must retain the above copyright
//    notice, this list of conditions and the following disclaimer.
// 2. Redistributions in binary form must reproduce the above copyright
//    notice, this list of conditions and the following disclaimer in the
//    documentation and/or other materials provided with the distribution.
// 3. Neither the name of the copyright holder nor the names of its
//    contributors may be used to endorse or promote products derived from
//    this software without specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
// AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
// IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
// ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE
// LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR
// CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF
// SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS
// INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN
// CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE)
// ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE
// POSSIBILITY OF SUCH DAMAGE.

package manifest

import (
	"fmt"

	"github.com/libgit2/git2go/v31"

	"go.unikraft.io/kit/pack"
	"go.unikraft.io/kit/pkg/unikraft"
)

// pullGit is used internally to pull a specific Manifest resource using if the
// Manifest has the repo defined within.
func (mp ManifestPackage) pullGit(opts ...pack.PullPackageOption) error {
	popts, err := pack.NewPullPackageOptions(opts...)
	if err != nil {
		return err
	}

	if len(popts.Workdir()) == 0 {
		return fmt.Errorf("cannot Git clone manifest package without working directory")
	}

	manifest := mp.Context(ManifestContext).(*Manifest)
	if manifest == nil {
		return fmt.Errorf("package does not contain manifest context")
	}

	mp.Log().Infof("using git to pull manifest package %s", mp.CanonicalName())

	if len(manifest.GitRepo) == 0 {
		return fmt.Errorf("requesting Git with empty repository in manifest")
	}

	copts := &git.CloneOptions{
		FetchOptions: &git.FetchOptions{
			RemoteCallbacks: git.RemoteCallbacks{
				TransferProgressCallback: func(stats git.TransferProgress) git.ErrorCode {
					popts.OnProgress(float64(stats.IndexedObjects) / float64(stats.TotalObjects))
					return 0
				},
			},
		},
		CheckoutOpts: &git.CheckoutOptions{
			Strategy: git.CheckoutSafe,
		},
		CheckoutBranch: mp.Options().Version,
		Bare: false,
	}
	
	// TODO: Authetication.  This needs to be handled via the authentication
	// callback provided by CloneOptions.
	// Attribute any supplied authentication, supplied with hostname as key
	// u, err := url.Parse(manifest.GitRepo)
	// if err != nil {
	// 	return fmt.Errorf("could not parse Git repository: %v", err)
	// }

	// if auth, ok := manifest.auths[u.Host]; ok {
	// 	...
	// }

	local, err := unikraft.PlaceComponent(
		popts.Workdir(),
		manifest.Type,
		manifest.Name,
	)
	if err != nil {
		return fmt.Errorf("could not place component package: %s", err)
	}

	mp.Log().Infof("cloning %s into %s", manifest.GitRepo, local)

	_, err = git.Clone(manifest.GitRepo, local, copts)
	if err != nil {
		return fmt.Errorf("could not clone repository: %v", err)
	}

	mp.Log().Infof("successfulyl cloned %s into %s", manifest.GitRepo, local)

	return nil
}
