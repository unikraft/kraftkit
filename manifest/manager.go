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

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/log"
	"kraftkit.sh/pack"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/component"
)

type manager struct{}

// useGit is a local variable used within the context of the manifest package
// and is dynamically injected as a CLI option.
var useGit = false

func init() {
	// Register a new pack.Package type
	packmanager.RegisterPackageManager(ManifestFormat, NewManifestManager())

	// Register additional command-line flags
	cmdfactory.RegisterFlag(
		"kraft pkg pull",
		cmdfactory.BoolVarP(
			&useGit,
			"git", "g",
			false,
			"Use Git when pulling sources",
		),
	)
}

// NewManifestManager returns a `packmanager.PackageManager` which manipulates
// Unikraft manifests.
func NewManifestManager() packmanager.PackageManager {
	return manager{}
}

// update retrieves and returns a cache of the upstream manifest registry
func (m manager) update(ctx context.Context) (*ManifestIndex, error) {
	if len(config.G[config.KraftKit](ctx).Unikraft.Manifests) == 0 {
		return nil, fmt.Errorf("no manifests specified in config")
	}

	localIndex := &ManifestIndex{
		LastUpdated: time.Now(),
	}

	mopts := []ManifestOption{
		WithAuthConfig(config.G[config.KraftKit](ctx).Auth),
		WithSourcesRootDir(config.G[config.KraftKit](ctx).Paths.Sources),
	}

	for _, manipath := range config.G[config.KraftKit](ctx).Unikraft.Manifests {
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

		localIndex.Origin = manipath
		localIndex.Manifests = append(localIndex.Manifests, manifests...)
	}

	return localIndex, nil
}

func (m manager) Update(ctx context.Context) error {
	// Create parent directories if not present
	if err := os.MkdirAll(filepath.Dir(m.LocalManifestIndex(ctx)), 0o771); err != nil {
		return err
	}

	localIndex, err := m.update(ctx)
	if err != nil {
		return err
	}

	// TODO: Partition directories when there is a large number of manifests
	// TODO: Merge manifests of same name and type?

	// Create a file for each manifest
	for i, manifest := range localIndex.Manifests {
		filename := manifest.Name + ".yaml"

		if manifest.Type != unikraft.ComponentTypeCore {
			filename = manifest.Type.Plural() + "/" + filename
		}

		fileloc := filepath.Join(m.LocalManifestsDir(ctx), filename)
		if err := os.MkdirAll(filepath.Dir(fileloc), 0o771); err != nil {
			return err
		}

		log.G(ctx).WithFields(logrus.Fields{
			"path": fileloc,
		}).Debugf("saving manifest")

		if err := manifest.WriteToFile(fileloc); err != nil {
			log.G(ctx).Errorf("could not save manifest: %s", err)
		}

		// Replace manifest with relative path
		localIndex.Manifests[i] = &Manifest{
			Name:     manifest.Name,
			Type:     manifest.Type,
			Manifest: "./" + filename,
		}
	}

	return localIndex.WriteToFile(m.LocalManifestIndex(ctx))
}

func (m manager) AddSource(ctx context.Context, source string) error {
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

func (m manager) RemoveSource(ctx context.Context, source string) error {
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

func (m manager) Pack(ctx context.Context, c component.Component, opts ...packmanager.PackOption) ([]pack.Package, error) {
	return nil, fmt.Errorf("not implemented manifest.manager.Pack")
}

func (m manager) Unpack(ctx context.Context, p pack.Package, opts ...packmanager.UnpackOption) ([]component.Component, error) {
	return nil, fmt.Errorf("not implemented manifest.manager.Unpack")
}

func (m manager) From(sub pack.PackageFormat) (packmanager.PackageManager, error) {
	return nil, fmt.Errorf("method not applicable to manifest manager")
}

func (m manager) Catalog(ctx context.Context, query packmanager.CatalogQuery) ([]pack.Package, error) {
	var err error
	var index *ManifestIndex
	var allManifests []*Manifest

	mopts := []ManifestOption{
		WithAuthConfig(config.G[config.KraftKit](ctx).Auth),
		WithSourcesRootDir(config.G[config.KraftKit](ctx).Paths.Sources),
	}

	log.G(ctx).WithFields(logrus.Fields{
		"name":    query.Name,
		"version": query.Version,
		"source":  query.Source,
		"types":   query.Types,
		"cache":   !query.NoCache,
	}).Debug("querying manifest catalog")

	if len(query.Source) > 0 {
		provider, err := NewProvider(ctx, query.Source, mopts...)
		if err != nil {
			return nil, err
		}

		manifests, err := provider.Manifests()
		if err != nil {
			return nil, err
		}

		allManifests = append(allManifests, manifests...)
	} else if query.NoCache {
		index, err = m.update(ctx)
		if err != nil {
			return nil, err
		}

		allManifests = append(allManifests, index.Manifests...)
	} else {
		index, err = NewManifestIndexFromFile(ctx, m.LocalManifestIndex(ctx))
		if err != nil {
			return nil, err
		}

		manifests, err := FindManifestsFromSource(ctx, index.Origin, mopts...)
		if err != nil {
			return nil, err
		}

		allManifests = append(allManifests, manifests...)
	}

	log.G(ctx).Debugf("found %d manifests in catalog", len(allManifests))

	var packages []pack.Package
	var g glob.Glob

	if len(query.Name) > 0 {
		t, n, v, err := unikraft.GuessTypeNameVersion(query.Name)

		// Overwrite additional attributes if pattern-matchable
		if err == nil {
			query.Name = n
			if t != unikraft.ComponentTypeUnknown {
				query.Types = append(query.Types, t)
			}

			if len(v) > 0 {
				query.Version = v
			}
		}
	}

	g = glob.MustCompile(query.Name)

	for _, manifest := range allManifests {
		if len(query.Types) > 0 {
			found := false
			for _, t := range query.Types {
				if manifest.Type == t {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		if len(query.Source) > 0 && manifest.Origin != query.Source {
			continue
		}

		if len(query.Name) > 0 && !g.Match(manifest.Name) {
			continue
		}

		var versions []string
		if len(query.Version) > 0 {
			for _, version := range manifest.Versions {
				if version.Version == query.Version {
					versions = append(versions, version.Version)
					break
				}
			}
			if len(versions) == 0 {
				for _, channel := range manifest.Channels {
					if channel.Name == query.Version {
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
				p, err := NewPackageFromManifestWithVersion(ctx, manifest, version, mopts...)
				if err != nil {
					log.G(ctx).Warnf("%v", err)
					continue
					// TODO: Config option for fast-fail?
					// return nil, err
				}

				packages = append(packages, p)
			}
		} else {
			more, err := NewPackageFromManifest(ctx, manifest, mopts...)
			if err != nil {
				log.G(ctx).Warnf("%v", err)
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

	return packages, nil
}

func (m manager) IsCompatible(ctx context.Context, source string) (packmanager.PackageManager, bool, error) {
	log.G(ctx).WithFields(logrus.Fields{
		"source": source,
	}).Debug("checking if source is compatible with the manifest manager")
	if _, err := NewProvider(ctx, source); err != nil {
		return nil, false, fmt.Errorf("incompatible source")
	}

	return m, true, nil
}

// LocalManifestDir returns the user configured path to all the manifests
func (m manager) LocalManifestsDir(ctx context.Context) string {
	if len(config.G[config.KraftKit](ctx).Paths.Manifests) > 0 {
		return config.G[config.KraftKit](ctx).Paths.Manifests
	}

	return filepath.Join(config.DataDir(), "manifests")
}

// LocalManifestIndex returns the user configured path to the manifest index
func (m manager) LocalManifestIndex(ctx context.Context) string {
	return filepath.Join(m.LocalManifestsDir(ctx), "index.yaml")
}

func (m manager) Format() pack.PackageFormat {
	return ManifestFormat
}
