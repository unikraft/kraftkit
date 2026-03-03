// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package packmanager

import (
	"kraftkit.sh/initrd"
	"kraftkit.sh/kconfig"
	"kraftkit.sh/unikraft/arch"
	"kraftkit.sh/unikraft/plat"
)

// PackOptions contains the list of options which can be set when packaging a
// component.
type PackOptions struct {
	appSourceFiles bool
	architecture   arch.Architecture
	platform       plat.Platform
	args           []string
	env            []string
	initrd         initrd.Initrd
	roms           []string
	kconfig        kconfig.KeyValueMap
	kernel         string
	kernelDbg      string
	kernelVersion  string
	labels         map[string]string
	name           string
	output         string
	mergeStrategy  MergeStrategy
}

// NewPackOptions returns an instantiated *NewPackOptions with default
// configuration options values.
func NewPackOptions() *PackOptions {
	return &PackOptions{
		mergeStrategy: StrategyAbort,
	}
}

// PackAppSourceFiles returns whether the application source files should be
// packaged.
func (popts *PackOptions) PackAppSourceFiles() bool {
	return popts.appSourceFiles
}

// Architecture returns the architecture of the package.
func (popts *PackOptions) Architecture() arch.Architecture {
	return popts.architecture
}

// Platform returns the platform of the package.
func (popts *PackOptions) Platform() plat.Platform {
	return popts.platform
}

// Args returns the arguments to pass to the kernel.
func (popts *PackOptions) Args() []string {
	return popts.args
}

// Env returns the environment variables to be passed to the kernel.
func (popts *PackOptions) Env() []string {
	return popts.env
}

// Kernel returns the path of the kernel file that should be packaged.
func (popts *PackOptions) Kernel() string {
	return popts.kernel
}

// Initrd returns the path of the initrd file that should be packaged.
func (popts *PackOptions) Initrd() initrd.Initrd {
	return popts.initrd
}

// Auxiliary read-only memory blobs.
func (popts *PackOptions) Roms() []string {
	return popts.roms
}

// KConfig returns whether the .config file should be packaged.
func (popts *PackOptions) KConfig() kconfig.KeyValueMap {
	return popts.kconfig
}

// PackKernelDbg returns return whether to package the debug kernel.
func (popts *PackOptions) KernelDbg() string {
	return popts.kernelDbg
}

// KernelVersion returns the version of the kernel
func (popts *PackOptions) KernelVersion() string {
	return popts.kernelVersion
}

// Name returns the name of the package.
func (popts *PackOptions) Name() string {
	return popts.name
}

// Output returns the location of the package.
func (popts *PackOptions) Output() string {
	return popts.output
}

// Labels returns the labels to be added to the package.
func (popts *PackOptions) Labels() map[string]string {
	return popts.labels
}

// MergeStrategy ...
func (popts *PackOptions) MergeStrategy() MergeStrategy {
	return popts.mergeStrategy
}

// PackOption is an option function which is used to modify PackOptions.
type PackOption func(*PackOptions)

// PackAppSourceFiles marks to include application source files
func PackAppSourceFiles(pack bool) PackOption {
	return func(popts *PackOptions) {
		popts.appSourceFiles = pack
	}
}

// PackArchitecture sets the architecture of the package.
func PackArchitecture(architecture arch.Architecture) PackOption {
	return func(popts *PackOptions) {
		popts.architecture = architecture
	}
}

// PackPlatform sets the platform of the package.
func PackPlatform(platform plat.Platform) PackOption {
	return func(popts *PackOptions) {
		popts.platform = platform
	}
}

// PackArgs sets the arguments to be passed to the application.
func PackArgs(args ...string) PackOption {
	return func(popts *PackOptions) {
		popts.args = args
	}
}

// PackKConfig marks to include the kconfig `.config` file into the package.
func PackKConfig(kcfg kconfig.KeyValueMap) PackOption {
	return func(popts *PackOptions) {
		popts.kconfig = kcfg
	}
}

// PackKernel includes the kernel in the package.
func PackKernel(kernel string) PackOption {
	return func(popts *PackOptions) {
		popts.kernel = kernel
	}
}

// PackInitrd includes the provided path to an initrd file in the package.
func PackInitrd(rootfs initrd.Initrd) PackOption {
	return func(popts *PackOptions) {
		popts.initrd = rootfs
	}
}

// PackRoms includes auxiliary read-only memory blobs in the package.
func PackRoms(roms ...string) PackOption {
	return func(popts *PackOptions) {
		popts.roms = roms
	}
}

// PackKernelDbg includes the debug kernel in the package.
func PackKernelDbg(kernelDbg string) PackOption {
	return func(popts *PackOptions) {
		popts.kernelDbg = kernelDbg
	}
}

// PackWithKernelVersion sets the version of the Unikraft core.
func PackWithKernelVersion(version string) PackOption {
	return func(popts *PackOptions) {
		popts.kernelVersion = version
	}
}

// PackName sets the name of the package.
func PackName(name string) PackOption {
	return func(popts *PackOptions) {
		popts.name = name
	}
}

// PackOutput sets the location of the output artifact package.
func PackOutput(output string) PackOption {
	return func(popts *PackOptions) {
		popts.output = output
	}
}

// PackMergeStrategy sets the mechanism to use when an existing package of the
// same name exists.
func PackMergeStrategy(strategy MergeStrategy) PackOption {
	return func(popts *PackOptions) {
		popts.mergeStrategy = strategy
	}
}

// PackWithEnv adds the environment variables to the package.
func PackWithEnvs(envs []string) PackOption {
	return func(popts *PackOptions) {
		popts.env = envs
	}
}

// PackLabels adds the labels to the package.
func PackLabels(labels map[string]string) PackOption {
	return func(popts *PackOptions) {
		popts.labels = labels
	}
}
