// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package pkg

import (
	"context"
	"fmt"
	"os"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"kraftkit.sh/config"
	"kraftkit.sh/unikraft"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/initrd"
	"kraftkit.sh/log"
	"kraftkit.sh/pack"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/tui/processtree"
	"kraftkit.sh/unikraft/app"
	"kraftkit.sh/unikraft/target"

	"kraftkit.sh/cmd/kraft/pkg/list"
	"kraftkit.sh/cmd/kraft/pkg/pull"
	"kraftkit.sh/cmd/kraft/pkg/source"
	"kraftkit.sh/cmd/kraft/pkg/unsource"
	"kraftkit.sh/cmd/kraft/pkg/update"
)

type Pkg struct {
	Architecture string   `local:"true" long:"arch" short:"m" usage:"Filter the creation of the package by architecture of known targets"`
	DotConfig    string   `local:"true" long:"config" short:"c" usage:"Override the path to the KConfig .config file"`
	Force        bool     `local:"true" long:"force-format" usage:"Force the use of a packaging handler format"`
	Format       string   `local:"true" long:"as" short:"M" usage:"Force the packaging despite possible conflicts" default:"auto"`
	Initrd       string   `local:"true" long:"initrd" short:"i" usage:"Path to init ramdisk to bundle within the package (passing a path will automatically generate a CPIO image)"`
	Kernel       string   `local:"true" long:"kernel" short:"k" usage:"Override the path to the unikernel image"`
	KernelDbg    bool     `local:"true" long:"dbg" usage:"Package the debuggable (symbolic) kernel image instead of the stripped image"`
	Name         string   `local:"true" long:"name" short:"n" usage:"Specify the name of the package"`
	Output       string   `local:"true" long:"output" short:"o" usage:"Save the package at the following output"`
	Platform     string   `local:"true" long:"plat" short:"p" usage:"Filter the creation of the package by platform of known targets"`
	Target       string   `local:"true" long:"target" short:"t" usage:"Package a particular known target"`
	Volumes      []string `local:"true" long:"volume" short:"v" usage:"Additional volumes to bundle within the package"`
	WithDbg      bool     `local:"true" long:"with-dbg" usage:"In addition to the stripped kernel, include the debug image"`
}

func New() *cobra.Command {
	cmd := cmdfactory.New(&Pkg{}, cobra.Command{
		Short: "Package and distribute Unikraft unikernels and their dependencies",
		Use:   "pkg [FLAGS] [SUBCOMMAND|DIR]",
		Args:  cmdfactory.MaxDirArgs(1),
		Long: heredoc.Docf(`
			Package and distribute Unikraft unikernels and their dependencies.

			With %[1]skraft pkg%[1]s you are able to turn output artifacts from %[1]skraft build%[1]s
			into a distributable archive ready for deployment.  At the same time,
			%[1]skraft pkg%[1]s allows you to manage these archives: pulling, pushing, or
			adding them to a project.

			The default behaviour of %[1]skraft pkg%[1]s is to package a project.  Given no
			arguments, you will be guided through interactive mode.

			For initram and disk images, passing in a directory as the argument will
			result automatically packaging that directory into the requested format.
			Separating the input with a %[1]s:%[1]s delimiter allows you to set the
			output that of the artifact.
		`, "`"),
		Example: heredoc.Doc(`
			# Package the current Unikraft project (cwd)
			$ kraft pkg

			# Package path to a Unikraft project
			$ kraft pkg path/to/application

			# Package with an additional initramfs
			$ kraft pkg --initrd ./root-fs .

			# Same as above but also save the resulting CPIO artifact locally
			$ kraft pkg --initrd ./root-fs:./root-fs.cpio .`),
		Annotations: map[string]string{
			"help:group": "pkg",
		},
	})

	cmd.AddCommand(list.New())
	cmd.AddCommand(pull.New())
	cmd.AddCommand(source.New())
	cmd.AddCommand(unsource.New())
	cmd.AddCommand(update.New())

	return cmd
}

func (opts *Pkg) Pre(cmd *cobra.Command, args []string) error {
	if (len(opts.Architecture) > 0 || len(opts.Platform) > 0) && len(opts.Target) > 0 {
		return fmt.Errorf("the `--arch` and `--plat` options are not supported in addition to `--target`")
	}

	return nil
}

func (opts *Pkg) Run(cmd *cobra.Command, args []string) error {
	var err error
	var workdir string

	if len(args) == 0 {
		workdir, err = os.Getwd()
		if err != nil {
			return err
		}
	} else {
		workdir = args[0]
	}

	ctx := cmd.Context()

	// Interpret the project directory
	project, err := app.NewProjectFromOptions(
		app.WithProjectWorkdir(workdir),
		app.WithProjectDefaultKraftfiles(),
	)
	if err != nil {
		return err
	}

	var packages []pack.Package

	// Generate a package for every matching requested target
	targets, err := project.Targets()
	if err != nil {
		return err
	}
	for _, targ := range targets {
		switch true {
		case
			// If no arguments are supplied
			len(opts.Target) == 0 &&
				len(opts.Architecture) == 0 &&
				len(opts.Platform) == 0,

			// If the --target flag is supplied and the target name match
			len(opts.Target) > 0 &&
				targ.Name() == opts.Target,

			// If only the --arch flag is supplied and the target's arch matches
			len(opts.Architecture) > 0 &&
				len(opts.Platform) == 0 &&
				targ.Architecture.Name() == opts.Architecture,

			// If only the --plat flag is supplied and the target's platform matches
			len(opts.Platform) > 0 &&
				len(opts.Architecture) == 0 &&
				targ.Platform.Name() == opts.Platform,

			// If both the --arch and --plat flag are supplied and match the target
			len(opts.Platform) > 0 &&
				len(opts.Architecture) > 0 &&
				targ.Architecture.Name() == opts.Architecture &&
				targ.Platform.Name() == opts.Platform:

			packs, err := initAppPackage(ctx, project, targ, opts)
			if err != nil {
				return fmt.Errorf("could not create package: %s", err)
			}

			packages = append(packages, packs...)

		default:
			continue
		}
	}

	if len(packages) == 0 {
		log.G(ctx).Info("nothing to package")
		return nil
	}

	parallel := !config.G(ctx).NoParallel
	norender := log.LoggerTypeFromString(config.G(ctx).Log.Type) != log.FANCY

	var tree []*processtree.ProcessTreeItem
	for _, p := range packages {
		// See: https://github.com/golang/go/wiki/CommonMistakes#using-reference-to-loop-iterator-variable
		p := p

		tree = append(tree, processtree.NewProcessTreeItem(
			"Packaging "+p.CanonicalName(),
			p.Options().ArchPlatString(),
			func(ctx context.Context) error {
				return p.Pack(ctx)
			},
		))
	}

	model, err := processtree.NewProcessTree(
		ctx,
		[]processtree.ProcessTreeOption{
			processtree.WithVerb("Packaging..."),
			processtree.IsParallel(parallel),
			processtree.WithRenderer(norender),
		},
		tree...,
	)
	if err != nil {
		return err
	}

	return model.Start()
}

func initAppPackage(ctx context.Context,
	project *app.ApplicationConfig,
	targ target.TargetConfig,
	opts *Pkg,
) ([]pack.Package, error) {
	var err error
	log.G(ctx).Tracef("initializing package")

	// Path to the kernel image
	kernel := opts.Kernel
	if len(kernel) == 0 {
		kernel = targ.Kernel
	}

	// Prefer the debuggable (symbolic) kernel as the main kernel
	if opts.KernelDbg && !opts.WithDbg {
		kernel = targ.KernelDbg
	}

	name := opts.Name

	targets, err := project.Targets()
	if err != nil {
		return nil, err
	}

	// This is a built in naming convention format, which for now allows us to
	// differentiate between different targets.  This should be further discussed
	// the community if this is the best approach.  This can ultimately be
	// overwritten using the --tag flag.
	if len(name) == 0 && len(targets) == 1 {
		name = project.Name()
	} else if len(name) == 0 {
		name = project.Name() + "-" + targ.Name()
	}

	version := project.Version()
	if len(version) == 0 {
		version = "latest"
	}

	extraPackOpts := []pack.PackageOption{
		pack.WithName(name),
		pack.WithVersion(version),
		pack.WithType(unikraft.ComponentTypeApp),
		pack.WithArchitecture(targ.Architecture.Name()),
		pack.WithPlatform(targ.Platform.Name()),
		pack.WithKernel(kernel),
		pack.WithWorkdir(project.WorkingDir()),
		pack.WithLocalLocation(opts.Output, opts.Force),
	}

	// Options for the initramfs if set
	initrdConfig := targ.Initrd
	if len(opts.Initrd) > 0 {
		initrdConfig, err = initrd.ParseInitrdConfig(project.WorkingDir(), opts.Initrd)
		if err != nil {
			return nil, fmt.Errorf("could not parse --initrd flag with value %s: %s", opts.Initrd, err)
		}
	}

	// Warn if potentially missing configuration options
	// if initrdConfig != nil && unikraft.EnabledInitramfs()
	extraPackOpts = append(extraPackOpts,
		pack.WithInitrdConfig(initrdConfig),
	)

	packOpts, err := pack.NewPackageOptions(extraPackOpts...)
	if err != nil {
		return nil, fmt.Errorf("could not prepare package for target: %s: %v", targ.Name(), err)
	}

	pm := packmanager.G(ctx)

	// Switch the package manager the desired format for this target
	if len(targ.Format) > 0 && targ.Format != "auto" {
		if pm.Format() == "umbrella" {
			pm, err = pm.From(targ.Format)
			if err != nil {
				return nil, err
			}

			// Skip this target as we cannot package it
		} else if pm.Format() != targ.Format && !opts.Force {
			log.G(ctx).Warnf("skipping %s target %s", targ.Format, targ.Name())
			return nil, nil
		}
	}

	pack, err := pm.NewPackageFromOptions(ctx, packOpts)
	if err != nil {
		return nil, fmt.Errorf("could not initialize package: %s", err)
	}

	return pack, nil
}
