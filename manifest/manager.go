// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package manifest

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/Masterminds/semver"
	"github.com/gobwas/glob"
	"github.com/sirupsen/logrus"

	"kraftkit.sh/config"
	"kraftkit.sh/log"
	"kraftkit.sh/pack"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/component"
)

type ManifestManager struct {
	manifests          []string
	indexCache         *ManifestIndex
	localManifestDir   string
	auths              map[string]config.AuthConfig
	defaultChannelName string
	cacheDir           string
}

// NewPackageManager satisfies the `packmanager.NewPackageManager` interface
// and returns a new `packmanager.PackageManager` for the manifest manager.
func NewPackageManager(ctx context.Context, opts ...any) (packmanager.PackageManager, error) {
	mopts := make([]ManifestManagerOption, 0)
	for _, opt := range opts {
		if o, ok := opt.(ManifestManagerOption); ok {
			mopts = append(mopts, o)
		}
	}

	return NewManifestManager(ctx, mopts...)
}

// NewManifestManager instantiates a new package manager which manipulates
// Unikraft manifests.
func NewManifestManager(ctx context.Context, opts ...ManifestManagerOption) (*ManifestManager, error) {
	manager := ManifestManager{}

	for _, opt := range opts {
		if err := opt(ctx, &manager); err != nil {
			return nil, err
		}
	}

	if len(manager.auths) == 0 {
		manager.auths = config.G[config.KraftKit](ctx).Auth
	}

	if len(manager.cacheDir) == 0 {
		manager.cacheDir = config.G[config.KraftKit](ctx).Paths.Sources
	}

	if len(manager.manifests) == 0 {
		if len(config.G[config.KraftKit](ctx).Unikraft.Manifests) == 0 {
			manager.manifests = []string{config.DefaultManifestIndex}
		} else {
			manager.manifests = config.G[config.KraftKit](ctx).Unikraft.Manifests
		}
	}

	if manager.defaultChannelName == "" {
		manager.defaultChannelName = DefaultChannelName
	}

	return &manager, nil
}

// Index retrieves and returns a cache of the upstream manifest registry
func (m *ManifestManager) Index(ctx context.Context) (*ManifestIndex, error) {
	index := &ManifestIndex{
		LastUpdated: time.Now(),
	}

	mopts := []ManifestOption{
		WithAuthConfig(m.auths),
		WithCacheDir(m.cacheDir),
		WithUpdate(true),
		WithDefaultChannelName(m.defaultChannelName),
	}

	all := map[string]*Manifest{}

	for _, manipath := range m.manifests {
		// If the path of the manipath is the same as the current manifest or it
		// resides in the same directory as KraftKit's configured path for manifests
		// then we can skip this since we don't want to update ourselves.
		// if manipath == m.LocalManifestIndex() || filepath.Dir(manipath) == m.LocalManifestsDir() {
		// 	m.opts.Log.Debugf("skipping: %s", manipath)
		// 	continue
		// }

		log.G(ctx).WithFields(logrus.Fields{
			"manifest": manipath,
		}).Debug("fetching")

		manifests, err := FindManifestsFromSource(ctx, manipath, mopts...)
		if err != nil {
			log.G(ctx).Warnf("%s", err)
		}

		// Merge manifests
		for _, manifest := range manifests {
			if _, ok := all[manifest.Name]; !ok {
				all[manifest.Name] = manifest
			}

			// Merge channels
			for _, channel := range manifest.Channels {
				found := false
				for _, c := range all[manifest.Name].Channels {
					if c.Name == channel.Name {
						found = true
						break
					}
				}

				if !found {
					all[manifest.Name].Channels = append(all[manifest.Name].Channels, channel)
				}
			}

			// Merge versions
			for _, version := range manifest.Versions {
				found := false
				for _, v := range all[manifest.Name].Versions {
					if v.Version == version.Version {
						found = true
						break
					}
				}

				if !found {
					all[manifest.Name].Versions = append(all[manifest.Name].Versions, version)
				}
			}

			all[manifest.Name].Origin = manipath
		}

	}

	for _, manifest := range all {
		index.Manifests = append(index.Manifests, manifest)
	}

	// Sort manifests by name
	sort.Slice(index.Manifests, func(i, j int) bool {
		return index.Manifests[i].Name < index.Manifests[j].Name
	})

	for i := range index.Manifests {
		// Sort manifest versions by version
		sort.Slice(index.Manifests[i].Versions, func(j, k int) bool {
			jSemVer, err := semver.NewVersion(index.Manifests[i].Versions[j].Version)
			if err != nil {
				return index.Manifests[i].Versions[j].Version > index.Manifests[i].Versions[k].Version
			}

			kSemVer, err := semver.NewVersion(index.Manifests[i].Versions[j].Version)
			if err != nil {
				return index.Manifests[i].Versions[j].Version > index.Manifests[i].Versions[k].Version
			}

			return jSemVer.GreaterThan(kSemVer)
		})

		// Now, sort manifest versions by Unikraft version.  This prioritizes the
		// Unikraft version but ensures the latest version of manifest is also
		// first for the given Unikraft version.
		sort.Slice(index.Manifests[i].Versions, func(j, k int) bool {
			jSemVer, err := semver.NewVersion(index.Manifests[i].Versions[j].Unikraft)
			if err != nil {
				return index.Manifests[i].Versions[j].Unikraft > index.Manifests[i].Versions[k].Unikraft
			}

			kSemVer, err := semver.NewVersion(index.Manifests[i].Versions[k].Unikraft)
			if err != nil {
				return index.Manifests[i].Versions[j].Unikraft > index.Manifests[i].Versions[k].Unikraft
			}

			return jSemVer.GreaterThan(kSemVer)
		})

		// Sort manifest channels by name
		sort.Slice(index.Manifests[i].Channels, func(j, k int) bool {
			return index.Manifests[i].Channels[j].Name < index.Manifests[i].Channels[k].Name
		})
	}

	return index, nil
}

func (m *ManifestManager) Update(ctx context.Context) error {
	index, err := m.Index(ctx)
	if err != nil {
		return err
	}

	m.indexCache = index

	return m.saveIndex(ctx, index)
}

func (m *ManifestManager) saveIndex(ctx context.Context, index *ManifestIndex) error {
	if index == nil {
		return nil
	}

	return index.SaveTo(ctx, m.LocalManifestIndex(ctx))
}

func (m *ManifestManager) SetSources(_ context.Context, sources ...string) error {
	m.manifests = sources
	return nil
}

func (m *ManifestManager) AddSource(ctx context.Context, source string) error {
	if m.manifests == nil {
		m.manifests = make([]string, 0)
	}

	m.manifests = append(m.manifests, source)

	return nil
}

// Delete implements packmanager.PackageManager.
func (m *ManifestManager) Delete(ctx context.Context, qopts ...packmanager.QueryOption) error {
	packs, err := m.Catalog(ctx, qopts...)
	if err != nil {
		return err
	}

	var errs []error

	for _, pack := range packs {
		if err := pack.Delete(ctx); err != nil {
			errs = append(errs, err)
		}
	}

	// Since the package has been deleted by the underlying package provider (i.e.
	// manifest), update the cache index and save this to disk.
	m.indexCache, err = NewManifestIndexFromFile(m.LocalManifestIndex(ctx))
	if err == nil {
		query := packmanager.NewQuery(qopts...)
		manifests, err := FindManifestsFromSource(ctx,
			m.indexCache.Origin,
			WithAuthConfig(query.Auths()),
			WithCacheDir(m.cacheDir),
			WithUpdate(query.Remote()),
			WithDefaultChannelName(m.defaultChannelName),
		)
		if err != nil {
			return err
		}

		m.indexCache.Manifests = manifests
	}

	if err := m.saveIndex(ctx, m.indexCache); err != nil {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

// Purge implements packmanager.PackageManager.
func (m *ManifestManager) Purge(ctx context.Context) error {
	return os.RemoveAll(m.LocalManifestIndex(ctx))
}

func (m *ManifestManager) RemoveSource(ctx context.Context, source string) error {
	for i, needle := range m.manifests {
		if needle == source {
			ret := make([]string, 0)
			ret = append(ret, m.manifests[:i]...)
			m.manifests = append(ret, m.manifests[i+1:]...)
			break
		}
	}

	return nil
}

func (m *ManifestManager) Pack(ctx context.Context, c component.Component, opts ...packmanager.PackOption) ([]pack.Package, error) {
	return nil, fmt.Errorf("not implemented manifest.manager.Pack")
}

func (m *ManifestManager) Unpack(ctx context.Context, p pack.Package, opts ...packmanager.UnpackOption) ([]component.Component, error) {
	return nil, fmt.Errorf("not implemented manifest.manager.Unpack")
}

func (m *ManifestManager) From(sub pack.PackageFormat) (packmanager.PackageManager, error) {
	return nil, fmt.Errorf("method not applicable to manifest manager")
}

func (m *ManifestManager) Catalog(ctx context.Context, qopts ...packmanager.QueryOption) ([]pack.Package, error) {
	var err error
	var manifests []*Manifest

	query := packmanager.NewQuery(qopts...)
	auths := query.Auths()
	if len(auths) == 0 {
		auths = m.auths
	}
	mopts := []ManifestOption{
		WithAuthConfig(auths),
		WithCacheDir(m.cacheDir),
		WithUpdate(query.Remote()),
		WithDefaultChannelName(m.defaultChannelName),
	}

	log.G(ctx).WithFields(query.Fields()).Debug("querying manifest catalog")

	if len(query.Source()) > 0 {
		provider, err := NewProvider(ctx, query.Source(), mopts...)
		if err != nil {
			return nil, err
		}

		manifests, err = provider.Manifests()
		if err != nil {
			return nil, err
		}
	} else if query.Remote() {
		// If Catalog is executed in multiple successive calls, which occurs when
		// searching for multiple packages sequentially, check if the cacheIndex has
		// been set.  Even if UseCache set has been set, it means that at least once
		// call to Catalog has properly updated the index.
		if m.indexCache == nil {
			indexCache, err := m.Index(ctx)
			if err != nil {
				return nil, err
			}

			m.indexCache = &ManifestIndex{}
			*m.indexCache = *indexCache
		}

		manifests = m.indexCache.Manifests
	} else if query.Local() {
		m.indexCache, err = NewManifestIndexFromFile(m.LocalManifestIndex(ctx))
		if err == nil {
			manifests, err = FindManifestsFromSource(ctx, m.indexCache.Origin, mopts...)
			if err != nil {
				return nil, err
			}
		}
	}

	var packages []pack.Package
	var g glob.Glob
	types := query.Types()
	name := query.Name()
	version := query.Version()

	if len(name) > 0 {
		t, n, v, err := unikraft.GuessTypeNameVersion(name)

		// Overwrite additional attributes if pattern-matchable
		if err == nil {
			name = n
			if t != unikraft.ComponentTypeUnknown {
				types = append(types, t)
			}

			if len(v) > 0 {
				version = v
			}
		}
	}

	g = glob.MustCompile(name)

	for _, manifest := range manifests {
		if len(types) > 0 {
			found := false
			for _, t := range types {
				if manifest.Type == t {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		if len(query.Source()) > 0 && manifest.Origin != query.Source() {
			continue
		}

		if len(name) > 0 && !g.Match(manifest.Name) {
			continue
		}

		var versions []string
		if len(version) > 0 {
			if len(manifest.Versions) == 1 && len(manifest.Versions[0].Version) == 0 {
				log.G(ctx).Warn("manifest does not supply version")
			}

			for _, v := range manifest.Versions {
				if v.Version == version {
					versions = append(versions, v.Version)
					break
				}
				if v.Unikraft == version {
					versions = append(versions, v.Unikraft)
					break
				}
			}
			if len(versions) == 0 {
				for _, channel := range manifest.Channels {
					if channel.Name == version {
						versions = append(versions, channel.Name)
						break
					}
				}
			}

			if len(versions) == 0 {
				break
			}
		}

		if len(versions) > 0 {
			for _, version := range versions {
				p, err := NewPackageFromManifestWithVersion(manifest, version, mopts...)
				if err != nil {
					log.G(ctx).Warn(err)
					continue
					// TODO: Config option for fast-fail?
					// return nil, err
				}

				packages = append(packages, p)
			}
		} else {
			more, err := NewPackageFromManifest(manifest, mopts...)
			if err != nil {
				log.G(ctx).Trace(err)
				continue
				// TODO: Config option for fast-fail?
				// return nil, err
			}

			packages = append(packages, more)
		}
	}

	// Sort packages by name before returning
	sort.SliceStable(packages, func(i, j int) bool {
		iRunes := []rune(packages[i].Name())
		jRunes := []rune(packages[j].Name())

		max := len(iRunes)
		if max > len(jRunes) {
			max = len(jRunes)
		}

		for idx := 0; idx < max; idx++ {
			ir := iRunes[idx]
			jr := jRunes[idx]

			lir := unicode.ToLower(ir)
			ljr := unicode.ToLower(jr)

			if lir != ljr {
				return lir < ljr
			}

			// the lowercase runes are the same, so compare the original
			if ir != jr {
				return ir < jr
			}
		}

		// If the strings are the same up to the length of the shortest string,
		// the shorter string comes first
		return len(iRunes) < len(jRunes)
	})

	log.G(ctx).Debugf("found %d/%d matching packages in manifest catalog", len(packages), len(manifests))

	return packages, nil
}

func (m *ManifestManager) IsCompatible(ctx context.Context, source string, qopts ...packmanager.QueryOption) (packmanager.PackageManager, bool, error) {
	log.G(ctx).WithFields(logrus.Fields{
		"source": source,
	}).Trace("checking if source is compatible with the manifest manager")

	if source == "" {
		return nil, false, fmt.Errorf("empty source")
	}

	if t, _, _, err := unikraft.GuessTypeNameVersion(source); err == nil && t != unikraft.ComponentTypeUnknown {
		return m, true, nil
	}

	if strings.HasSuffix(source, ".git") ||
		strings.HasPrefix(source, "git@") ||
		strings.HasPrefix(source, "github.com/") ||
		strings.Contains(source, "://github.com/") {
		return m, true, nil
	}

	query := packmanager.NewQuery(qopts...)
	auths := query.Auths()
	if len(auths) == 0 {
		auths = m.auths
	}

	if _, err := NewProvider(ctx, source,
		WithUpdate(packmanager.NewQuery(qopts...).Remote()),
		WithAuthConfig(auths),
	); err != nil {
		return nil, false, fmt.Errorf("incompatible source: %w", err)
	}

	return m, true, nil
}

// LocalManifestDir returns the user configured path to all the manifests
func (m *ManifestManager) LocalManifestsDir(ctx context.Context) string {
	if len(m.localManifestDir) > 0 {
		return m.localManifestDir
	}

	if len(config.G[config.KraftKit](ctx).Paths.Manifests) > 0 {
		return config.G[config.KraftKit](ctx).Paths.Manifests
	}

	return filepath.Join(config.DataDir(), "manifests")
}

// LocalManifestIndex returns the user configured path to the manifest index
func (m *ManifestManager) LocalManifestIndex(ctx context.Context) string {
	return filepath.Join(m.LocalManifestsDir(ctx), "index.yaml")
}

func (m *ManifestManager) Format() pack.PackageFormat {
	return ManifestFormat
}
