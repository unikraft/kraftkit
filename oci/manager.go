// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package oci

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"

	"github.com/gobwas/glob"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content"

	"kraftkit.sh/config"
	"kraftkit.sh/internal/set"
	"kraftkit.sh/internal/version"
	"kraftkit.sh/log"
	"kraftkit.sh/oci/cache"
	"kraftkit.sh/oci/handler"
	"kraftkit.sh/oci/simpleauth"
	"kraftkit.sh/pack"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/component"
	"kraftkit.sh/unikraft/target"
)

type OCIManager struct {
	registries []string
	auths      map[string]config.AuthConfig
	handle     func(ctx context.Context) (context.Context, handler.Handler, error)
}

const OCIFormat pack.PackageFormat = "oci"

// NewPackageManager satisfies the `packmanager.NewPackageManager` interface
// and returns a new `packmanager.PackageManager` for the manifest manager.
func NewPackageManager(ctx context.Context, opts ...any) (packmanager.PackageManager, error) {
	mopts := make([]OCIManagerOption, 0)
	for _, opt := range opts {
		if o, ok := opt.(OCIManagerOption); ok {
			mopts = append(mopts, o)
		}
	}

	return NewOCIManager(ctx, mopts...)
}

// NewOCIManager instantiates a new package manager based on OCI archives.
func NewOCIManager(ctx context.Context, opts ...OCIManagerOption) (*OCIManager, error) {
	manager := OCIManager{}

	for _, opt := range opts {
		if err := opt(ctx, &manager); err != nil {
			return nil, err
		}
	}

	if manager.handle == nil {
		return nil, fmt.Errorf("cannot instantiate OCI Manager without handler")
	}

	return &manager, nil
}

// Update implements packmanager.PackageManager
func (manager *OCIManager) Update(ctx context.Context) error {
	indexes, packs, err := manager.update(ctx, nil, nil)
	if err != nil {
		return err
	}

	for ref, pack := range packs {
		pack := pack.(*ociPackage) // Safe since we're in the oci package

		log.G(ctx).Debugf("saving %s", pack.String())

		for _, manifest := range pack.index.manifests {
			if _, err := manifest.Save(ctx, ref, nil); err != nil {
				return fmt.Errorf("could not save manifest: %w", err)
			}
		}
	}

	ctx, handle, err := manager.handle(ctx)
	if err != nil {
		return err
	}

	for ref, index := range indexes {
		indexJson, err := json.MarshalIndent(index, "", "  ")
		if err != nil {
			return fmt.Errorf("could not marshal index: %w", err)
		}
		indexJson = append(indexJson, '\n')

		if err := handle.SaveDescriptor(ctx, ref,
			content.NewDescriptorFromBytes(ocispec.MediaTypeImageIndex, indexJson),
			bytes.NewReader(indexJson),
			nil,
		); err != nil {
			return fmt.Errorf("could not save index: %w", err)
		}
	}

	return nil
}

func (manager *OCIManager) update(ctx context.Context, auths map[string]config.AuthConfig, query *packmanager.Query) (map[string]ocispec.Index, map[string]pack.Package, error) {
	ctx, handle, err := manager.handle(ctx)
	if err != nil {
		return nil, nil, err
	}

	if auths == nil {
		auths = config.G[config.KraftKit](ctx).Auth
	}

	packs := make(map[string]pack.Package)
	indexes := make(map[string]ocispec.Index)

	for _, domain := range manager.registries {
		log.G(ctx).
			WithField("registry", domain).
			Trace("querying")

		nopts := []name.Option{}
		authConfig := &authn.AuthConfig{}
		transport := http.DefaultTransport.(*http.Transport).Clone()

		// Annoyingly convert between regtypes and authn.
		if auth, ok := auths[domain]; ok {
			authConfig.Username = auth.User
			authConfig.Password = auth.Token

			if !auth.VerifySSL {
				transport.TLSClientConfig = &tls.Config{
					InsecureSkipVerify: true,
				}
				nopts = append(nopts, name.Insecure)
			}
		}

		regName, err := name.NewRegistry(domain, nopts...)
		if err != nil {
			log.G(ctx).
				WithField("registry", domain).
				Debugf("could not parse registry: %v", err)
			continue
		}

		catalog, err := remote.Catalog(ctx, regName,
			remote.WithContext(ctx),
			remote.WithAuth(&simpleauth.SimpleAuthenticator{
				Auth: authConfig,
			}),
			remote.WithTransport(transport),
		)
		if err != nil {
			log.G(ctx).
				WithField("registry", domain).
				Debugf("could not query catalog: %v", err)
			continue
		}

		var wg sync.WaitGroup
		wg.Add(len(catalog))
		var mu sync.RWMutex

		for _, fullref := range catalog {
			go func(fullref string) {
				defer wg.Done()

				ref, err := name.ParseReference(fullref,
					name.WithDefaultRegistry(domain),
					name.WithDefaultTag(DefaultTag),
				)
				if err != nil {
					log.G(ctx).
						WithField("ref", fullref).
						Tracef("skipping index: could not parse reference: %s", err.Error())
					return
				}

				index, err := cache.RemoteIndex(ref,
					remote.WithContext(ctx),
					remote.WithAuth(&simpleauth.SimpleAuthenticator{
						Auth: authConfig,
					}),
					remote.WithTransport(transport),
				)
				if err != nil {
					log.G(ctx).
						WithField("ref", fullref).
						Tracef("skipping index: could not retrieve image: %s", err.Error())
					return
				}

				v1IndexRaw, err := index.RawManifest()
				if err != nil {
					log.G(ctx).
						WithField("ref", fullref).
						Tracef("could not access the raw index: %s", err.Error())
					return
				}

				var ociIndex ocispec.Index
				if err := json.Unmarshal(v1IndexRaw, &ociIndex); err != nil {
					log.G(ctx).
						WithField("ref", fullref).
						Tracef("could not unmarshal index: %s", err.Error())
					return
				}

				mu.Lock()
				indexes[fullref] = ociIndex
				mu.Unlock()

				v1IndexManifest, err := index.IndexManifest()
				if err != nil {
					log.G(ctx).
						WithField("ref", fullref).
						Tracef("could not access the index's manifest object: %s", err.Error())
					return
				}

				v1ManifestPackages := processV1IndexManifests(ctx,
					handle,
					fullref,
					query,
					FromGoogleV1DescriptorToOCISpec(v1IndexManifest.Manifests...),
				)

				mu.Lock()
				for checksum, pack := range v1ManifestPackages {
					var title []string
					for _, column := range pack.Columns() {
						if len(column.Value) > 12 {
							continue
						}

						title = append(title, column.Value)
					}

					log.G(ctx).
						Tracef("found %s (%s)", pack.String(), strings.Join(title, ", "))
					packs[checksum] = pack
				}
				mu.Unlock()
			}(fullref)
		}

		wg.Wait()
	}

	return indexes, packs, nil
}

// Pack implements packmanager.PackageManager
func (manager *OCIManager) Pack(ctx context.Context, entity component.Component, opts ...packmanager.PackOption) ([]pack.Package, error) {
	ctx, handle, err := manager.handle(ctx)
	if err != nil {
		return nil, err
	}

	var pkg pack.Package
	if targ, ok := entity.(target.Target); ok {
		pkg, err = NewPackageFromTarget(ctx, handle, targ, opts...)
	} else {
		pkg, err = NewPackage(ctx, handle, opts...)
	}
	if err != nil {
		return nil, err
	}

	return []pack.Package{pkg}, nil
}

// Unpack implements packmanager.PackageManager
func (manager *OCIManager) Unpack(ctx context.Context, entity pack.Package, opts ...packmanager.UnpackOption) ([]component.Component, error) {
	return nil, fmt.Errorf("not implemented: oci.manager.Unpack")
}

// processV1IndexManifests is an internal utility method which is able to
// iterate over the supplied slice of ocispec.Descriptors which represent a
// Manifest from an Index.  Based on the provided criterium from the query,
// identify the Descriptor that is compatible and instantiate a pack.Package
// structure from it.
func processV1IndexManifests(ctx context.Context, handle handler.Handler, fullref string, query *packmanager.Query, manifests []ocispec.Descriptor) map[string]pack.Package {
	packs := make(map[string]pack.Package)
	var wg sync.WaitGroup
	wg.Add(len(manifests))
	var mu sync.RWMutex

	for _, descriptor := range manifests {
		go func(descriptor ocispec.Descriptor) {
			defer wg.Done()
			if ok, err := IsOCIDescriptorKraftKitCompatible(&descriptor); !ok {
				log.G(ctx).
					WithField("digest", descriptor.Digest.String()).
					WithField("ref", fullref).
					Tracef("incompatible index structure: %s", err.Error())
				return
			}

			if query != nil && query.Platform() != "" && query.Platform() != descriptor.Platform.OS {
				log.G(ctx).
					WithField("ref", fullref).
					WithField("digest", descriptor.Digest.String()).
					WithField("want", query.Platform()).
					WithField("got", descriptor.Platform.OS).
					Trace("skipping manifest: platform does not match query")
				return
			}

			if query != nil && query.Architecture() != "" && query.Architecture() != descriptor.Platform.Architecture {
				log.G(ctx).
					WithField("ref", fullref).
					WithField("digest", descriptor.Digest.String()).
					WithField("want", query.Architecture()).
					WithField("got", descriptor.Platform.Architecture).
					Trace("skipping manifest: architecture does not match query")
				return
			}

			if query != nil && len(query.KConfig()) > 0 {
				// If the list of requested features is greater than the list of
				// available features, there will be no way for the two to match.  We
				// are searching for a subset of query.KConfig() from
				// m.Platform.OSFeatures to match.
				if len(query.KConfig()) > len(descriptor.Platform.OSFeatures) {
					log.G(ctx).
						WithField("ref", fullref).
						WithField("digest", descriptor.Digest.String()).
						Trace("skipping descriptor: query contains more features than available")
					return
				}

				available := set.NewStringSet(descriptor.Platform.OSFeatures...)

				// Iterate through the query's requested set of features and skip only
				// if the descriptor does not contain the requested KConfig feature.
				for _, a := range query.KConfig() {
					if !available.Contains(a) {
						log.G(ctx).
							WithField("ref", fullref).
							WithField("digest", descriptor.Digest.String()).
							WithField("feature", a).
							Trace("skipping manifest: missing feature")
						return
					}
				}
			}

			var auths map[string]config.AuthConfig
			if query != nil {
				auths = query.Auths()
			}

			// If we have made it this far, the query has been successfully
			// satisfied by this particular manifest and we can generate a package
			// from it.
			pack, err := NewPackageFromOCIManifestDigest(ctx,
				handle,
				fullref,
				auths,
				descriptor.Digest,
			)
			if err != nil {
				log.G(ctx).
					WithField("ref", fullref).
					WithField("digest", descriptor.Digest.String()).
					Tracef("skipping manifest: could not instantiate package from manifest digest: %s", err.Error())
				return
			}

			mu.Lock()
			packs[descriptor.Digest.String()] = pack
			mu.Unlock()
		}(descriptor)
	}

	wg.Wait()

	return packs
}

// Catalog implements packmanager.PackageManager
func (manager *OCIManager) Catalog(ctx context.Context, qopts ...packmanager.QueryOption) ([]pack.Package, error) {
	query := packmanager.NewQuery(qopts...)

	// Do not perform a search if a query for a specific type is requested and it
	// does not include the application-type.
	if len(query.Types()) > 0 && !slices.Contains(query.Types(), unikraft.ComponentTypeApp) {
		return nil, nil
	}

	var qglob glob.Glob
	var err error
	packs := make(map[string]pack.Package)
	qname := query.Name()
	total := 0

	if strings.ContainsRune(qname, '*') {
		qglob, err = glob.Compile(qname)
		if err != nil {
			return nil, fmt.Errorf("query name is not glob-able: %w", err)
		}
	} else if !strings.ContainsRune(qname, ':') && len(query.Version()) > 0 {
		if strings.Contains(query.Version(), ":") {
			qname = fmt.Sprintf("%s@%s", qname, query.Version())
		} else {
			qname = fmt.Sprintf("%s:%s", qname, query.Version())
		}
	}

	qversion := query.Version()
	// Adjust for the version being suffixed in a prototypical OCI reference
	// format.
	ref, refErr := name.ParseReference(qname,
		name.WithDefaultRegistry(""),
		name.WithDefaultTag(DefaultTag),
	)
	if refErr == nil {
		qname = ref.Context().Name()
		if ref.Identifier() != "latest" && qversion != "" && ref.Identifier() != qversion {
			return nil, fmt.Errorf("cannot determine which version as name contains version and version query paremeter set")
		} else if qversion == "" {
			qversion = ref.Identifier()
		}
	}

	unsetRegistry := false

	// No default registry found, re-parse with
	if ref != nil && ref.Context().RegistryStr() == "" {
		unsetRegistry = true
		var formatRef string
		if strings.Contains(qversion, ":") {
			formatRef = fmt.Sprintf("%s@%s", qname, qversion)
		} else {
			formatRef = fmt.Sprintf("%s:%s", qname, qversion)
		}
		ref, refErr = name.ParseReference(formatRef,
			name.WithDefaultRegistry(DefaultRegistry),
			name.WithDefaultTag(DefaultTag),
		)
	}

	log.G(ctx).
		WithFields(query.Fields()).
		Debug("querying catalog")

	ctx, handle, err := manager.handle(ctx)
	if err != nil {
		return nil, err
	}

	var auths map[string]config.AuthConfig
	if query.Auths() == nil {
		auths = config.G[config.KraftKit](ctx).Auth
	} else {
		auths = query.Auths()
	}

	descriptors := make(map[string][]ocispec.Descriptor)

	// If a direct reference can be made, attempt to generate a package from it.
	if query.Remote() && refErr == nil && !unsetRegistry {
		authConfig := &authn.AuthConfig{}

		ropts := []remote.Option{
			remote.WithContext(ctx),
		}

		// Annoyingly convert between regtypes and authn.
		if auth, ok := auths[ref.Context().RegistryStr()]; ok {
			authConfig.Username = auth.User
			authConfig.Password = auth.Token

			if !auth.VerifySSL {
				rt := http.DefaultTransport.(*http.Transport).Clone()
				rt.TLSClientConfig = &tls.Config{
					InsecureSkipVerify: true,
				}
				ropts = append(ropts,
					remote.WithTransport(rt),
				)
			}

			ropts = append(ropts,
				remote.WithAuth(&simpleauth.SimpleAuthenticator{
					Auth: authConfig,
				}),
			)
		}

		log.G(ctx).
			WithField("ref", ref.Name()).
			Trace("getting remote index")

		v1ImageIndex, err := cache.RemoteIndex(ref, ropts...)
		if err != nil {
			log.G(ctx).
				Debugf("could not get index: %v", err)

			// Trying manifest instead of index
			v1ImageManifest, err := cache.RemoteImage(ref, ropts...)
			if err != nil {
				log.G(ctx).
					WithField("ref", ref).
					Debugf("could not retrieve image manifest: %s", err.Error())
				goto resolveLocalIndex
			}

			manifest, _ := v1ImageManifest.Manifest()
			dgst, _ := v1ImageManifest.Digest()

			descriptors[ref.String()] = append(descriptors[ref.String()], []ocispec.Descriptor{
				{
					MediaType: string(manifest.MediaType),
					Digest:    digest.Digest(dgst.String()),
					Platform: &ocispec.Platform{
						Architecture: manifest.Config.Platform.Architecture,
						OS:           manifest.Config.Platform.OS,
						OSVersion:    manifest.Config.Platform.OSVersion,
						OSFeatures:   manifest.Config.Platform.OSFeatures,
					},
					Annotations: manifest.Annotations,
				},
			}...)

			goto searchLocalIndexes
		}

		v1IndexManifest, err := v1ImageIndex.IndexManifest()
		if err != nil {
			log.G(ctx).
				WithField("ref", ref).
				Tracef("could not access the index's manifest object: %s", err.Error())
			goto resolveLocalIndex
		}

		descriptors[ref.String()] = append(descriptors[ref.String()],
			FromGoogleV1DescriptorToOCISpec(v1IndexManifest.Manifests...)...,
		)

		// No need to search remote indexes by registry if the registry has been
		// included as part of the ref.
		goto searchLocalIndexes
	}

	if query.Remote() {
		_, more, err := manager.update(ctx, auths, query)
		if err != nil {
			log.G(ctx).
				Debugf("could not update: %v", err)
		} else {
			for checksum, pack := range more {
				total++

				var formattedRef string
				if strings.HasPrefix(pack.Version(), "sha256:") {
					formattedRef = fmt.Sprintf("%s@%s", pack.Name(), pack.Version())
				} else {
					formattedRef = fmt.Sprintf("%s:%s", pack.Name(), pack.Version())
				}

				ref, err := name.ParseReference(formattedRef)
				if err != nil {
					log.G(ctx).
						WithField("ref", pack.Name()).
						Tracef("skipping index: could not parse reference: %s", err.Error())
					continue
				}

				fullref := fmt.Sprintf("%s:%s", ref.Context().RepositoryStr(), ref.Identifier())

				// If the query did specify a registry include this in check otherwise
				// search for indexes without this as prefix.
				if !unsetRegistry {
					fullref = fmt.Sprintf("%s/%s", ref.Context().RegistryStr(), fullref)
				}

				if qglob != nil && !qglob.Match(fullref) {
					log.G(ctx).
						WithField("want", qname).
						WithField("got", fullref).
						Trace("skipping manifest: glob does not match")
					continue
				} else if qglob == nil {
					if len(qversion) > 0 && len(qname) > 0 {
						var formattedRef string
						if strings.HasPrefix(query.Version(), "sha256:") {
							formattedRef = fmt.Sprintf("%s@%s", qname, qversion)
						} else {
							formattedRef = fmt.Sprintf("%s:%s", qname, qversion)
						}

						if fullref != formattedRef {
							log.G(ctx).
								WithField("want", formattedRef).
								WithField("got", fullref).
								Trace("skipping manifest: name does not match")
							continue
						}
					} else if len(qname) > 0 && fullref != qname {
						log.G(ctx).
							WithField("want", qname).
							WithField("got", fullref).
							Trace("skipping manifest: name does not match")
						continue
					}
				}
				log.G(ctx).
					WithField("ref", pack.ID()).
					WithField("via", "remote").
					Trace("found")
				packs[checksum] = pack
			}
		}
	}

resolveLocalIndex:
	// If the query is local and the reference is a fully qualified OCI reference,
	// attempt to resolve the exact index and generate packages from it.
	if query.Local() && len(qversion) > 0 && len(qname) > 0 {
		var oref string
		if strings.Contains(qversion, ":") {
			oref = fmt.Sprintf("%s@%s", qname, qversion)
		} else {
			oref = fmt.Sprintf("%s:%s", qname, qversion)
		}

		// First check if the oref refers to an index.
		index, _, err := handle.ResolveIndex(ctx, oref)
		if err != nil {
			log.G(ctx).
				WithField("ref", oref).
				Trace("could not resolve exact index")

			// The oref did not refer to an index. Maybe it refers to a manifest.
			// This is only possible if the qversion is an actual digest.
			if strings.Contains(qversion, ":") {
				dgst, err := digest.Parse(qversion)
				if err != nil {
					goto searchLocalIndexes
				}

				manifest, _, err := handle.ResolveManifest(ctx, oref, dgst)
				if err != nil {
					goto searchLocalIndexes
				}

				descriptors[ref.String()] = append(descriptors[oref], []ocispec.Descriptor{
					{
						MediaType:   manifest.MediaType,
						Digest:      dgst,
						Platform:    manifest.Config.Platform,
						Annotations: manifest.Annotations,
					},
				}...)
			} else {
				goto searchLocalIndexes
			}
		}
		if index != nil {
			descriptors[ref.String()] = append(descriptors[oref], index.Manifests...)
		}

		// If the register was set, then an exact local index lookup was expected so
		// we can return here.
		if !unsetRegistry {
			goto returnPacks
		}
	}

searchLocalIndexes:
	if query.Local() {
		// Access local indexes that are available on the host
		indexes, err := handle.ListIndexes(ctx)
		if err != nil {
			return nil, err
		}

		for oref, index := range indexes {
			ref, err := name.ParseReference(oref,
				name.WithDefaultRegistry(""),
				name.WithDefaultTag(DefaultTag),
			)
			if err != nil {
				log.G(ctx).
					WithField("ref", oref).
					Tracef("skipping index: invalid reference format: %s", err.Error())
				total += len(index.Manifests)
				continue
			}

			var fullref string
			if strings.ContainsRune(qversion, ':') {
				_, dgst, _ := handle.ResolveIndex(ctx, oref)
				fullref = fmt.Sprintf("%s@%s", ref.Context().RepositoryStr(), dgst)
			} else {
				fullref = fmt.Sprintf("%s:%s", ref.Context().RepositoryStr(), ref.Identifier())
			}

			// If the query did specify a registry include this in check otherwise
			// search for indexes without this as prefix.
			if !unsetRegistry {
				fullref = fmt.Sprintf("%s/%s", ref.Context().RegistryStr(), fullref)
			}

			if ok, err := IsOCIIndexKraftKitCompatible(index); !ok {
				log.G(ctx).
					WithField("ref", fullref).
					Tracef("skipping index: incompatible index structure: %s", err.Error())
				total += len(index.Manifests)
				continue
			}

			if qglob != nil && !qglob.Match(fullref) {
				log.G(ctx).
					WithField("want", qname).
					WithField("got", fullref).
					Trace("skipping index: glob does not match")
				total += len(index.Manifests)
				continue
			} else if qglob == nil {
				if len(qversion) > 0 && len(qname) > 0 {
					var formattedRef string
					if strings.ContainsRune(qversion, ':') {
						formattedRef = fmt.Sprintf("%s@%s", qname, qversion)
					} else {
						formattedRef = fmt.Sprintf("%s:%s", qname, qversion)
					}
					if fullref != formattedRef {
						log.G(ctx).
							WithField("want", formattedRef).
							WithField("got", fullref).
							Trace("skipping index: name does not match")
						total += len(index.Manifests)
						continue
					}
				} else if len(qname) > 0 && fullref != qname {
					log.G(ctx).
						WithField("want", qname).
						WithField("got", fullref).
						Trace("skipping index: name does not match")
					total += len(index.Manifests)
					continue
				}
			}

			descriptors[ref.String()] = append(descriptors[oref], index.Manifests...)
		}
	}

returnPacks:
	var wg sync.WaitGroup
	var mu sync.Mutex
	wg.Add(len(descriptors))
	for oref, descs := range descriptors {
		go func(total *int, packs map[string]pack.Package) {
			defer wg.Done()
			for checksum, pack := range processV1IndexManifests(ctx,
				handle,
				oref,
				query,
				descs,
			) {
				log.G(ctx).
					WithField("ref", pack.ID()).
					Trace("found")
				mu.Lock()
				packs[checksum] = pack
				mu.Unlock()
				*total++
			}
		}(&total, packs)
	}

	wg.Wait()

	var ret []pack.Package

	for _, pack := range packs {
		var title []string
		for _, column := range pack.Columns() {
			if len(column.Value) > 12 {
				continue
			}

			title = append(title, column.Value)
		}

		log.G(ctx).
			Debugf("found %s (%s)", pack.String(), strings.Join(title, ", "))

		ret = append(ret, pack)
	}

	log.G(ctx).Debugf("found %d/%d matching packages in oci catalog", len(packs), total)

	return ret, nil
}

// SetSources implements packmanager.PackageManager
func (manager *OCIManager) SetSources(_ context.Context, sources ...string) error {
	manager.registries = sources
	return nil
}

// AddSource implements packmanager.PackageManager
func (manager *OCIManager) AddSource(ctx context.Context, source string) error {
	if manager.registries == nil {
		manager.registries = make([]string, 0)
	}

	manager.registries = append(manager.registries, source)

	return nil
}

// Delete implements packmanager.PackageManager.
func (manager *OCIManager) Delete(ctx context.Context, qopts ...packmanager.QueryOption) error {
	packs, err := manager.Catalog(ctx, qopts...)
	if err != nil {
		return err
	}

	var errs []error

	for _, pack := range packs {
		if err := pack.Delete(ctx); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

// Purge implements packmanager.PackageManager.
func (manager *OCIManager) Purge(ctx context.Context) error {
	ctx, handler, err := manager.handle(ctx)
	if err != nil {
		return fmt.Errorf("could not initialize handler: %w", err)
	}

	indexes, err := handler.ListIndexes(ctx)
	if err != nil {
		return fmt.Errorf("could not list indexes: %w", err)
	}

	for ref := range indexes {
		log.G(ctx).
			WithField("ref", ref).
			Trace("deleting")

		if err := handler.DeleteIndex(ctx, ref, true); err != nil {
			return fmt.Errorf("could not delete index %s: %w", ref, err)
		}
	}

	dgsts, err := handler.ListDigests(ctx)
	if err != nil {
		return fmt.Errorf("could not list digests: %w", err)
	}

	for _, dgst := range dgsts {
		log.G(ctx).
			WithField("digest", dgst.String()).
			Trace("deleting")

		if err := handler.DeleteDigest(ctx, dgst); err != nil {
			return fmt.Errorf("could not delete digest %s: %w", dgst, err)
		}
	}

	return nil
}

// RemoveSource implements packmanager.PackageManager
func (manager *OCIManager) RemoveSource(ctx context.Context, source string) error {
	for i, needle := range manager.registries {
		if needle == source {
			ret := make([]string, 0)
			ret = append(ret, manager.registries[:i]...)
			manager.registries = append(ret, manager.registries[i+1:]...)
			break
		}
	}

	return nil
}

// IsCompatible implements packmanager.PackageManager
func (manager *OCIManager) IsCompatible(ctx context.Context, source string, qopts ...packmanager.QueryOption) (packmanager.PackageManager, bool, error) {
	ctx, handle, err := manager.handle(ctx)
	if err != nil {
		return nil, false, err
	}

	isFullyQualifiedNameReference := func(source string) bool {
		_, err := name.ParseReference(source)
		if err != nil && errors.Is(err, &name.ErrBadName{}) {
			return false
		}

		return true
	}

	query := packmanager.NewQuery(qopts...)

	// Check if the provided source is a fully qualified OCI reference
	isLocalImage := func(source string) bool {
		// First try without known registries
		if _, _, err := handle.ResolveIndex(ctx, source); err == nil {
			return true
		}

		// Now try with known registries
		for _, registry := range manager.registries {
			ref, err := name.ParseReference(source,
				name.WithDefaultRegistry(registry),
				name.WithDefaultTag(DefaultTag),
			)
			if err != nil {
				continue
			}

			if _, _, err := handle.ResolveIndex(ctx, ref.Context().String()); err == nil {
				return true
			}
		}

		return false
	}

	// Check if the provided source an OCI Distrubtion Spec capable registry
	isRegistry := func(source string) bool {
		log.G(ctx).
			WithField("source", source).
			Tracef("checking if source is registry")

		regName, err := name.NewRegistry(source)
		if err != nil {
			return false
		}

		if _, err := transport.Ping(ctx, regName, http.DefaultTransport.(*http.Transport).Clone()); err == nil {
			return true
		}

		return false
	}

	// Check if the provided source is OCI registry
	isRemoteImage := func(source string) bool {
		log.G(ctx).
			WithField("source", source).
			Tracef("checking if source is remote image")

		ref, err := name.ParseReference(source,
			name.WithDefaultRegistry(DefaultRegistry),
			name.WithDefaultTag(DefaultTag),
		)
		if err != nil {
			return false
		}

		source = fmt.Sprintf("%s:%s", ref.Context().String(), ref.Identifier())

		// log.G(ctx).WithField("source", source).Debug("checking if source is registry")
		opts := []crane.Option{
			crane.WithContext(ctx),
			crane.WithUserAgent(version.UserAgent()),
			crane.WithPlatform(&v1.Platform{
				OS:           query.Platform(),
				OSFeatures:   query.KConfig(),
				Architecture: query.Architecture(),
			}),
		}

		if auth, ok := config.G[config.KraftKit](ctx).Auth[ref.Context().Registry.RegistryStr()]; ok {
			// We split up the options for authenticating and the option for
			// "verifying ssl" such that a user can simply disable secure connection
			// to a registry if desired.

			if auth.User != "" && auth.Token != "" {
				log.G(ctx).
					WithField("registry", source).
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

		desc, err := crane.Head(source, opts...)
		if err == nil && desc != nil {
			return true
		}

		log.G(ctx).WithField("source", source).Trace(err)

		return false
	}

	checks := []func(string) bool{
		isLocalImage,
		isFullyQualifiedNameReference,
	}

	if query.Remote() {
		checks = append(checks,
			isRegistry,
			isRemoteImage,
		)
	}

	for _, check := range checks {
		if check(source) {
			return manager, true, nil
		}
	}

	return nil, false, nil
}

// From implements packmanager.PackageManager
func (manager *OCIManager) From(pack.PackageFormat) (packmanager.PackageManager, error) {
	return nil, fmt.Errorf("not possible: oci.manager.From")
}

// Format implements packmanager.PackageManager
func (manager *OCIManager) Format() pack.PackageFormat {
	return OCIFormat
}
