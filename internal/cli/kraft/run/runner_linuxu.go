// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package run

import (
	"context"
	"debug/elf"
	"fmt"
	"os"

	machineapi "kraftkit.sh/api/machine/v1alpha1"
	"kraftkit.sh/config"
	"kraftkit.sh/log"
	"kraftkit.sh/pack"
	"kraftkit.sh/tui/paraprogress"
	"kraftkit.sh/unikraft/runtime"
)

// https://github.com/file/file/blob/FILE5_44/src/readelf.h#L543
const DF_1_PIE elf.DynFlag = 0x08000000 //nolint:stylecheck

// runnerLinuxu is a runner that allows for the instantiation of a Linux
// Userspace binary in "binary compatibility"-mode which in turn utilizes the
// Unikraft ELFLoader application to enable its runtime.  The binary (and
// desired root filesystem) are mounted to this application to enable its
// runtime.  E.g.:
//
//	$ kraft run path/to/bin
type runnerLinuxu struct {
	exePath string
	args    []string
}

// String implements Runner.
func (runner *runnerLinuxu) String() string {
	return fmt.Sprintf("run the Linux userspace binary '%s' in a unikernel and ignore cwd", runner.exePath)
}

// Name implements Runner.
func (runner *runnerLinuxu) Name() string {
	return "linuxu"
}

// Runnable implements Runner.
func (runner *runnerLinuxu) Runnable(ctx context.Context, opts *RunOptions, args ...string) (bool, error) {
	if len(args) == 0 {
		return false, fmt.Errorf("no arguments supplied")
	}

	runner.exePath = args[0]
	runner.args = args[1:]

	fs, err := os.Stat(runner.exePath)
	if err != nil {
		return false, err
	} else if fs.IsDir() {
		return false, fmt.Errorf("first positional argument is a directory: %s", runner.exePath)
	}

	fi, err := os.Open(runner.exePath)
	if err != nil {
		return false, fmt.Errorf("opening file %s: %w", runner.exePath, err)
	}

	defer fi.Close()

	ef, err := elf.NewFile(fi)
	if err != nil {
		return false, fmt.Errorf("reading ELF file %s: %w", runner.exePath, err)
	}

	// Both static and dynamic PIEs have this type.
	if ef.Type != elf.ET_DYN {
		return false, fmt.Errorf("ELF file is shared object")
	}

	// Based on file(1) and elf.(*File).DynString.
	// https://github.com/file/file/blob/FILE5_44/src/readelf.c#L1141-L1147
	// https://cs.opensource.google/go/go/+/refs/tags/go1.20.4:src/debug/elf/file.go;l=1602-1643
	ds := ef.SectionByType(elf.SHT_DYNAMIC)
	if ds == nil {
		return false, fmt.Errorf("ELF file has type %s but no dynamic section", ef.Type)
	}

	d, err := ds.Data()
	if err != nil {
		return false, fmt.Errorf("reading ELF section %s of type %s", ds.Name, ds.Type)
	}

	for len(d) > 0 {
		var t elf.DynTag
		var v uint64
		switch ef.Class {
		case elf.ELFCLASS32:
			t = elf.DynTag(ef.ByteOrder.Uint32(d[0:4]))
			v = uint64(ef.ByteOrder.Uint32(d[4:8]))
			d = d[8:]
		case elf.ELFCLASS64:
			t = elf.DynTag(ef.ByteOrder.Uint64(d[0:8]))
			v = ef.ByteOrder.Uint64(d[8:16])
			d = d[16:]
		}
		if t == elf.DT_FLAGS_1 && elf.DynFlag(v)&DF_1_PIE != 0 {
			return true, nil
		}
	}

	// Fallback
	// This logic originates from Debian's hardening-check devscript.
	// https://salsa.debian.org/debian/devscripts/-/blob/v2.23.3/scripts/hardening-check.pl?ref_type=tags#L293-303
	for _, p := range ef.Progs {
		if p.Type == elf.PT_PHDR {
			return true, nil
		}
	}

	return false, fmt.Errorf("file is not ELF executable")
}

// Prepare implements Runner.
func (runner *runnerLinuxu) Prepare(ctx context.Context, opts *RunOptions, machine *machineapi.Machine, args ...string) error {
	if opts.Platform == "" {
		opts.Platform = "qemu"
	}

	loader, err := runtime.NewRuntime(ctx, opts.Runtime,
		runtime.WithPlatform(opts.Platform),
		runtime.WithArchitecture(opts.Architecture),
	)
	if err != nil {
		return err
	}

	// Create a temporary directory where the image can be stored
	dir, err := os.MkdirTemp("", "")
	if err != nil {
		return err
	}

	// TODO(nderjung): For now, there is no proper support for using the --rootfs
	// flag since we set Linux userspace application as the initramfs.  In the
	// future when we have better volume/filesystem support, we can re-think its
	// use here.
	if len(opts.Rootfs) > 0 {
		log.G(ctx).Warnf("ignoring --rootfs in favour of Linux userspace binary")
	}

	paramodel, err := paraprogress.NewParaProgress(
		ctx,
		[]*paraprogress.Process{paraprogress.NewProcess(
			fmt.Sprintf("pulling %s", loader.Name()),
			func(ctx context.Context, w func(progress float64)) error {
				popts := []pack.PullOption{
					pack.WithPullWorkdir(dir),
				}
				if log.LoggerTypeFromString(config.G[config.KraftKit](ctx).Log.Type) == log.FANCY {
					popts = append(popts, pack.WithPullProgressFunc(w))
				}

				return loader.Pull(
					ctx,
					popts...,
				)
			},
		)},
		paraprogress.IsParallel(false),
		paraprogress.WithRenderer(
			log.LoggerTypeFromString(config.G[config.KraftKit](ctx).Log.Type) != log.FANCY,
		),
		paraprogress.WithFailFast(true),
	)
	if err != nil {
		return err
	}

	if err := paramodel.Start(); err != nil {
		return err
	}

	machine.Spec.Architecture = loader.Architecture().Name()
	machine.Spec.Platform = loader.Platform().Name()
	machine.Spec.Kernel = fmt.Sprintf("elfloader://%s:%s", loader.Name(), loader.Version())
	machine.Spec.ApplicationArgs = runner.args
	machine.Status.InitrdPath = runner.exePath

	// Use the symbolic debuggable kernel image?
	if opts.WithKernelDbg {
		machine.Status.KernelPath = loader.KernelDbg()
	} else {
		machine.Status.KernelPath = loader.Kernel()
	}

	return nil
}
