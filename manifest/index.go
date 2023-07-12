// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package manifest

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"

	"kraftkit.sh/log"
	"kraftkit.sh/pack"

	"kraftkit.sh/internal/version"
)

type ManifestIndex struct {
	Name        string      `yaml:"name,omitempty"`
	LastUpdated time.Time   `yaml:"last_updated"`
	Manifests   []*Manifest `yaml:"manifests"`
	Origin      string      `yaml:"-"`
}

type ManifestIndexProvider struct {
	path  string
	index *ManifestIndex
	mopts []ManifestOption
	ctx   context.Context
}

// NewManifestIndexProvider accepts an input path which is checked against first
// a local file on disk and then a remote URL.  If either of these checks pass,
// the Provider is instantiated since the path does indeed represent a
// ManifestIndex.
func NewManifestIndexProvider(ctx context.Context, path string, mopts ...ManifestOption) (Provider, error) {
	index, err := NewManifestIndexFromFile(path, mopts...)
	if err == nil {
		log.G(ctx).WithFields(logrus.Fields{
			"path": path,
		}).Trace("retrieved index")
		return &ManifestIndexProvider{
			path:  path,
			index: index,
			mopts: mopts,
			ctx:   ctx,
		}, nil
	}

	index, err = NewManifestIndexFromURL(ctx, path, mopts...)
	if err == nil {
		log.G(ctx).WithFields(logrus.Fields{
			"path": path,
		}).Trace("retrieved index")
		return &ManifestIndexProvider{
			path:  path,
			index: index,
			mopts: mopts,
			ctx:   ctx,
		}, nil
	}

	return nil, fmt.Errorf("provided path is not a manifest index: %s", path)
}

func (mip *ManifestIndexProvider) Manifests() ([]*Manifest, error) {
	return mip.index.Manifests, nil
}

func (mip *ManifestIndexProvider) PullManifest(ctx context.Context, manifest *Manifest, opts ...pack.PullOption) error {
	return fmt.Errorf("not implemented: manifest.ManifestIndexProvider.PullManifest")
}

func (mip *ManifestIndexProvider) String() string {
	return "index"
}

// NewManifestIndexFromBytes parses a byte array of a YAML representing a
// manifest index
func NewManifestIndexFromBytes(raw []byte, opts ...ManifestOption) (*ManifestIndex, error) {
	index := &ManifestIndex{}

	if err := yaml.Unmarshal(raw, index); err != nil {
		return nil, err
	}

	if index.Manifests == nil {
		return nil, fmt.Errorf("nothing found in manifest index")
	}

	for i, manifest := range index.Manifests {
		manifest.mopts = NewManifestOptions(opts...)
		index.Manifests[i] = manifest
	}

	return index, nil
}

func NewManifestIndexFromFile(path string, mopts ...ManifestOption) (*ManifestIndex, error) {
	f, err := os.Stat(path)
	if err != nil {
		return nil, err
	} else if f.Size() == 0 {
		return nil, fmt.Errorf("manifest index is empty: %s", path)
	}

	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	index, err := NewManifestIndexFromBytes(contents, mopts...)
	if err != nil {
		return nil, err
	}

	index.Origin = path

	return index, nil
}

// NewManifestFromURL retrieves a provided path as a ManifestIndex from a remote
// location over HTTP
func NewManifestIndexFromURL(ctx context.Context, path string, mopts ...ManifestOption) (*ManifestIndex, error) {
	_, err := url.Parse(path)
	if err != nil {
		return nil, err
	}

	client := &http.Client{}

	head, err := http.NewRequestWithContext(ctx, "HEAD", path, nil)
	if err != nil {
		return nil, err
	}

	head.Header.Set("User-Agent", version.UserAgent())

	log.G(ctx).WithFields(logrus.Fields{
		"url":    path,
		"method": "HEAD",
	}).Trace("http")

	resp, err := client.Do(head)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("manifest index not found: %s", path)
	}

	get, err := http.NewRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}

	get.Header.Set("User-Agent", version.UserAgent())

	log.G(ctx).WithFields(logrus.Fields{
		"url":    path,
		"method": "GET",
	}).Trace("http")

	resp, err = client.Do(get)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("received %d error when retreiving: %s", resp.StatusCode, path)
	}

	// Check if we're directly pointing to a compatible manifest file
	ext := filepath.Ext(path)
	if ext != ".yml" && ext != ".yaml" {
		return nil, fmt.Errorf("unsupported manifest index extension for path: %s", path)
	}

	contents, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	index, err := NewManifestIndexFromBytes(contents, mopts...)
	if err != nil {
		return nil, err
	}

	index.Origin = path

	return index, nil
}

func (mi *ManifestIndex) WriteToFile(path string) error {
	// Open the file (create if not present)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return fmt.Errorf("could not open file: %v", err)
	}

	defer f.Close()

	contents, err := yaml.Marshal(mi)
	if err != nil {
		return err
	}

	// TODO: This serialization mechanism is used to encode the provider into the
	// resulting manifest file and feels a bit of a hack since we are running
	// `yaml.Marshal` twice.  The library exposes `yaml.Marshler` and
	// `yaml.Unmarshaller` which is a nicer implementation.  The challenge though
	// is that the marshalling should ideally occur on the Provider implementation
	// -- which would ultimately require "trial-and-error" to discover, or
	// however, map to the correct implementation.  Because this interface is not
	// implemented, this code is duplicated also inside of manifest.go
	var iface map[string]interface{}
	if err := yaml.Unmarshal(contents, &iface); err != nil {
		return err
	}

	delete(iface, "provider")

	contents, err = yaml.Marshal(iface)
	if err != nil {
		return err
	}

	if err := f.Truncate(0); err != nil {
		return err
	}

	_, err = f.Write(contents)
	if err != nil {
		return err
	}

	return nil
}
