// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package manifest

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"

	"kraftkit.sh/config"
	"kraftkit.sh/log"
	"kraftkit.sh/pack"
	"kraftkit.sh/unikraft"

	"kraftkit.sh/internal/version"
)

type Manifest struct {
	// Name of the entity which this manifest represents
	Name string `yaml:"name"`

	// Type of entity which this manifest represetns
	Type unikraft.ComponentType `yaml:"type"`

	// Manifest is used to point to remote manifest, allowing the manifest itself
	// to be retrieved by indirection.  Manifest is XOR with Versions and should
	// be back-propagated.
	Manifest string `yaml:"manifest,omitempty"`

	// Description of what this manifest represents
	Description string `yaml:"description,omitempty"`

	// Origin represents where (and therefore how) this manifest was populated
	Origin string `yaml:"origin,omitempty"`

	// Provider is the string name of the underlying implementation providing the
	// contents of this manifest
	Provider Provider `yaml:"provider,omitempty"`

	// Channels provides multiple ways to retrieve versions.  Classically this is
	// a separation between "staging" and "stable"
	Channels []ManifestChannel `yaml:"channels,omitempty"`

	// Versions
	Versions []ManifestVersion `yaml:"versions,omitempty"`

	// auth is an internal property set by a ManifestOption which is used by the
	// Manifest to access information a bout itself aswell as downloading a given
	// resource
	auths map[string]config.AuthConfig
}

type ManifestProvider struct {
	path     string
	manifest *Manifest
}

// NewManifestProvider accepts an input path which is checked against a local
// file on disk and a remote URL.  If populating a Manifest struct is possible
// given the path, then this provider is able to return list of exactly 1
// manifest.
func NewManifestProvider(ctx context.Context, path string, mopts ...ManifestOption) (Provider, error) {
	manifest, err := NewManifestFromFile(ctx, path, mopts...)
	if err == nil {
		log.G(ctx).WithFields(logrus.Fields{
			"path": path,
		}).Trace("retrieved manifest")
		return ManifestProvider{
			path:     path,
			manifest: manifest,
		}, nil
	}

	manifest, err = NewManifestFromURL(ctx, path, mopts...)
	if err == nil {
		log.G(ctx).WithFields(logrus.Fields{
			"path": path,
		}).Trace("retrieved manifest")
		return ManifestProvider{
			path:     path,
			manifest: manifest,
		}, nil
	}

	return nil, fmt.Errorf("provided path is not a manifest: %s", path)
}

func (mp ManifestProvider) Manifests() ([]*Manifest, error) {
	return []*Manifest{mp.manifest}, nil
}

func (mp ManifestProvider) PullPackage(ctx context.Context, manifest *Manifest, popts *pack.PackageOptions, ppopts *pack.PullPackageOptions) error {
	return pullArchive(ctx, manifest, popts, ppopts)
}

func (mp ManifestProvider) String() string {
	return "manifest"
}

// NewManifestFromBytes parses a byte array of a YAML representing a manifest
func NewManifestFromBytes(ctx context.Context, raw []byte, mopts ...ManifestOption) (*Manifest, error) {
	// TODO: This deserialization mechanism is used to encode the provider into the
	// resulting manifest file and feels a bit of a hack since we are running
	// `yaml.Marshal` twice.  The library exposes `yaml.Marshler` and
	// `yaml.Unmarshaller` which is a nicer implementation.  The challenge though
	// is that the marshalling should ideally occur on the Provider implementation
	// -- which would ultimately require "trial-and-error" to discover, or
	// however, map to the correct implementation.  Because this interface is not
	// implemented, this code is duplicated also inside of index.go

	contents := make(map[string]interface{})

	if err := yaml.Unmarshal(raw, contents); err != nil {
		return nil, err
	}

	providerName := ""

	if v, ok := contents["provider"]; ok {
		providerName = v.(string)

		delete(contents, "provider")
	}

	raw, err := yaml.Marshal(contents)
	if err != nil {
		return nil, err
	}

	manifest := &Manifest{}
	if err := yaml.Unmarshal(raw, manifest); err != nil {
		return nil, err
	}

	if len(manifest.Name) == 0 {
		return nil, fmt.Errorf("unset name in manifest")
	} else if len(manifest.Type) == 0 {
		return nil, fmt.Errorf("unset type in manifest")
	}

	if providerName != "" {
		manifest.Provider, err = NewProviderFromString(ctx, providerName, manifest.Origin, manifest, mopts...)
		if err != nil {
			return nil, err
		}
	}

	for _, o := range mopts {
		if err := o(manifest); err != nil {
			return nil, err
		}
	}

	return manifest, nil
}

// NewManifestFromFile reads in a manifest file from a given path
func NewManifestFromFile(ctx context.Context, path string, mopts ...ManifestOption) (*Manifest, error) {
	f, err := os.Stat(path)
	if err != nil {
		return nil, err
	} else if f.Size() == 0 {
		return nil, fmt.Errorf("manifest path is empty: %s", path)
	}

	// Check if we're directly pointing to a compatible manifest file
	ext := filepath.Ext(path)
	if ext != ".yml" && ext != ".yaml" {
		return nil, fmt.Errorf("unsupported manifest extension for path: %s", path)
	}

	contents, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	manifest, err := NewManifestFromBytes(ctx, contents, mopts...)
	if err != nil {
		return nil, err
	}

	manifest.Origin = path

	return manifest, nil
}

// NewManifestFromURL retrieves a provided path as a Manifest from a remote
// location over HTTP
func NewManifestFromURL(ctx context.Context, path string, mopts ...ManifestOption) (*Manifest, error) {
	u, err := url.Parse(path)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("provided path was not a valid URL")
	}

	var contents []byte
	client := &http.Client{}

	head, err := http.NewRequestWithContext(ctx, "HEAD", path, nil)
	if err != nil {
		return nil, err
	}

	head.Header.Set("User-Agent", "kraftkit/"+version.Version())

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

	get, err := http.NewRequestWithContext(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	get.Header.Set("User-Agent", "kraftkit/"+version.Version())

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
		return nil, fmt.Errorf("received %d error when retrieving: %s", resp.StatusCode, path)
	}

	// Check if we're directly pointing to a compatible manifest file
	ext := filepath.Ext(path)
	if ext != ".yml" && ext != ".yaml" {
		return nil, fmt.Errorf("unsupported manifest extension for path: %s", path)
	}

	contents, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	manifest, err := NewManifestFromBytes(ctx, contents, mopts...)
	if err != nil {
		return nil, err
	}

	manifest.Origin = path

	return manifest, nil
}

// FindManifestsFromSource is a recursive method which follows a given source
// and attempts to instantiate a Provider which matches the given source.  If
// the source is recognised by a provider, it is traversed to return all the
// known Manifests.
func FindManifestsFromSource(ctx context.Context, source string, mopts ...ManifestOption) ([]*Manifest, error) {
	return findManifestsFromSource(ctx, "", source, mopts)
}

// findManifestsFromSource is an internal method which recursively traverses a
// path to a manifest and if symbolic link is presented within the read
// manifest, it is retrieved via this method.  This is only recursive if the
// option to be followed is set.
func findManifestsFromSource(ctx context.Context, lastSource, source string, mopts []ManifestOption) ([]*Manifest, error) {
	var manifests []*Manifest

	// Follow relative paths by using the lastSource
	if len(lastSource) > 0 {
		if f, err := os.Stat(lastSource); err == nil && f.IsDir() {
			source = filepath.Join(lastSource, source)
		} else {
			u, err := url.ParseRequestURI(lastSource)

			if err != nil || u.Scheme == "" || u.Host == "" {
				// Source is not an URL, so we can assume it's file structured
				dir, _ := filepath.Split(lastSource)
				source = filepath.Join(dir, source)
			} else {
				// Source is an URL, so we can just append the path
				dir, _ := filepath.Split(lastSource)
				source = dir + source[2:]
			}
		}
	}

	provider, err := NewProvider(ctx, source, mopts...)
	if err != nil {
		return nil, err
	}

	newManifests, err := provider.Manifests()
	if err != nil {
		return nil, err
	}

	var wg sync.WaitGroup
	var mu sync.RWMutex
	wg.Add(len(newManifests))

	for i := range newManifests {
		go func(i int) {
			if len(newManifests[i].Manifest) > 0 {
				defer wg.Done()
				next, ohno := findManifestsFromSource(ctx, source, newManifests[i].Manifest, mopts)
				if ohno != nil {
					mu.Lock()
					err = ohno
					mu.Unlock()
					return
				}

				if len(next) > 0 {
					mu.Lock()
					manifests = append(manifests, next...)
					mu.Unlock()
				}
			} else {
				mu.Lock()
				newManifests[i].Origin = source // Save the origin of the manifest
				newManifests[i].Provider = provider
				manifests = append(manifests, newManifests[i])
				mu.Unlock()
				wg.Done()
			}
		}(i)
	}

	wg.Wait()

	if err != nil {
		return nil, err
	}

	return manifests, nil
}

// WriteToFile saves the manifest as a YAML format file at the given path
func (m Manifest) WriteToFile(path string) error {
	// Open the file (create if not present)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return fmt.Errorf("could not open file: %v", err)
	}

	defer f.Close()

	contents, err := yaml.Marshal(m)
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
	// implemented, this code is duplicated also inside of index.go
	var iface map[string]interface{}
	if err := yaml.Unmarshal(contents, &iface); err != nil {
		return err
	}

	if m.Provider != nil {
		iface["provider"] = m.Provider.String()
	} else {
		delete(iface, "provider")
	}

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

// DefaultChannel returns the default channel of the Manifest
func (m Manifest) DefaultChannel() (*ManifestChannel, error) {
	if len(m.Channels) == 0 {
		return nil, fmt.Errorf("manifest does not have any channels")
	}

	// Use the channel by default to determine the latest version.  In the
	// scenario where the version is not a semver (and thus can be compared
	// mechanicslly) this field will be populated correctly by upstream manifests.
	for _, channel := range m.Channels {
		if channel.Default {
			return &channel, nil
		}
	}

	return nil, fmt.Errorf("manifest does not have a default channel: %s", m.Origin)
}

// Auths returns the map of provided authentication configuration passed as an
// option to the Manifest
func (m Manifest) Auths() map[string]config.AuthConfig {
	return m.auths
}
