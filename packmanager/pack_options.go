// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package packmanager

// PackOptions contains the list of options which can be set when packaging a
// component.
type PackOptions struct {
	appSourceFiles                   bool
	args                             []string
	initrd                           string
	kconfig                          bool
	kernelDbg                        bool
	kernelLibraryIntermediateObjects bool
	kernelLibraryObjects             bool
	kernelSourceFiles                bool
	kernelVersion                    string
	name                             string
	output                           string
	mergeStrategy                    MergeStrategy
}

// NewPackOptions returns an instantiated *NewPackOptions with default
// configuration options values.
func NewPackOptions() *PackOptions {
	return &PackOptions{
		mergeStrategy: StrategyExit,
	}
}

// PackAppSourceFiles returns whether the application source files should be
// packaged.
func (popts *PackOptions) PackAppSourceFiles() bool {
	return popts.appSourceFiles
}

// Args returns the arguments to pass to the kernel.
func (popts *PackOptions) Args() []string {
	return popts.args
}

// Initrd returns the path of the initrd file that should be packaged.
func (popts *PackOptions) Initrd() string {
	return popts.initrd
}

// PackKConfig returns whether the .config file should be packaged.
func (popts *PackOptions) PackKConfig() bool {
	return popts.kconfig
}

// PackKernelDbg returns return whether to package the debug kernel.
func (popts *PackOptions) KernelDbg() bool {
	return popts.kernelDbg
}

// PackKernelLibraryIntermediateObjects returns whether to package intermediate
// kernel library object files.
func (popts *PackOptions) PackKernelLibraryIntermediateObjects() bool {
	return popts.kernelLibraryIntermediateObjects
}

// PackKernelLibraryObjects returns whether to package kernel library objects.
func (popts *PackOptions) PackKernelLibraryObjects() bool {
	return popts.kernelLibraryObjects
}

// PackKernelSourceFiles returns the whether to package kernel source files.
func (popts *PackOptions) PackKernelSourceFiles() bool {
	return popts.kernelSourceFiles
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

// PackArgs sets the arguments to be passed to the application.
func PackArgs(args ...string) PackOption {
	return func(popts *PackOptions) {
		popts.args = args
	}
}

// PackKConfig marks to include the kconfig `.config` file into the package.
func PackKConfig(kconfig bool) PackOption {
	return func(popts *PackOptions) {
		popts.kconfig = kconfig
	}
}

// PackInitrd includes the provided path to an initrd file in the package.
func PackInitrd(initrd string) PackOption {
	return func(popts *PackOptions) {
		popts.initrd = initrd
	}
}

// PackKernelDbg includes the debug kernel in the package.
func PackKernelDbg(dbg bool) PackOption {
	return func(popts *PackOptions) {
		popts.kernelDbg = dbg
	}
}

// PackKernelLibraryIntermediateObjects marks to include intermediate library
// object files, e.g. libnolibc/errno.o
func PackKernelLibraryIntermediateObjects(pack bool) PackOption {
	return func(popts *PackOptions) {
		popts.kernelSourceFiles = pack
	}
}

// PackKernelLibraryObjects marks to include library object files, e.g. nolibc.o
func PackKernelLibraryObjects(pack bool) PackOption {
	return func(popts *PackOptions) {
		popts.kernelSourceFiles = pack
	}
}

// PackKernelSourceFiles marks that all source files which make up the build
// from the Unikraft build side are to be included.  Including these files will
// enable a reproducible build.
func PackKernelSourceFiles(pack bool) PackOption {
	return func(popts *PackOptions) {
		popts.kernelSourceFiles = pack
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
