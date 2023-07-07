// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package manifest

import (
	"context"
	"os"
	"path/filepath"

	"github.com/juju/errors"
	"kraftkit.sh/log"
	"kraftkit.sh/pack"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/app"
)

type DirectoryProvider struct {
	typ   unikraft.ComponentType
	name  string
	dir   string
	mopts *ManifestOptions
	ctx   context.Context
}

// NewDirectoryProvider attempts to parse a provided path as a Unikraft
// "microlibirary" directory
func NewDirectoryProvider(ctx context.Context, path string, opts ...ManifestOption) (Provider, error) {
	if f, err := os.Stat(path); err != nil || (err == nil && !f.IsDir()) {
		return nil, errors.Annotatef(err, "could not access directory '%s'", path)
	}

	// Determine the type preemptively
	dirname := filepath.Base(path)
	t, n, _, err := unikraft.GuessTypeNameVersion(dirname)
	if err != nil || t == unikraft.ComponentTypeUnknown {
		for _, f := range app.DefaultFileNames {
			if f, err := os.Stat(filepath.Join(path, f)); err == nil && f.Size() > 0 {
				t = unikraft.ComponentTypeApp
				break
			}
		}
	}

	// TODO: This is a very lightweight way of determining whether the provided
	// directory is a microlibrary.  In reality, a `Config.uk` and `Makefile.uk`
	// could also exist within an application or external platform.  Use
	// introspection to learn the registration used to determine the type/
	if t == unikraft.ComponentTypeUnknown {
		if f, err := os.Stat(filepath.Join(path, unikraft.Config_uk)); err == nil && f.Size() > 0 {
			t = unikraft.ComponentTypeLib
		}
	}

	if t == unikraft.ComponentTypeUnknown {
		return nil, errors.Errorf("unknown type for directory: %s", path)
	}

	provider := DirectoryProvider{
		typ:   t,
		name:  n,
		dir:   path,
		mopts: NewManifestOptions(opts...),
		ctx:   ctx,
	}

	return &provider, nil
}

func (dp DirectoryProvider) Manifests() ([]*Manifest, error) {
	manifest := &Manifest{
		Type:     dp.typ,
		Name:     dp.name,
		Provider: dp,
		Origin:   dp.dir,
		Channels: []ManifestChannel{
			{
				Name:     "default",
				Default:  true,
				Resource: dp.dir,
			},
		},
	}

	return []*Manifest{manifest}, nil
}

func (dp DirectoryProvider) PullManifest(ctx context.Context, manifest *Manifest, opts ...pack.PullOption) error {
	popts, err := pack.NewPullOptions(opts...)
	if err != nil {
		return err
	}

	if len(popts.Workdir()) == 0 {
		return errors.New("cannot pull without without working directory")
	}

	// The directory provider only has one channel, exploit this knowledge
	if len(manifest.Channels) != 1 {
		return errors.New("cannot determine channel for directory provider")
	}

	local, err := unikraft.PlaceComponent(
		popts.Workdir(),
		manifest.Type,
		manifest.Name,
	)
	if err != nil {
		return errors.Errorf("could not place component package: %s", err)
	}

	f, err := os.Lstat(local)
	if err != nil && os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(local), 0o755); err != nil {
			return err
		}
	} else if err == nil && f.IsDir() {
		log.G(ctx).Warnf("local directory already exists: %s", local)
		return nil
	} else if err == nil && f.Mode()&os.ModeSymlink == os.ModeSymlink {
		if err := os.Remove(local); err != nil {
			return errors.Annotate(err, "could not remove symlink")
		}
	}

	// Simply generate a symbolic link to the directory resource
	if err := os.Symlink(manifest.Channels[0].Resource, local); err != nil {
		return errors.Annotate(err, "could not copy directory")
	}

	return nil
}

func (dp DirectoryProvider) String() string {
	return "dir"
}
