// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package oci

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
	"oras.land/oras-go/v2/content"

	"kraftkit.sh/config"
	"kraftkit.sh/initrd"
	kraftkitversion "kraftkit.sh/internal/version"
	"kraftkit.sh/kconfig"
	"kraftkit.sh/log"
	"kraftkit.sh/oci/handler"
	"kraftkit.sh/oci/simpleauth"
	"kraftkit.sh/pack"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/arch"
	"kraftkit.sh/unikraft/plat"
	"kraftkit.sh/unikraft/target"
)

const ConfigFilename = "config.json"

type ociPackage struct {
	handle   handler.Handler
	ref      name.Reference
	manifest *Manifest

	// Embedded attributes which represent target.Target
	arch    arch.Architecture
	plat    plat.Platform
	kconfig kconfig.KeyValueMap
	kernel  string
	initrd  *initrd.InitrdConfig
	command []string
}

var (
	_ pack.Package  = (*ociPackage)(nil)
	_ target.Target = (*ociPackage)(nil)
)

// NewPackageFromTarget generates an OCI implementation of the pack.Package
// construct based on an input Application and options.
func NewPackageFromTarget(ctx context.Context, targ target.Target, opts ...packmanager.PackOption) (pack.Package, error) {
	var err error

	popts := &packmanager.PackOptions{}
	for _, opt := range opts {
		opt(popts)
	}

	// Initialize the ociPackage by copying over target.Target attributes
	ocipack := ociPackage{
		arch:    targ.Architecture(),
		plat:    targ.Platform(),
		kconfig: targ.KConfig(),
		kernel:  targ.Kernel(),
		initrd:  targ.Initrd(),
		command: popts.Args(),
	}

	if popts.Name() == "" {
		return nil, fmt.Errorf("cannot create package without name")
	}
	ocipack.ref, err = name.ParseReference(
		popts.Name(),
		name.WithDefaultRegistry(""),
	)
	if err != nil {
		return nil, fmt.Errorf("could not parse image reference: %w", err)
	}

	auths, err := defaultAuths(ctx)
	if err != nil {
		return nil, err
	}

	if contAddr := config.G[config.KraftKit](ctx).ContainerdAddr; len(contAddr) > 0 {
		namespace := DefaultNamespace
		if n := os.Getenv("CONTAINERD_NAMESPACE"); n != "" {
			namespace = n
		}

		log.G(ctx).WithFields(logrus.Fields{
			"addr":      contAddr,
			"namespace": namespace,
		}).Debug("oci: packaging via containerd")

		ctx, ocipack.handle, err = handler.NewContainerdHandler(ctx, contAddr, namespace, auths)
	} else {
		if gerr := os.MkdirAll(config.G[config.KraftKit](ctx).RuntimeDir, fs.ModeSetgid|0o775); gerr != nil {
			return nil, fmt.Errorf("could not create local oci cache directory: %w", gerr)
		}

		ociDir := filepath.Join(config.G[config.KraftKit](ctx).RuntimeDir, "oci")

		log.G(ctx).WithFields(logrus.Fields{
			"path": ociDir,
		}).Trace("oci: directory handler")

		ocipack.handle, err = handler.NewDirectoryHandler(ociDir, auths)
	}
	if err != nil {
		return nil, err
	}

	// TODO: Remove the existing reference if a --force-remove|--overwrite flag is
	// provided (which should then translate into an PackOptions attribute).
	// existingDesc, err := handle.Resolve(ctx, ocipack.Name())
	// if err == nil && popts.Overwrite() && existingDesc.MediaType == ocispec.MediaTypeImageManifest {
	// 	reader, err := handle.Fetch(ctx, existingDesc)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	//
	//  log.G(ctx).WithFields(logrus.Fields{
	//		"tag": ocipack.Name(),
	//	}).Warn("removing existing reference")
	//
	// 	// TODO: Remove the manifest descriptor
	//
	// }

	ocipack.manifest, err = NewManifest(ctx, ocipack.handle)
	if err != nil {
		return nil, err
	}

	log.G(ctx).WithFields(logrus.Fields{
		"dest": WellKnownKernelPath,
	}).Debug("oci: including kernel")

	layer, err := NewLayerFromFile(ctx,
		ocispec.MediaTypeImageLayer,
		ocipack.Kernel(),
		WellKnownKernelPath,
		WithLayerAnnotation(AnnotationKernelPath, WellKnownKernelPath),
	)
	if err != nil {
		return nil, err
	}
	defer os.Remove(layer.tmp)

	if _, err := ocipack.manifest.AddLayer(ctx, layer); err != nil {
		return nil, err
	}

	if popts.Initrd() != "" {
		log.G(ctx).Debug("oci: including initrd")
		initRdPath := popts.Initrd()
		if f, err := os.Stat(initRdPath); err == nil && f.IsDir() {
			cwd, err2 := os.Getwd()
			if err2 != nil {
				return nil, err2
			}

			file, err := os.CreateTemp("", "kraftkit-oci-archive-*")
			if err != nil {
				return nil, err
			}
			defer os.Remove(file.Name())

			cfg, err2 := initrd.NewFromMapping(cwd,
				file.Name(),
				fmt.Sprintf("%s:/", initRdPath))
			if err2 != nil {
				return nil, err2
			}

			initRdPath = cfg.Output
		}
		layer, err := NewLayerFromFile(ctx,
			ocispec.MediaTypeImageLayer,
			initRdPath,
			WellKnownInitrdPath,
			WithLayerAnnotation(AnnotationKernelInitrdPath, WellKnownInitrdPath),
		)
		if err != nil {
			return nil, err
		}
		defer os.Remove(layer.tmp)

		if _, err := ocipack.manifest.AddLayer(ctx, layer); err != nil {
			return nil, err
		}
	}

	// TODO(nderjung): See below.

	// if popts.PackKernelLibraryObjects() {
	// 	log.G(ctx).Debug("oci: including kernel library objects")
	// }

	// if popts.PackKernelLibraryIntermediateObjects() {
	// 	log.G(ctx).Debug("oci: including kernel library intermediate objects")
	// }

	// if popts.PackKernelSourceFiles() {
	// 	log.G(ctx).Debug("oci: including kernel source files")
	// }

	// if popts.PackAppSourceFiles() {
	// 	log.G(ctx).Debug("oci: including application source files")
	// }

	ocipack.manifest.SetAnnotation(ctx, AnnotationName, ocipack.Name())
	ocipack.manifest.SetAnnotation(ctx, AnnotationVersion, ocipack.ref.Identifier())
	ocipack.manifest.SetAnnotation(ctx, AnnotationKraftKitVersion, kraftkitversion.Version())
	if version := popts.KernelVersion(); len(version) > 0 {
		ocipack.manifest.SetAnnotation(ctx, AnnotationKernelVersion, version)
		ocipack.manifest.SetOSVersion(ctx, version)
	}

	if popts.PackKConfig() {
		log.G(ctx).Debug("oci: including .config")
		for _, k := range ocipack.KConfig() {
			// TODO(nderjung): Not sure if these filters are best placed here or
			// elsewhere.

			// Filter out host-specific KConfig options
			if k.Key == "CONFIG_UK_BASE" {
				continue
			}

			ocipack.manifest.SetOSFeature(ctx, k.String())
		}
	}

	ocipack.manifest.SetCmd(ctx, ocipack.Command())
	ocipack.manifest.SetOS(ctx, ocipack.Platform().Name())
	ocipack.manifest.SetArchitecture(ctx, ocipack.Architecture().Name())

	log.G(ctx).WithFields(logrus.Fields{
		"tag": ocipack.Name(),
	}).Debug("oci: saving image")

	_, err = ocipack.manifest.Save(ctx, ocipack.imageRef(), nil)
	if err != nil {
		return nil, err
	}

	return &ocipack, nil
}

// NewPackageFromOCIManifestSpec generates a package from a supplied OCI image
// manifest specification.
func NewPackageFromOCIManifestSpec(ctx context.Context, handle handler.Handler, ref string, manifest ocispec.Manifest) (pack.Package, error) {
	// Check if the OCI image has a known annotation which identifies if a
	// unikernel is contained within
	if _, ok := manifest.Annotations[AnnotationKernelVersion]; !ok {
		return nil, fmt.Errorf("OCI image does not contain a Unikraft unikernel")
	}

	var err error

	ocipack := ociPackage{
		handle: handle,
	}

	ocipack.ref, err = name.ParseReference(ref,
		name.WithDefaultRegistry(DefaultRegistry),
	)
	if err != nil {
		return nil, err
	}

	ocipack.manifest, err = NewManifestFromSpec(ctx, handle, manifest)
	if err != nil {
		return nil, err
	}

	architecture, err := arch.TransformFromSchema(ctx, ocipack.manifest.manifest.Config.Platform.Architecture)
	if err != nil {
		return nil, err
	}

	ocipack.arch = architecture.(arch.Architecture)

	platform, err := plat.TransformFromSchema(ctx, ocipack.manifest.manifest.Config.Platform.OS)
	if err != nil {
		return nil, err
	}

	ocipack.plat = platform.(plat.Platform)

	return &ocipack, nil
}

// NewPackageFromRemoteOCIRef generates a new package from a given OCI image
// reference which is accessed by its remote registry.
func NewPackageFromRemoteOCIRef(ctx context.Context, handle handler.Handler, ref string) (pack.Package, error) {
	var err error

	ocipack := ociPackage{
		handle: handle,
	}

	ocipack.ref, err = name.ParseReference(ref,
		name.WithDefaultRegistry(DefaultRegistry),
	)
	if err != nil {
		return nil, fmt.Errorf("cannot parse OCI image name reference: %v", err)
	}

	auths, err := defaultAuths(ctx)
	if err != nil {
		return nil, err
	}

	authConfig := &authn.AuthConfig{}

	// Annoyingly convert between regtypes and authn.
	if auth, ok := auths[ocipack.ref.Context().RegistryStr()]; ok {
		authConfig.Username = auth.User
		authConfig.Password = auth.Token
	}

	raw, err := crane.Manifest(ref,
		crane.WithAuth(&simpleauth.SimpleAuthenticator{
			Auth: authConfig,
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("could not get manifest: %v", err)
	}

	var manifest ocispec.Manifest

	if err := json.Unmarshal(raw, &manifest); err != nil {
		return nil, fmt.Errorf("could not unmarshal manifest: %v", err)
	}

	// Check if the OCI image has a known annotation which identifies if a
	// unikernel is contained within
	if _, ok := manifest.Annotations[AnnotationKernelVersion]; !ok {
		return nil, fmt.Errorf("OCI image does not contain a Unikraft unikernel")
	}

	ocipack.manifest, err = NewManifestFromSpec(ctx, handle, manifest)
	if err != nil {
		return nil, fmt.Errorf("could not generate image from manifest: %v", err)
	}

	if manifest.Config.Platform == nil {
		return nil, fmt.Errorf("remote image platform is unknown")
	}

	// TODO(nderjung): Setting the architecture and platform are a bit of a hack
	// at the moment.  A nicer mechanism should be used.
	architecture, err := arch.TransformFromSchema(ctx, manifest.Config.Platform.Architecture)
	if err != nil {
		return nil, fmt.Errorf("could not convert architecture string")
	}

	var ok bool

	ocipack.arch, ok = architecture.(arch.Architecture)
	if !ok {
		return nil, fmt.Errorf("could not convert architecture string")
	}

	platform, err := plat.TransformFromSchema(ctx, manifest.Config.Platform.OS)
	if err != nil {
		return nil, fmt.Errorf("could not convert platform string")
	}

	ocipack.plat, ok = platform.(plat.Platform)
	if !ok {
		return nil, fmt.Errorf("could not convert platform string")
	}

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
func (ocipack *ociPackage) Metadata() any {
	return ocipack.manifest.config
}

// Push implements pack.Package
func (ocipack *ociPackage) Push(ctx context.Context, opts ...pack.PushOption) error {
	manifestJson, err := json.Marshal(ocipack.manifest.manifest)
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}

	ocipack.manifest.desc = content.NewDescriptorFromBytes(
		ocispec.MediaTypeImageManifest,
		manifestJson,
	)
	return ocipack.manifest.handle.PushImage(ctx, ocipack.imageRef(), &ocipack.manifest.desc)
}

// Pull implements pack.Package
func (ocipack *ociPackage) Pull(ctx context.Context, opts ...pack.PullOption) error {
	popts, err := pack.NewPullOptions(opts...)
	if err != nil {
		return err
	}

	// If it's possible to resolve the image reference, the image has already been
	// pulled to the local image store
	image, err := ocipack.handle.ResolveImage(ctx, ocipack.imageRef())
	if err == nil {
		goto unpack
	}

	if err := ocipack.manifest.handle.FetchImage(
		ctx,
		ocipack.imageRef(),
		fmt.Sprintf("%s/%s", ocipack.plat.Name(), ocipack.arch.Name()),
		popts.OnProgress,
	); err != nil {
		return err
	}

	// Try resolving the image again after pulling it
	image, err = ocipack.handle.ResolveImage(ctx, ocipack.imageRef())
	if err != nil {
		return err
	}

unpack:
	// Unpack the image if a working directory has been provided
	if len(popts.Workdir()) > 0 {
		if err := ocipack.manifest.handle.UnpackImage(
			ctx,
			ocipack.imageRef(),
			popts.Workdir(),
		); err != nil {
			return err
		}

		// Set the kernel, since it is a well-known within the destination path
		ocipack.kernel = filepath.Join(popts.Workdir(), WellKnownKernelPath)

		// Set the command
		ocipack.command = image.Config.Cmd
		ocipack.manifest.config = image

		// Set the initrd if available
		initrdPath := filepath.Join(popts.Workdir(), WellKnownInitrdPath)
		if f, err := os.Stat(initrdPath); err == nil && f.Size() > 0 {
			ocipack.initrd, err = initrd.NewFromFile(popts.Workdir(), initrdPath)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// Delete deletes OCI package from the host machine.
func (ocipack *ociPackage) Delete(ctx context.Context, version string) error {
	ociDir := path.Join(config.G[config.KraftKit](ctx).RuntimeDir, "oci")

	// Removing config layer
	typeAndId := strings.Split(string(ocipack.manifest.manifest.Config.Digest), ":")
	if err := os.RemoveAll(path.Join(ociDir, "configs", typeAndId[0], typeAndId[1])); err != nil {
		return err
	}

	// Removing image layers
	for _, layer := range ocipack.manifest.manifest.Layers {
		typeAndId = strings.Split(string(layer.Digest), ":")
		if err := os.RemoveAll(path.Join(ociDir, "layers", typeAndId[0], typeAndId[1])); err != nil {
			return err
		}
	}
	if _, err := os.Stat(path.Join(ociDir, "manifests", ocipack.Name())); !os.IsNotExist(err) {
		if err := deleteManifests(ocipack.Name(), ocipack.Version(), ociDir); err != nil {
			return err
		}
	}

	if strings.HasPrefix(ocipack.Name(), DefaultRegistry) {
		if err := deleteManifests(path.Join("index.unikraft.io", ocipack.Name()),
			ocipack.Version(), ociDir); err != nil {
			return err
		}
	}
	return nil
}

func deleteManifests(name string, version string, ociDir string) error {
	// Removing manifest file
	if err := os.RemoveAll(path.Join(ociDir,
		"manifests",
		name,
		version+".json")); err != nil {
		return err
	}

	files, err := os.ReadDir(path.Join(ociDir, "manifests", name))
	if err != nil {
		return err
	}
	if len(files) == 0 {
		if err = os.RemoveAll(path.Join(ociDir, "manifests", name)); err != nil {
			return err
		}
	}
	return nil
}

// Pull implements pack.Package
func (ocipack *ociPackage) Format() pack.PackageFormat {
	return OCIFormat
}

// Source implements unikraft.component.Component
func (ocipack *ociPackage) Source() string {
	return ""
}

// Path implements unikraft.component.Component
func (ocipack *ociPackage) Path() string {
	return ""
}

// KConfigTree implements unikraft.component.Component
func (ocipack *ociPackage) KConfigTree(context.Context, ...*kconfig.KeyValue) (*kconfig.KConfigFile, error) {
	return nil, fmt.Errorf("not implemented: oci.ociPackage.KConfigTree")
}

// KConfig implements unikraft.component.Component
func (ocipack *ociPackage) KConfig() kconfig.KeyValueMap {
	return ocipack.kconfig
}

// PrintInfo implements unikraft.component.Component
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
	return ocipack.kernel
}

// Initrd implements unikraft.target.Target
func (ocipack *ociPackage) Initrd() *initrd.InitrdConfig {
	return ocipack.initrd
}

// Command implements unikraft.target.Target
func (ocipack *ociPackage) Command() []string {
	if len(ocipack.command) == 0 {
		return []string{"--"}
	}

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
