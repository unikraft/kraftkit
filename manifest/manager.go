// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package manifest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
	"unicode"

	"github.com/gobwas/glob"
	"github.com/sirupsen/logrus"

	"kraftkit.sh/config"
	"kraftkit.sh/log"
	"kraftkit.sh/pack"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/component"
)

type manifestManager struct {
	manifests  []string
	indexCache *ManifestIndex
}

// NewManifestManager returns a `packmanager.PackageManager` which manipulates
// Unikraft manifests.
func NewManifestManager(ctx context.Context, opts ...any) (packmanager.PackageManager, error) {
	manager := manifestManager{}

	for _, mopt := range opts {
		opt, ok := mopt.(ManifestManagerOption)
		if !ok {
			return nil, fmt.Errorf("cannot cast ManifestManager option")
		}

		if err := opt(ctx, &manager); err != nil {
			return nil, err
		}
	}

	// Populate the internal list of manifests with locally saved manifests
	for _, manifest := range config.G[config.KraftKit](ctx).Unikraft.Manifests {
		if _, compatible, _ := manager.IsCompatible(ctx, manifest); compatible {
			manager.manifests = append(manager.manifests, manifest)
		}
	}

	return &manager, nil
}

// update retrieves and returns a cache of the upstream manifest registry
func (m *manifestManager) update(ctx context.Context) (*ManifestIndex, error) {
	if len(m.manifests) == 0 {
		return nil, fmt.Errorf("no manifests specified in config")
	}

	index := &ManifestIndex{
		LastUpdated: time.Now(),
	}

	mopts := []ManifestOption{
		WithAuthConfig(config.G[config.KraftKit](ctx).Auth),
		WithCacheDir(config.G[config.KraftKit](ctx).Paths.Sources),
	}

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

		index.Origin = manipath
		index.Manifests = append(index.Manifests, manifests...)
	}

	return index, nil
}

func (m *manifestManager) Update(ctx context.Context) error {
	index, err := m.update(ctx)
	if err != nil {
		return err
	}

	m.indexCache = new(ManifestIndex)
	*m.indexCache = *index

	manifests := make([]*Manifest, len(index.Manifests))

	// Create parent directories if not present
	if err := os.MkdirAll(filepath.Dir(m.LocalManifestIndex(ctx)), 0o771); err != nil {
		return err
	}

	// TODO: Partition directories when there is a large number of manifests
	// TODO: Merge manifests of same name and type?

	// Create a file for each manifest
	for i, manifest := range index.Manifests {
		filename := manifest.Name + ".yaml"

		if manifest.Type != unikraft.ComponentTypeCore {
			filename = manifest.Type.Plural() + string(filepath.Separator) + filename
		}

		fileloc := filepath.Join(m.LocalManifestsDir(ctx), filename)
		if err := os.MkdirAll(filepath.Dir(fileloc), 0o771); err != nil {
			return err
		}

		log.G(ctx).WithFields(logrus.Fields{
			"path": fileloc,
		}).Tracef("saving manifest")

		if err := manifest.WriteToFile(fileloc); err != nil {
			log.G(ctx).Errorf("could not save manifest: %s", err)
		}

		// Replace manifest with relative path
		manifests[i] = &Manifest{
			Name:     manifest.Name,
			Type:     manifest.Type,
			Manifest: "./" + filename,
		}
	}

	index.Manifests = manifests

	return index.WriteToFile(m.LocalManifestIndex(ctx))
}

func (m *manifestManager) SetSources(_ context.Context, sources ...string) error {
	m.manifests = sources
	return nil
}

func (m *manifestManager) AddSource(ctx context.Context, source string) error {
	if m.manifests == nil {
		m.manifests = make([]string, 0)
	}

	m.manifests = append(m.manifests, source)

	return nil
}

func (m *manifestManager) RemoveSource(ctx context.Context, source string) error {
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

func (m *manifestManager) Pack(ctx context.Context, c component.Component, opts ...packmanager.PackOption) ([]pack.Package, error) {
	return nil, fmt.Errorf("not implemented manifest.manager.Pack")
}

func (m *manifestManager) Unpack(ctx context.Context, p pack.Package, opts ...packmanager.UnpackOption) ([]component.Component, error) {
	return nil, fmt.Errorf("not implemented manifest.manager.Unpack")
}

func (m *manifestManager) From(sub pack.PackageFormat) (packmanager.PackageManager, error) {
	return nil, fmt.Errorf("method not applicable to manifest manager")
}

func (m *manifestManager) Catalog(ctx context.Context, qopts ...packmanager.QueryOption) ([]pack.Package, error) {
	var err error
	var manifests []*Manifest

	query := packmanager.NewQuery(qopts...)
	mopts := []ManifestOption{
		WithAuthConfig(query.Auths()),
		WithCacheDir(config.G[config.KraftKit](ctx).Paths.Sources),
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
	} else if !query.UseCache() {
		// If Catalog is executed in multiple successive calls, which occurs when
		// searching for multiple packages sequentially, check if the cacheIndex has
		// been set.  Even if UseCache set has been set, it means that at least once
		// call to Catalog has properly updated the index.
		if m.indexCache == nil {
			indexCache, err := m.update(ctx)
			if err != nil {
				return nil, err
			}

			m.indexCache = &ManifestIndex{}
			*m.indexCache = *indexCache
		}

		manifests = m.indexCache.Manifests
	} else {
		m.indexCache, err = NewManifestIndexFromFile(m.LocalManifestIndex(ctx))
		if err != nil {
			return nil, err
		}

		manifests, err = FindManifestsFromSource(ctx, m.indexCache.Origin, mopts...)
		if err != nil {
			return nil, err
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

	log.G(ctx).Tracef("found %d/%d matching manifests in catalog", len(packages), len(manifests))

	return packages, nil
}

func (m *manifestManager) IsCompatible(ctx context.Context, source string, qopts ...packmanager.QueryOption) (packmanager.PackageManager, bool, error) {
	log.G(ctx).WithFields(logrus.Fields{
		"source": source,
	}).Trace("checking if source is compatible with the manifest manager")

	if t, _, _, err := unikraft.GuessTypeNameVersion(source); err == nil && t != unikraft.ComponentTypeUnknown {
		return m, true, nil
	}

	if _, err := NewProvider(ctx, source); err != nil {
		return nil, false, fmt.Errorf("incompatible source")
	}

	return m, true, nil
}

// LocalManifestDir returns the user configured path to all the manifests
func (m *manifestManager) LocalManifestsDir(ctx context.Context) string {
	if len(config.G[config.KraftKit](ctx).Paths.Manifests) > 0 {
		return config.G[config.KraftKit](ctx).Paths.Manifests
	}

	return filepath.Join(config.DataDir(), "manifests")
}

// LocalManifestIndex returns the user configured path to the manifest index
func (m *manifestManager) LocalManifestIndex(ctx context.Context) string {
	return filepath.Join(m.LocalManifestsDir(ctx), "index.yaml")
}

func (m *manifestManager) Format() pack.PackageFormat {
	return ManifestFormat
}
