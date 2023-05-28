// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package oci

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"

	"github.com/docker/docker/api/types"
	regtool "github.com/genuinetools/reg/registry"
	"github.com/genuinetools/reg/repoutils"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"

	"kraftkit.sh/config"
	"kraftkit.sh/internal/version"
	"kraftkit.sh/log"
	"kraftkit.sh/oci/handler"
	"kraftkit.sh/pack"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/unikraft/component"
	"kraftkit.sh/unikraft/target"
)

type ociManager struct{}

const OCIFormat pack.PackageFormat = "oci"

// NewOCIManager instantiates a new package manager based on OCI archives.
func NewOCIManager(ctx context.Context) (packmanager.PackageManager, error) {
	manager := ociManager{}

	// Attempt to initialize the handler simply to determine whether an error will
	// be thrown as this will pre-emptively prevent subsequent calls which in turn
	// cannot be made.  In the context of KraftKit, the umbrella package will
	// simply skip its construction process and omit its further use.
	_, _, err := manager.handle(ctx)
	if err != nil {
		return nil, err
	}

	return &manager, nil
}

// handle is an internal method that returns the instantiated handler.Handler.
// This allows for lazy-loading the instantiation to the relevant client only
// when it is needed.
func (manager ociManager) handle(ctx context.Context) (context.Context, handler.Handler, error) {
	if contAddr := config.G[config.KraftKit](ctx).ContainerdAddr; len(contAddr) > 0 {
		namespace := defaultNamespace
		if n := os.Getenv("CONTAINERD_NAMESPACE"); n != "" {
			namespace = n
		}

		log.G(ctx).WithFields(logrus.Fields{
			"addr":      contAddr,
			"namespace": namespace,
		}).Debug("using containerd handler")

		ctx, handle, err := handler.NewContainerdHandler(ctx, contAddr, namespace)
		if err != nil {
			return nil, nil, err
		}

		return ctx, handle, nil
	}

	return nil, nil, fmt.Errorf("could not determine handler")
}

// Update implements packmanager.PackageManager
func (manager ociManager) Update(ctx context.Context) error {
	return nil
}

// Pack implements packmanager.PackageManager
func (manager ociManager) Pack(ctx context.Context, entity component.Component, opts ...packmanager.PackOption) ([]pack.Package, error) {
	targ, ok := entity.(target.Target)
	if !ok {
		return nil, fmt.Errorf("entity is not Unikraft target")
	}

	pkg, err := NewPackageFromTarget(ctx, targ, opts...)
	if err != nil {
		return nil, err
	}

	return []pack.Package{pkg}, nil
}

// Unpack implements packmanager.PackageManager
func (manager ociManager) Unpack(ctx context.Context, entity pack.Package, opts ...packmanager.UnpackOption) ([]component.Component, error) {
	return nil, fmt.Errorf("not implemented: oci.manager.Unpack")
}

// registry is a wrapper method for authenticating and listing OCI repositories
// from a provided domain representing a registry.
func registry(ctx context.Context, domain string) (*regtool.Registry, error) {
	var err error
	var auth types.AuthConfig
	insecure := false

	if a, ok := config.G[config.KraftKit](ctx).Auth[domain]; ok {
		log.G(ctx).
			WithField("registry", domain).
			Debug("authenticating")

		auth, err = repoutils.GetAuthConfig(a.User, a.Token, domain)
		if err != nil {
			return nil, err
		}

		if !a.VerifySSL {
			insecure = true
		}
	} else {
		auth, err = repoutils.GetAuthConfig("", "", domain)
		if err != nil {
			log.G(ctx).WithField("registry", domain).Warn(err)
		}
	}

	reg, err := regtool.New(ctx, auth, regtool.Opt{
		Domain:   domain,
		Insecure: insecure,
		Debug:    false,
		SkipPing: true,
		Headers: map[string]string{
			"User-Agent": version.UserAgent(),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("could not initialize registry: %v", err)
	}

	return reg, nil
}

// Catalog implements packmanager.PackageManager
func (manager ociManager) Catalog(ctx context.Context, qopts ...packmanager.QueryOption) ([]pack.Package, error) {
	var packs []pack.Package
	query := packmanager.NewQuery(qopts...)
	qname := query.Name()
	qversion := query.Version()

	// Adjust for the version being suffixed in a prototypical OCI reference
	// format
	ref, refErr := name.ParseReference(qname,
		name.WithDefaultRegistry(DefaultRegistry),
	)
	if refErr == nil {
		if ref.Identifier() != "latest" && qversion != "" && ref.Identifier() != qversion {
			return nil, fmt.Errorf("cannot determine which version as name contains version and version query paremeter set")
		} else if qversion == "" {
			qname = ref.Context().String()
			qversion = ref.Identifier()
		}
	}

	log.G(ctx).WithFields(query.Fields()).Debug("querying oci catalog")

	ctx, handle, err := manager.handle(ctx)
	if err != nil {
		return nil, err
	}

	if !query.UseCache() {
		// If a direct reference can be made, attempt to generate a package from it
		if refErr == nil {
			pack, err := NewPackageFromRemoteOCIRef(ctx, handle, ref.String())
			if err != nil {
				log.G(ctx).Warn(err)
			} else {
				packs = append(packs, pack)
			}
		}

		for _, domain := range manager.registries {
			log.G(ctx).
				WithField("registry", domain).
				Trace("querying")

			reg, err := registry(ctx, domain)
			if err != nil {
				log.G(ctx).
					WithField("registry", domain).
					Debugf("could not initialize registry: %v", err)
				continue
			}

			catalog, err := reg.Catalog(ctx, "")
			if err != nil {
				log.G(ctx).
					WithField("registry", domain).
					Debugf("could not query catalog: %v", err)
				continue
			}

			for _, fullref := range catalog {
				// Skip direct references from the remote registry
				if !query.UseCache() && refErr == nil && ref.String() == fullref {
					continue
				}

				if len(qname) > 0 && fullref != qname {
					continue
				}

				raw, err := crane.Manifest(fullref)
				if err != nil {
					continue
				}

				var manifest ocispec.Manifest

				if err := json.Unmarshal(raw, &manifest); err != nil {
					continue
				}

				pack, err := NewPackageFromOCIManifestSpec(
					ctx,
					handle,
					fullref,
					manifest,
				)
				if err != nil {
					continue
				}

				packs = append(packs, pack)
			}
		}
	}

	// Access local images that are available on the host
	manifests, err := handle.ListManifests(ctx)
	if err != nil {
		return nil, err
	}

	for _, manifest := range manifests {
		// Check if the OCI image has a known annotation which identifies if a
		// unikernel is contained within
		if _, ok := manifest.Annotations[AnnotationKernelVersion]; !ok {
			log.G(ctx).
				WithField("ref", manifest.Config.Digest.String()).
				Trace("skipping non-unikernel digest")
			continue
		}

		// Could not determine name from manifest specification
		refname, ok := manifest.Annotations[ocispec.AnnotationRefName]
		if !ok {
			log.G(ctx).
				WithField("ref", manifest.Config.Digest.String()).
				Trace("skipping non-unikernel digest")
			continue
		}

		// Skip if querying for the name and the name does not match
		if len(qname) > 0 && refname != qname {
			log.G(ctx).
				WithField("ref", manifest.Config.Digest.String()).
				Trace("skipping non-unikernel digest")
			continue
		}

		// Could not determine name from manifest specification
		revision, ok := manifest.Annotations[ocispec.AnnotationRevision]
		if !ok {
			log.G(ctx).
				WithField("ref", manifest.Config.Digest.String()).
				Trace("skipping non-unikernel digest")
			continue
		}

		fullref := fmt.Sprintf("%s:%s", refname, revision)

		// Skip direct references from the remote registry
		if !query.UseCache() && refErr == nil && ref.String() == fullref {
			log.G(ctx).
				WithField("ref", manifest.Config.Digest.String()).
				Trace("skipping non-unikernel digest")
			continue
		}

		// Skip if querying for a version and the version does not match
		if len(qversion) > 0 && revision != qversion {
			log.G(ctx).
				WithField("ref", manifest.Config.Digest.String()).
				Trace("skipping non-unikernel digest")
			continue
		}

		log.G(ctx).WithField("ref", fullref).Debug("found")

		pack, err := NewPackageFromOCIManifestSpec(
			ctx,
			handle,
			fullref,
			manifest,
		)
		if err != nil {
			// log.G(ctx).Warn(err)
			continue
		}

		packs = append(packs, pack)
	}

	return packs, nil
}

// AddSource implements packmanager.PackageManager
func (manager ociManager) AddSource(ctx context.Context, source string) error {
	for _, manifest := range config.G[config.KraftKit](ctx).Unikraft.Manifests {
		if source == manifest {
			log.G(ctx).Warnf("manifest already saved: %s", source)
			return nil
		}
	}

	log.G(ctx).Infof("adding to list of manifests: %s", source)
	config.G[config.KraftKit](ctx).Unikraft.Manifests = append(
		config.G[config.KraftKit](ctx).Unikraft.Manifests,
		source,
	)
	return config.M[config.KraftKit](ctx).Write(true)
}

// RemoveSource implements packmanager.PackageManager
func (manager ociManager) RemoveSource(ctx context.Context, source string) error {
	manifests := []string{}

	for _, manifest := range config.G[config.KraftKit](ctx).Unikraft.Manifests {
		if source != manifest {
			manifests = append(manifests, manifest)
		}
	}

	log.G(ctx).Infof("removing from list of manifests: %s", source)
	config.G[config.KraftKit](ctx).Unikraft.Manifests = manifests
	return config.M[config.KraftKit](ctx).Write(false)
}

// IsCompatible implements packmanager.PackageManager
func (manager ociManager) IsCompatible(ctx context.Context, source string, qopts ...packmanager.QueryOption) (packmanager.PackageManager, bool, error) {
	log.G(ctx).
		WithField("source", source).
		Debug("checking if source is an oci unikernel")

	// 1. Check if the source is a reference to a local image

	ref, err := name.ParseReference(source,
		name.WithDefaultRegistry(defaultRegistry),
	)
	if err != nil {
		return nil, false, err
	}

	ctx, handle, err := manager.handle(ctx)
	if err != nil {
		return nil, false, err
	}

	_, err = handle.ResolveImage(ctx, source)
	if err == nil {
		return manager, true, nil
	}

	log.G(ctx).Debugf("could not resolve local image: %v", err)

	// 2. Check if the source is a remote registry

	if uri, err := url.Parse(source); err == nil && uri.Host == source {
		if reg, err := registry(ctx, source); err == nil && reg.Ping(ctx) == nil {
			return manager, true, nil
		}
	}

	log.G(ctx).Debugf("source is not a registry: %v", err)

	// 3. Check if the source is a full reference tag at a remote registry

	opts := []crane.Option{
		crane.WithContext(ctx),
		crane.WithUserAgent(version.UserAgent()),
	}

	if auth, ok := config.G[config.KraftKit](ctx).Auth[ref.Context().RegistryStr()]; ok {
		// We split up the options for authenticating and the option for "verifying
		// ssl" such that a user can simply disable secure connection to a registry
		// which is publically accessible.

		if auth.User != "" && auth.Token != "" {
			log.G(ctx).
				WithField("registry", ref.Context().RegistryStr()).
				Debug("authenticating")

			opts = append(opts,
				crane.WithAuth(authn.FromConfig(authn.AuthConfig{
					Username: auth.User,
					Password: auth.Token,
				})),
			)
		}

		if !auth.VerifySSL {
			rt := http.DefaultTransport.(*http.Transport).Clone()
			rt.TLSClientConfig = &tls.Config{
				InsecureSkipVerify: true,
			}
			opts = append(opts,
				crane.Insecure,
				crane.WithTransport(rt),
			)
		}
	}

	raw, err := crane.Config(source, opts...)
	if err == nil && len(raw) > 0 {
		return manager, true, nil
	}

	log.G(ctx).Warnf("could not resolve remote image: %v", err)

	return nil, false, nil
}

// From implements packmanager.PackageManager
func (manager ociManager) From(pack.PackageFormat) (packmanager.PackageManager, error) {
	return nil, fmt.Errorf("not possible: oci.manager.From")
}

// Format implements packmanager.PackageManager
func (manager ociManager) Format() pack.PackageFormat {
	return OCIFormat
}
