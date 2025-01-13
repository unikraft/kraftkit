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
	golog "log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/google/go-containerregistry/pkg/authn"
	gcrlogs "github.com/google/go-containerregistry/pkg/logs"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	"oras.land/oras-go/v2/content"

	"kraftkit.sh/config"
	"kraftkit.sh/initrd"
	"kraftkit.sh/internal/set"
	"kraftkit.sh/internal/tableprinter"
	"kraftkit.sh/kconfig"
	"kraftkit.sh/log"
	"kraftkit.sh/oci/cache"
	"kraftkit.sh/oci/handler"
	"kraftkit.sh/oci/simpleauth"
	ociutils "kraftkit.sh/oci/utils"
	"kraftkit.sh/pack"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/arch"
	"kraftkit.sh/unikraft/plat"
	"kraftkit.sh/unikraft/target"
)

const ConfigFilename = "config.json"

// ociPackage works by referencing a specific manifest which represents the
// "package" as well as the index that manifest should be part of.  When
// when internally referencing the packaged entity, this is the manifest and its
// representation is presented via the index.
type ociPackage struct {
	handle   handler.Handler
	ref      name.Reference
	index    *Index
	manifest *Manifest
	auths    map[string]config.AuthConfig

	// Embedded attributes which represent target.Target
	arch      arch.Architecture
	plat      plat.Platform
	kconfig   kconfig.KeyValueMap
	kernel    string
	kernelDbg string
	initrd    initrd.Initrd
	command   []string
	labels    map[string]string

	original *ociPackage
}

var (
	_ pack.Package  = (*ociPackage)(nil)
	_ target.Target = (*ociPackage)(nil)
)

// NewPackageFromTarget generates an OCI implementation of the pack.Package
// construct based on an input Application and options.
func NewPackageFromTarget(ctx context.Context, handle handler.Handler, targ target.Target, opts ...packmanager.PackOption) (pack.Package, error) {
	var err error

	popts := packmanager.NewPackOptions()
	for _, opt := range opts {
		opt(popts)
	}

	// Initialize the ociPackage by copying over target.Target attributes
	ocipack := ociPackage{
		arch:      targ.Architecture(),
		plat:      targ.Platform(),
		kconfig:   targ.KConfig(),
		initrd:    targ.Initrd(),
		kernel:    targ.Kernel(),
		kernelDbg: targ.KernelDbg(),
		command:   popts.Args(),
		labels:    popts.Labels(),
		handle:    handle,
	}

	// It is possible that `NewPackageFromTarget` is called with an existing
	// `targ` which represents a previously generated OCI package, e.g. via
	// `NewPackageFromOCIManifestDigest`.  In this case, we can keep a reference
	// to the original package and use it to re-tag the original manifest or any
	// access any other related information which may otherwise be lost through
	// the `target.Target` or `pack.Package` interfaces.
	if original, ok := targ.(*ociPackage); ok {
		ocipack.original = original
	}

	if popts.Name() == "" {
		return nil, fmt.Errorf("cannot create package without name")
	}
	ocipack.ref, err = name.ParseReference(
		popts.Name(),
		name.WithDefaultRegistry(DefaultRegistry),
		name.WithDefaultTag(DefaultTag),
	)
	if err != nil {
		return nil, fmt.Errorf("could not parse image reference: %w", err)
	}

	// Prepare a new manifest which contains the individual components of the
	// target, including the kernel image.
	ocipack.manifest, err = NewManifest(ctx, ocipack.handle)
	if err != nil {
		return nil, fmt.Errorf("could not instantiate new manifest structure: %w", err)
	}

	if len(ocipack.Kernel()) > 0 {
		log.G(ctx).
			WithField("src", ocipack.Kernel()).
			WithField("dest", WellKnownKernelPath).
			Debug("including kernel")

		layer, err := NewLayerFromFile(ctx,
			ocispec.MediaTypeImageLayer,
			ocipack.Kernel(),
			WellKnownKernelPath,
			WithLayerAnnotation(AnnotationKernelPath, WellKnownKernelPath),
		)
		if err != nil {
			return nil, fmt.Errorf("could not create new layer structure from file: %w", err)
		}
		defer os.Remove(layer.tmp)

		if _, err := ocipack.manifest.AddLayer(ctx, layer); err != nil {
			return nil, fmt.Errorf("could not add layer to manifest: %w", err)
		}
	} else if ocipack.original != nil {
		// It is possible that a target is instantiated from a previously generated
		// package reference and a kernel has not been supplied explicitly.  In this
		// circumstance, we adopt the original manifest's list of layers, which can
		// include a reference to a kernel.
		ocipack.manifest.layers = ocipack.original.manifest.layers
	}

	if popts.KernelDbg() && len(ocipack.KernelDbg()) > 0 {
		log.G(ctx).
			WithField("src", ocipack.KernelDbg()).
			WithField("dest", WellKnownKernelDbgPath).
			Debug("oci: including kernel.dbg")

		layer, err := NewLayerFromFile(ctx,
			ocispec.MediaTypeImageLayer,
			ocipack.Kernel(),
			WellKnownKernelDbgPath,
		)
		if err != nil {
			return nil, fmt.Errorf("could not create new layer structure from file: %w", err)
		}
		defer os.Remove(layer.tmp)

		if _, err := ocipack.manifest.AddLayer(ctx, layer); err != nil {
			return nil, fmt.Errorf("could not add layer to manifest: %w", err)
		}
	}

	if popts.Initrd() != "" {
		log.G(ctx).
			WithField("src", popts.Initrd()).
			WithField("dest", WellKnownInitrdPath).
			Debug("including initrd")

		layer, err := NewLayerFromFile(ctx,
			ocispec.MediaTypeImageLayer,
			popts.Initrd(),
			WellKnownInitrdPath,
			WithLayerAnnotation(AnnotationKernelInitrdPath, WellKnownInitrdPath),
		)
		if err != nil {
			return nil, fmt.Errorf("could build layer from file: %w", err)
		}
		defer os.Remove(layer.tmp)

		if _, err := ocipack.manifest.AddLayer(ctx, layer); err != nil {
			return nil, err
		}
	}

	// TODO(nderjung): See below.

	// if popts.PackKernelLibraryObjects() {
	// 	log.G(ctx).Debug("including kernel library objects")
	// }

	// if popts.PackKernelLibraryIntermediateObjects() {
	// 	log.G(ctx).Debug("including kernel library intermediate objects")
	// }

	// if popts.PackKernelSourceFiles() {
	// 	log.G(ctx).Debug("including kernel source files")
	// }

	// if popts.PackAppSourceFiles() {
	// 	log.G(ctx).Debug("including application source files")
	// }

	if ocipack.original != nil {
		ocipack.manifest.config = ocipack.original.manifest.config
	}

	ocipack.manifest.SetAnnotation(ctx, AnnotationName, ocipack.Name())
	if version := popts.KernelVersion(); len(version) > 0 {
		ocipack.manifest.SetAnnotation(ctx, AnnotationKernelVersion, version)
		ocipack.manifest.SetOSVersion(ctx, version)
	}

	if len(ocipack.Command()) > 0 {
		cmd := ocipack.Command()
		log.G(ctx).
			WithField("args", cmd).
			Debug("cmd")

		ocipack.manifest.SetCmd(ctx, cmd)
	} else if ocipack.original != nil {
		cmd := ocipack.original.manifest.config.Config.Cmd
		log.G(ctx).
			WithField("args", cmd).
			Debug("cmd")

		ocipack.manifest.SetCmd(ctx, cmd)
	}

	ocipack.manifest.SetOS(ctx, ocipack.Platform().Name())
	ocipack.manifest.SetArchitecture(ctx, ocipack.Architecture().Name())
	ocipack.manifest.SetEnv(ctx, popts.Env())
	for _, env := range ocipack.manifest.config.Config.Env {
		k, v, _ := strings.Cut(env, "=")
		log.G(ctx).WithField(k, v).Debug("env")
	}

	switch popts.MergeStrategy() {
	case packmanager.StrategyMerge, packmanager.StrategyAbort:
		ocipack.index, err = NewIndexFromRef(ctx, ocipack.handle, ocipack.ref.Name())
		if err != nil {
			ocipack.index, err = NewIndex(ctx, ocipack.handle)
			if err != nil {
				return nil, fmt.Errorf("could not instantiate new image structure: %w", err)
			}
		} else if popts.MergeStrategy() == packmanager.StrategyAbort {
			return nil, fmt.Errorf("cannot overwrite existing manifest as merge strategy is set to exit on conflict")
		}

	case packmanager.StrategyOverwrite:
		if err := ocipack.handle.DeleteIndex(ctx, ocipack.ref.Name(), false); err != nil {
			return nil, fmt.Errorf("could not remove existing index: %w", err)
		}

		ocipack.index, err = NewIndex(ctx, ocipack.handle)
		if err != nil {
			return nil, fmt.Errorf("could not instantiate new image structure: %w", err)
		}
	default:
		return nil, fmt.Errorf("package merge strategy unset")
	}

	if popts.MergeStrategy() == packmanager.StrategyAbort && len(ocipack.index.manifests) > 0 {
		return nil, fmt.Errorf("cannot continue: reference already exists and merge strategy set to none")
	}

	if len(ocipack.index.manifests) > 0 {
		// Sort the features alphabetically.  This ensures that comparisons between
		// versions are symmetric.
		sort.Slice(ocipack.manifest.config.OSFeatures, func(i, j int) bool {
			// Check if we have numbers, sort them accordingly
			if z, err := strconv.Atoi(ocipack.manifest.config.OSFeatures[i]); err == nil {
				if y, err := strconv.Atoi(ocipack.manifest.config.OSFeatures[j]); err == nil {
					return y < z
				}
				// If we get only one number, alway say its greater than letter
				return true
			}
			// Compare letters normally
			return ocipack.manifest.config.OSFeatures[j] > ocipack.manifest.config.OSFeatures[i]
		})

		newManifestChecksum, err := ociutils.PlatformChecksum(ocipack.ref.String(), &ocispec.Platform{
			Architecture: ocipack.manifest.config.Architecture,
			OS:           ocipack.manifest.config.OS,
			OSVersion:    ocipack.manifest.config.OSVersion,
			OSFeatures:   ocipack.manifest.config.OSFeatures,
		})
		if err != nil {
			return nil, fmt.Errorf("could not generate manifest platform checksum: %w", err)
		}

		var manifests []*Manifest

		for _, existingManifest := range ocipack.index.manifests {
			existingManifestChecksum, err := ociutils.PlatformChecksum(ocipack.ref.String(), &ocispec.Platform{
				Architecture: existingManifest.config.Architecture,
				OS:           existingManifest.config.OS,
				OSVersion:    existingManifest.config.OSVersion,
				OSFeatures:   existingManifest.config.OSFeatures,
			})
			if err != nil {
				return nil, fmt.Errorf("could not generate manifest platform checksum for '%s': %w", existingManifest.desc.Digest.String(), err)
			}
			if existingManifestChecksum == newManifestChecksum {
				switch popts.MergeStrategy() {
				case packmanager.StrategyAbort:
					return nil, fmt.Errorf("cannot overwrite existing manifest as merge strategy is set to exit on conflict")

				// A manifest with the same configuration has been detected, in
				// both cases,
				case packmanager.StrategyOverwrite, packmanager.StrategyMerge:
					if err := ocipack.handle.DeleteManifest(ctx, ocipack.ref.Name(), existingManifest.desc.Digest); err != nil {
						return nil, fmt.Errorf("could not overwrite existing manifest: %w", err)
					}
				}
			} else {
				manifests = append(manifests, existingManifest)
			}
		}

		ocipack.index.saved = false
		ocipack.index.manifests = manifests
	}

	if popts.PackKConfig() {
		log.G(ctx).
			Debug("including list of kconfig as features")

		// TODO(nderjung): Not sure if these filters are best placed here or
		// elsewhere.
		skippable := set.NewStringSet(
			"CONFIG_UK_APP",
			"CONFIG_UK_BASE",
		)
		for _, k := range ocipack.KConfig() {
			// Filter out host-specific KConfig options.
			if skippable.Contains(k.Key) {
				continue
			}

			log.G(ctx).
				WithField(k.Key, k.Value).
				Trace("feature")

			ocipack.manifest.SetOSFeature(ctx, k.String())
		}
	}

	for k, v := range ocipack.labels {
		ocipack.manifest.SetLabel(ctx, k, v)
		log.G(ctx).
			WithField(k, v).
			Trace("label")
	}

	if err := ocipack.index.AddManifest(ctx, ocipack.manifest); err != nil {
		return nil, fmt.Errorf("could not add manifest to index: %w", err)
	}

	if _, err = ocipack.index.Save(ctx, ocipack.ref.String(), nil); err != nil {
		return nil, fmt.Errorf("could not save index: %w", err)
	}

	return &ocipack, nil
}

// newPackageFromOCIManifestDigest is an internal method which retrieves the OCI
// manifest from a remote reference and digest and returns, if found, an
// instantiated Index and Manifest structure based on its contents.
func newIndexAndManifestFromRemoteDigest(ctx context.Context, handle handler.Handler, fullref string, auths map[string]config.AuthConfig, dgst digest.Digest) (*Index, *Manifest, error) {
	ref, err := name.ParseReference(fullref,
		name.WithDefaultRegistry(""),
		name.WithDefaultTag(DefaultTag),
	)
	if err != nil {
		return nil, nil, err
	}

	if ref.Context().RegistryStr() == "" {
		ref, err = name.ParseReference(fullref,
			name.WithDefaultRegistry(DefaultRegistry),
			name.WithDefaultTag(DefaultTag),
		)
		if err != nil {
			return nil, nil, err
		}
	}

	if auths == nil {
		auths = config.G[config.KraftKit](ctx).Auth
	}

	var retManifest *Manifest
	authConfig := &authn.AuthConfig{}
	transport := http.DefaultTransport.(*http.Transport).Clone()

	// Annoyingly convert between regtypes and authn.
	if auth, ok := auths[ref.Context().RegistryStr()]; ok {
		authConfig.Username = auth.User
		authConfig.Password = auth.Token

		if !auth.VerifySSL {
			transport.TLSClientConfig = &tls.Config{
				InsecureSkipVerify: true,
			}
		}
	}

	v1ImageIndex, err := cache.RemoteIndex(ref,
		remote.WithContext(ctx),
		remote.WithAuth(&simpleauth.SimpleAuthenticator{
			Auth: authConfig,
		}),
		remote.WithTransport(transport),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("could not get index from registry: %v", err)
	}

	index, err := NewIndex(ctx, handle)
	if err != nil {
		return nil, nil, err
	}

	v1ImageIndexManifest, err := v1ImageIndex.IndexManifest()
	if err != nil {
		return nil, nil, fmt.Errorf("could not access index manifest: %w", err)
	}

	ociIndex, err := FromGoogleV1IndexManifestToOCISpec(*v1ImageIndexManifest)
	if err != nil {
		return nil, nil, fmt.Errorf("could not convert index manifest: %w", err)
	}

	indexJson, err := json.Marshal(ociIndex)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal manifest: %w", err)
	}

	indexDesc := content.NewDescriptorFromBytes(
		ocispec.MediaTypeImageIndex,
		indexJson,
	)

	index.desc = &indexDesc
	eg, egCtx := errgroup.WithContext(ctx)

	for i := range ociIndex.Manifests {
		eg.Go(func(i int) func() error {
			return func() error {
				descriptor := ociIndex.Manifests[i]

				manifest, err := NewManifest(egCtx, handle)
				if err != nil {
					return fmt.Errorf("could not instantiate new manifest: %w", err)
				}

				ref, err := name.ParseReference(
					fmt.Sprintf("%s@%s", ref.Context().Name(), descriptor.Digest),
				)
				if err != nil {
					return fmt.Errorf("could not parse reference: %w", err)
				}

				manifestSpec, imageSpec, err := handle.ResolveManifest(egCtx, "", descriptor.Digest)
				if err == nil {
					manifest.manifest = manifestSpec
					manifest.config = imageSpec
					manifest.config.Architecture = descriptor.Platform.Architecture
					manifest.config.Platform = *descriptor.Platform
				} else {
					manifest.v1Image, err = cache.RemoteImage(
						ref,
						remote.WithPlatform(v1.Platform{
							Architecture: descriptor.Platform.Architecture,
							OS:           descriptor.Platform.OS,
							OSFeatures:   descriptor.Platform.OSFeatures,
						}),
						remote.WithContext(egCtx),
						remote.WithAuth(&simpleauth.SimpleAuthenticator{
							Auth: authConfig,
						}),
						remote.WithTransport(transport),
					)
					if err != nil {
						return fmt.Errorf("getting image: %w", err)
					}

					b, err := manifest.v1Image.RawManifest()
					if err != nil {
						return fmt.Errorf("getting manifest: %w", err)
					}

					if err := json.Unmarshal(b, &manifest.manifest); err != nil {
						return fmt.Errorf("unmarshalling manifest: %w", err)
					}

					v1Manifest, err := v1.ParseManifest(bytes.NewReader(b))
					if err != nil {
						return fmt.Errorf("parsing manifest: %w", err)
					}

					for _, desc := range v1Manifest.Layers {
						manifest.layers = append(manifest.layers, &Layer{
							blob: &Blob{
								desc: FromGoogleV1DescriptorToOCISpec(desc)[0],
							},
						})
					}

					b, err = manifest.v1Image.RawConfigFile()
					if err != nil {
						return fmt.Errorf("getting config: %w", err)
					}

					if err := json.Unmarshal(b, manifest.config); err != nil {
						return fmt.Errorf("unmarshalling config: %w", err)
					}
				}

				manifest.desc = &descriptor
				manifest.saved = false
				index.manifests = append(index.manifests, manifest)

				if manifest.desc.Digest.String() == dgst.String() {
					retManifest = manifest
				}

				return nil
			}
		}(i))
	}

	if err := eg.Wait(); err != nil {
		return nil, nil, err
	}

	return index, retManifest, nil
}

// NewPackageFromOCIManifestDigest is a constructor method which
// instantiates a package based on the OCI format based on a provided OCI
// Image manifest digest.
func NewPackageFromOCIManifestDigest(ctx context.Context, handle handler.Handler, ref string, auths map[string]config.AuthConfig, dgst digest.Digest) (pack.Package, error) {
	var err error

	ocipack := ociPackage{
		handle: handle,
		auths:  auths,
	}

	ocipack.ref, err = name.ParseReference(ref,
		name.WithDefaultRegistry(""),
		name.WithDefaultTag(DefaultTag),
	)
	if err != nil {
		return nil, err
	}

	// First, check if the digest exists locally, this determines whether we
	// continue to instantiate it from the local host or from from a remote
	// registry.
	if info, _ := handle.DigestInfo(ctx, dgst); info != nil {
		ocipack.index, err = NewIndexFromRef(ctx, handle, ref)
		if err != nil {
			log.G(ctx).
				Debugf("could not instantiate index from local reference: %s", err.Error())

			// Re-attempt by fetching remotely.
			ocipack.index, ocipack.manifest, err = newIndexAndManifestFromRemoteDigest(ctx, handle, ref, auths, dgst)
			if err != nil {
				return nil, fmt.Errorf("could not instantiate index and manifest from remote digest: %w", err)
			}
		} else {
			manifest, err := NewManifestFromDigest(ctx, handle, dgst)
			if err != nil {
				return nil, fmt.Errorf("could not instantiate manifest from digest: %w", err)
			}

			ocipack.manifest = manifest
		}
	} else {
		ocipack.index, ocipack.manifest, err = newIndexAndManifestFromRemoteDigest(ctx, handle, ref, auths, dgst)
		if err != nil {
			return nil, err
		}

		if ocipack.manifest == nil {
			return nil, fmt.Errorf("could not find manifest with digest '%s' in index '%s'", dgst.String(), ref)
		}
	}

	architecture, err := arch.TransformFromSchema(ctx,
		ocipack.manifest.manifest.Config.Platform.Architecture,
	)
	if err != nil {
		return nil, err
	}

	ocipack.arch = architecture.(arch.Architecture)

	platform, err := plat.TransformFromSchema(ctx,
		ocipack.manifest.manifest.Config.Platform.OS,
	)
	if err != nil {
		return nil, err
	}

	ocipack.plat = platform.(plat.Platform)

	ocipack.kconfig = kconfig.KeyValueMap{}
	for _, feature := range ocipack.manifest.config.OSFeatures {
		_, kval := kconfig.NewKeyValue(feature)
		ocipack.kconfig.Override(kval)
	}

	ocipack.command = ocipack.manifest.config.Config.Cmd

	return &ocipack, nil
}

// Type implements unikraft.Nameable
func (ocipack *ociPackage) Type() unikraft.ComponentType {
	return unikraft.ComponentTypeApp
}

// Name implements unikraft.Nameable
func (ocipack *ociPackage) Name() string {
	return ocipack.ref.Context().Name()
}

// ID implements pack.Package
func (ocipack *ociPackage) ID() string {
	return fmt.Sprintf("%s@%s", ocipack.Name(), ocipack.index.desc.Digest.String())
}

// Name implements fmt.Stringer
func (ocipack *ociPackage) String() string {
	return fmt.Sprintf("%s (%s/%s)", ocipack.imageRef(), ocipack.Platform().Name(), ocipack.Architecture().Name())
}

// Version implements unikraft.Nameable
func (ocipack *ociPackage) Version() string {
	return ocipack.ref.Identifier()
}

// imageRef returns the OCI-standard image name in the format `name:tag`
func (ocipack *ociPackage) imageRef() string {
	if strings.HasPrefix(ocipack.Version(), "sha256:") {
		return fmt.Sprintf("%s@%s", ocipack.Name(), ocipack.Version())
	}
	return fmt.Sprintf("%s:%s", ocipack.Name(), ocipack.Version())
}

// Metadata implements pack.Package
func (ocipack *ociPackage) Metadata() interface{} {
	return ocipack.manifest.config
}

// Size in bytes of the package.
func (ocipack *ociPackage) Size() int64 {
	if len(ocipack.manifest.manifest.Layers) == 0 {
		return -1
	}

	var total int64 = 0

	for _, layer := range ocipack.manifest.manifest.Layers {
		total += layer.Size
	}

	return total
}

// Columns implements pack.Package
func (ocipack *ociPackage) Columns() []tableprinter.Column {
	size := "n/a"

	if sizeb := ocipack.Size(); sizeb > 0 {
		size = humanize.Bytes(uint64(sizeb))
	}

	return []tableprinter.Column{
		{Name: "manifest", Value: ocipack.manifest.desc.Digest.String()[7:14]},
		{Name: "index", Value: ocipack.index.desc.Digest.String()[7:14]},
		{Name: "plat", Value: fmt.Sprintf("%s/%s", ocipack.Platform().Name(), ocipack.Architecture().Name())},
		{Name: "size", Value: size},
	}
}

// Push implements pack.Package
func (ocipack *ociPackage) Push(ctx context.Context, opts ...pack.PushOption) error {
	// In the circumstance where the original package is available, we use
	// google/go-containerregistry to re-tag (which is achieved via `pusher.Push`
	// which ultimately checks if the manifest, its layers, config and ultimately
	// blobs are available in the remote registry, and simply performs a HEAD
	// request which does the actual "re-tagging").  Because the re-tagging
	// process includes a check for existing remote blobs, the original manifest
	// can be fully satisfied with only references which are stored locally and
	// without having to fetch the original blob or upload a new one, improving
	// performance of the `Push` method.
	if ocipack.original != nil && ocipack.original.manifest.v1Image != nil {
		log.G(ctx).
			Debug("re-tagging original package such that remote references are maintained")

		authConfig := &authn.AuthConfig{}
		transport := http.DefaultTransport.(*http.Transport).Clone()

		// Annoyingly convert between regtypes and authn.
		if auth, ok := config.G[config.KraftKit](ctx).Auth[ocipack.ref.Context().RegistryStr()]; ok {
			authConfig.Username = auth.User
			authConfig.Password = auth.Token

			if !auth.VerifySSL {
				transport.TLSClientConfig = &tls.Config{
					InsecureSkipVerify: true,
				}
			}
		}

		gcrlogs.Progress = golog.New(log.G(ctx).WriterLevel(logrus.TraceLevel), "", 0)

		pusher, err := remote.NewPusher(
			remote.WithContext(ctx),
			remote.WithAuth(&simpleauth.SimpleAuthenticator{
				Auth: authConfig,
			}),
			remote.WithTransport(transport),
		)
		if err != nil {
			return err
		}

		manRef, _ := name.ParseReference(
			fmt.Sprintf("%s@%s", ocipack.ref.Context().Name(), ocipack.original.manifest.desc.Digest.String()),
		)

		// Re-tag the original package's manifests
		if err := pusher.Push(ctx, manRef, ocipack.original.manifest.v1Image); err != nil {
			return err
		}
	}

	desc, err := ocipack.index.Descriptor()
	if err != nil {
		return err
	}

	if err := ocipack.handle.PushDescriptor(ctx, ocipack.imageRef(), desc); err != nil {
		return err
	}

	return nil
}

// Unpack implements pack.Package
func (ocipack *ociPackage) Unpack(ctx context.Context, dir string) error {
	image, err := ocipack.handle.UnpackImage(ctx,
		ocipack.imageRef(),
		ocipack.manifest.desc.Digest,
		dir,
	)
	if err != nil {
		return err
	}

	// Set the kernel, since it is a well-known within the destination path
	ocipack.kernel = filepath.Join(dir, WellKnownKernelPath)

	// Set the command
	ocipack.command = image.Config.Cmd

	// Set the initrd if available
	initrdPath := filepath.Join(dir, WellKnownInitrdPath)
	if f, err := os.Stat(initrdPath); err == nil && f.Size() > 0 {
		ocipack.initrd, err = initrd.New(ctx,
			initrdPath,
			initrd.WithArchitecture(image.Architecture),
			initrd.WithWorkdir(dir),
		)
		if err != nil {
			return err
		}
	}

	// Set the environment variables
	ocipack.manifest.config.Config.Env = image.Config.Env

	return nil
}

// Pull implements pack.Package
func (ocipack *ociPackage) Pull(ctx context.Context, opts ...pack.PullOption) error {
	popts, err := pack.NewPullOptions(opts...)
	if err != nil {
		return err
	}

	// Pull the index but set the platform such that the relevant manifests can
	// be retrieved as well.
	if err := ocipack.handle.PullDigest(
		ctx,
		ocispec.MediaTypeImageIndex,
		ocipack.imageRef(),
		ocipack.manifest.desc.Digest,
		ocipack.manifest.desc.Platform,
		popts.OnProgress,
	); err != nil {
		return err
	}

	// The digest for index has now changed following a pull.  Figure out the new
	// manifest by using the
	existingChecksum, err := ociutils.PlatformChecksum(ocipack.imageRef(), ocipack.manifest.desc.Platform)
	if err != nil {
		return fmt.Errorf("calculating checksum for '%s': %w", ocipack.imageRef(), err)
	}

	manifests, err := ocipack.handle.ListManifests(ctx)
	if err != nil {
		return fmt.Errorf("listing existing manifests: %w", err)
	}

	for dgstStr, manifest := range manifests {
		newChecksum, err := ociutils.PlatformChecksum(ocipack.imageRef(), manifest.Config.Platform)
		if err != nil {
			return fmt.Errorf("calculating checksum for '%s': %w", ocipack.imageRef(), err)
		}

		if existingChecksum != newChecksum {
			continue
		}

		dgst, _ := digest.Parse(dgstStr)
		ocipack.manifest, err = NewManifestFromDigest(ctx, ocipack.handle, dgst)
		if err != nil {
			return fmt.Errorf("could not rehydrate manifest: %w", err)
		}

		break
	}

	// Unpack the image if a working directory has been provided
	if len(popts.Workdir()) > 0 {
		return ocipack.Unpack(ctx, popts.Workdir())
	}

	return nil
}

// PulledAt implements pack.Package
func (ocipack *ociPackage) PulledAt(ctx context.Context) (bool, time.Time, error) {
	if len(ocipack.manifest.manifest.Layers) == 0 {
		return false, time.Time{}, nil
	}

	earliest := time.Now()
	pulled := false

	for _, layer := range ocipack.manifest.manifest.Layers {
		info, err := ocipack.handle.DigestInfo(ctx, layer.Digest)
		if err != nil && errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			continue
		}

		pulled = true
		if info.UpdatedAt.Before(earliest) {
			earliest = info.UpdatedAt
		}
	}

	if pulled {
		return true, earliest, nil
	}

	return false, time.Time{}, nil
}

// Delete implements pack.Package.
func (ocipack *ociPackage) Delete(ctx context.Context) error {
	log.G(ctx).
		WithField("ref", ocipack.imageRef()).
		Debug("deleting package")

	if err := ocipack.handle.DeleteManifest(ctx, ocipack.imageRef(), ocipack.manifest.desc.Digest); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("could not delete package manifest: %w", err)
	}

	indexDesc, err := ocipack.handle.ResolveIndex(ctx, ocipack.imageRef())
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return fmt.Errorf("could not resolve index: %w", err)
	} else if indexDesc == nil {
		return nil
	}

	var manifests []ocispec.Descriptor

	for _, manifest := range indexDesc.Manifests {
		if manifest.Digest.String() == ocipack.manifest.desc.Digest.String() {
			continue
		}

		manifests = append(manifests, manifest)
	}

	if len(manifests) == 0 {
		return ocipack.handle.DeleteIndex(ctx, ocipack.imageRef(), true)
	}

	indexDesc.Manifests = manifests

	newIndex, err := NewIndexFromSpec(ctx, ocipack.handle, indexDesc)
	if err != nil {
		return fmt.Errorf("could not prepare new index: %w", err)
	}

	_, err = newIndex.Save(ctx, ocipack.imageRef(), nil)
	return err
}

// Save implements pack.Package
func (ocipack *ociPackage) Save(ctx context.Context) error {
	if _, err := ocipack.manifest.Save(ctx, ocipack.imageRef(), nil); err != nil {
		return fmt.Errorf("saving manifest: %w", err)
	}

	return nil
}

// Pull implements pack.Package
func (ocipack *ociPackage) Format() pack.PackageFormat {
	return OCIFormat
}

// Source implements unikraft.target.Target
func (ocipack *ociPackage) Source() string {
	return ""
}

// Path implements unikraft.target.Target
func (ocipack *ociPackage) Path() string {
	return ""
}

// KConfigTree implements unikraft.target.Target
func (ocipack *ociPackage) KConfigTree(context.Context, ...*kconfig.KeyValue) (*kconfig.KConfigFile, error) {
	return nil, fmt.Errorf("not implemented: oci.ociPackage.KConfigTree")
}

// KConfig implements unikraft.target.Target
func (ocipack *ociPackage) KConfig() kconfig.KeyValueMap {
	return ocipack.kconfig
}

// PrintInfo implements unikraft.target.Target
func (ocipack *ociPackage) PrintInfo(context.Context) string {
	return "not implemented: oci.ociPackage.PrintInfo"
}

// Architecture implements unikraft.target.Target
func (ocipack *ociPackage) Architecture() arch.Architecture {
	return ocipack.arch
}

// Platform implements unikraft.target.Target
func (ocipack *ociPackage) Platform() plat.Platform {
	return ocipack.plat
}

// Kernel implements unikraft.target.Target
func (ocipack *ociPackage) Kernel() string {
	return ocipack.kernel
}

// KernelDbg implements unikraft.target.Target
func (ocipack *ociPackage) KernelDbg() string {
	return ocipack.kernelDbg
}

// Initrd implements unikraft.target.Target
func (ocipack *ociPackage) Initrd() initrd.Initrd {
	return ocipack.initrd
}

// Command implements unikraft.target.Target
func (ocipack *ociPackage) Command() []string {
	return ocipack.command
}

// ConfigFilename implements unikraft.target.Target
func (ocipack *ociPackage) ConfigFilename() string {
	return ""
}

// MarshalYAML implements unikraft.target.Target (yaml.Marshaler)
func (ocipack *ociPackage) MarshalYAML() (interface{}, error) {
	if ocipack == nil {
		return nil, nil
	}

	return map[string]interface{}{
		"architecture": ocipack.arch.Name(),
		"platform":     ocipack.plat.Name(),
	}, nil
}
