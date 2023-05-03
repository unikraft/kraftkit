// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package manifest

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"

	libgit "github.com/libgit2/git2go/v34"

	"kraftkit.sh/log"
	"kraftkit.sh/pack"
	"kraftkit.sh/unikraft"
)

// pullGit is used internally to pull a specific Manifest resource using if the
// Manifest has the repo defined within.
func pullGit(ctx context.Context, manifest *Manifest, opts ...pack.PullOption) error {
	popts, err := pack.NewPullOptions(opts...)
	if err != nil {
		return err
	}

	if len(popts.Workdir()) == 0 {
		return fmt.Errorf("cannot Git clone manifest package without working directory")
	}

	log.G(ctx).Debugf("using git to pull manifest package %s", manifest.Name)

	if len(manifest.Origin) == 0 {
		return fmt.Errorf("requesting Git with empty repository in manifest")
	}

	copts := &libgit.CloneOptions{
		FetchOptions: libgit.FetchOptions{
			RemoteCallbacks: libgit.RemoteCallbacks{
				TransferProgressCallback: func(stats libgit.TransferProgress) error {
					popts.OnProgress((float64(stats.IndexedObjects) / float64(stats.TotalObjects)) / 2)
					return nil
				},
				PackProgressCallback: func(_ int32, current, total uint32) error {
					popts.OnProgress(0.5 + (float64(current)/float64(total))/2)
					return nil
				},
				// Warning: Returning "error OK" here prevents hostkey look ups
				CertificateCheckCallback: func(cert *libgit.Certificate, _ bool, hostname string) error {
					if cert != nil && len(cert.Hostkey.Hostkey) > 0 {
						log.G(ctx).
							WithField("hostname", hostname).
							WithField("sha256", hex.EncodeToString(cert.Hostkey.HashSHA256[:])).
							Warn("hostkey look up disabled")
					}
					return nil
				},
			},
		},
		CheckoutOptions: libgit.CheckoutOptions{
			Strategy: libgit.CheckoutForce, // .CheckoutSafe,
		},
		Bare: false,
	}

	path := manifest.Origin

	// Is this an SSH URL?
	if isSSHURL(path) {
		// This is a quirk of git2go, if we have determined it was an SSH path and
		// it does not contain the prefix, we should include it so it can be
		// recognised internally by the module.
		if strings.HasPrefix(path, "git@") {
			path = "ssh://" + path
		}

		copts.FetchOptions.RemoteCallbacks.CredentialsCallback = func(_, usernameFromUrl string, allowedTypes libgit.CredentialType) (*libgit.Credential, error) {
			if allowedTypes == libgit.CredentialTypeUsername && len(usernameFromUrl) == 0 {
				return nil, fmt.Errorf("username required in SSH URL")
			}

			return libgit.NewCredentialSSHKeyFromAgent(usernameFromUrl)
		}
	} else {
		copts.FetchOptions.RemoteCallbacks.CredentialsCallback = func(fullUrl, _ string, _ libgit.CredentialType) (*libgit.Credential, error) {
			u, err := url.Parse(fullUrl)
			if err != nil {
				return nil, fmt.Errorf("could not parse git repository: %v", err)
			}

			if auth, ok := manifest.Auths()[u.Host]; ok {
				if len(auth.User) > 0 && len(auth.Token) > 0 {
					return libgit.NewCredentialUserpassPlaintext(auth.User, auth.Token)
				}
			}

			return libgit.NewCredentialDefault()
		}
	}

	if popts.Version() != "" {
		copts.CheckoutBranch = popts.Version()
	}

	local, err := unikraft.PlaceComponent(
		popts.Workdir(),
		manifest.Type,
		manifest.Name,
	)
	if err != nil {
		return fmt.Errorf("could not place component package: %s", err)
	}

	log.G(ctx).
		WithField("from", path).
		WithField("to", local).
		WithField("branch", popts.Version()).
		Infof("git clone")

	if _, err = libgit.Clone(path, local, copts); err != nil {
		return fmt.Errorf("could not clone repository: %v", err)
	}

	log.G(ctx).Infof("successfully cloned %s into %s", path, local)

	return nil
}
