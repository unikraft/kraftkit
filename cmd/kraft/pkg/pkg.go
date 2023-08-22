// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package pkg

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/MakeNowJust/heredoc"
	"github.com/mattn/go-shellwords"
	"github.com/spf13/cobra"

	"kraftkit.sh/config"
	"kraftkit.sh/machine/platform"
	"kraftkit.sh/pack"
	"kraftkit.sh/unikraft"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/log"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/tui/processtree"
	"kraftkit.sh/unikraft/app"

	"kraftkit.sh/cmd/kraft/pkg/list"
	"kraftkit.sh/cmd/kraft/pkg/pull"
	"kraftkit.sh/cmd/kraft/pkg/push"
	"kraftkit.sh/cmd/kraft/pkg/source"
	"kraftkit.sh/cmd/kraft/pkg/unsource"
	"kraftkit.sh/cmd/kraft/pkg/update"
)

type Pkg struct {
	Architecture string `local:"true" long:"arch" short:"m" usage:"Filter the creation of the package by architecture of known targets"`
	Args         string `local:"true" long:"args" short:"a" usage:"Pass arguments that will be part of the running kernel's command line"`
	Dbg          bool   `local:"true" long:"dbg" usage:"Package the debuggable (symbolic) kernel image instead of the stripped image"`
	Force        bool   `local:"true" long:"force-format" usage:"Force the use of a packaging handler format"`
	Format       string `local:"true" long:"as" short:"M" usage:"Force the packaging despite possible conflicts" default:"auto"`
	Initrd       string `local:"true" long:"initrd" short:"i" usage:"Path to init ramdisk to bundle within the package (passing a path will automatically generate a CPIO image)"`
	Kernel       string `local:"true" long:"kernel" short:"k" usage:"Override the path to the unikernel image"`
	Kraftfile    string `long:"kraftfile" usage:"Set an alternative path of the Kraftfile"`
	Name         string `local:"true" long:"name" short:"n" usage:"Specify the name of the package"`
	Output       string `local:"true" long:"output" short:"o" usage:"Save the package at the following output"`
	Platform     string `local:"true" long:"plat" short:"p" usage:"Filter the creation of the package by platform of known targets"`
	Target       string `local:"true" long:"target" short:"t" usage:"Package a particular known target"`
	WithKConfig  bool   `local:"true" long:"with-kconfig" usage:"Include the target .config"`
}

func New() *cobra.Command {
	cmd, err := cmdfactory.New(&Pkg{}, cobra.Command{
		Short: "Package and distribute Unikraft unikernels and their dependencies",
		Use:   "pkg [FLAGS] [SUBCOMMAND|DIR|PACKAGE]",
		Args:  cmdfactory.MaxDirArgs(1),
		Long: heredoc.Docf(`
			Package and distribute Unikraft unikernels and their dependencies.

			With %[1]skraft pkg%[1]s you are able to turn output artifacts from %[1]skraft build%[1]s
			into a distributable archive ready for deployment.  At the same time,
			%[1]skraft pkg%[1]s allows you to manage these archives: pulling, pushing, or
			adding them to a project.

			The default behaviour of %[1]skraft pkg%[1]s is to package a project.  Given no
			arguments, you will be guided through interactive mode.
		`, "`"),
		Example: heredoc.Doc(`
			# Package a project as an OCI archive and embed the target's KConfig.
			$ kraft pkg --as oci --name unikraft.org/nginx:latest --with-kconfig

			# Package a project as an OCI archive and embed an initrd and arguments.
			$ kraft pkg --as oci --name unikraft.org/nginx:latest --initrd ./fs0 --args "-h"

			# Package an existing OCI archive and embed a different initrd.
			$ kraft pkg --as oci --initrd ./fs0 unikraft.org/nginx:latest
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "pkg",
		},
	})
	if err != nil {
		panic(err)
	}

	cmd.AddCommand(list.New())
	cmd.AddCommand(pull.New())
	cmd.AddCommand(push.New())
	cmd.AddCommand(source.New())
	cmd.AddCommand(unsource.New())
	cmd.AddCommand(update.New())

	return cmd
}

func (opts *Pkg) Pre(cmd *cobra.Command, _ []string) error {
	if (len(opts.Architecture) > 0 || len(opts.Platform) > 0) && len(opts.Target) > 0 {
		return fmt.Errorf("the `--arch` and `--plat` options are not supported in addition to `--target`")
	}

	ctx := cmd.Context()
	pm, err := packmanager.NewUmbrellaManager(ctx)
	if err != nil {
		return err
	}

	cmd.SetContext(packmanager.WithPackageManager(ctx, pm))

	opts.Platform = platform.PlatformByName(opts.Platform).String()

	return nil
}

func (opts *Pkg) Run(cmd *cobra.Command, args []string) error {
	var err error
	var source string
	var sourcePackage pack.Package
	popts := []app.ProjectOption{}

	ctx := cmd.Context()
	var pmananger packmanager.PackageManager
	if opts.Format != "auto" {
		pmananger = packmanager.PackageManagers()[pack.PackageFormat(opts.Format)]
		if pmananger == nil {
			return errors.New("invalid package format specified")
		}
	} else {
		pmananger = packmanager.G(ctx)
	}

	if len(args) == 0 {
		source, err = os.Getwd()
		if err != nil {
			return err
		}
		popts = append(popts, app.WithProjectWorkdir(source))

		fmt.Println("Empty ARGS")
	} else {
		source = args[0]

		fmt.Println(source)

		// The provided argument is either a directory or a Kraftfile
		if fi, err := os.Stat(args[0]); err == nil && fi.IsDir() {
			popts = append(popts, app.WithProjectWorkdir(source))
		} else {
			fmt.Println(source)
			if pm, compatible, err := pmananger.IsCompatible(ctx, source); err == nil && compatible {
				packages, err := pm.Catalog(ctx,
					packmanager.WithCache(true),
					packmanager.WithName(source),
				)
				if err != nil {
					return err
				}

				if len(packages) == 0 {
					return fmt.Errorf("no package found for %s", source)
				} else if len(packages) > 1 {
					return fmt.Errorf("multiple packages found for %s", source)
				}

				sourcePackage = packages[0]
			}
		}
	}

	var tree []*processtree.ProcessTreeItem
	var project app.Application

	if sourcePackage != nil {
		cmdShellArgs, err := shellwords.Parse(opts.Args)
		if err != nil {
			return err
		}

		tree = append(tree, processtree.NewProcessTreeItem(
			sourcePackage.Name(),
			sourcePackage.Version(),
			func(ctx context.Context) error {
				var err error
				pm := packmanager.G(ctx)

				// Switch the package manager the desired format for this target
				if sourcePackage.Format().String() != "auto" {
					pm, err = pm.From(sourcePackage.Format())
					if err != nil {
						return err
					}
				}

				popts := []packmanager.PackOption{
					packmanager.PackArgs(cmdShellArgs...),
					packmanager.PackInitrd(opts.Initrd),
					packmanager.PackKConfig(opts.WithKConfig),
					packmanager.PackName(opts.Name),
					packmanager.PackOutput(opts.Output),
				}

				if _, err := pm.Pack(ctx, nil, popts...); err != nil {
					return err
				}

				return nil
			},
		))
	} else {
		if len(opts.Kraftfile) > 0 {
			popts = append(popts, app.WithProjectKraftfile(opts.Kraftfile))
		} else {
			popts = append(popts, app.WithProjectDefaultKraftfiles())
		}

		// Interpret the project directory
		project, err = app.NewProjectFromOptions(ctx, popts...)
		if err != nil {
			return err
		}
	}

	parallel := !config.G[config.KraftKit](ctx).NoParallel
	norender := log.LoggerTypeFromString(config.G[config.KraftKit](ctx).Log.Type) != log.FANCY

	// Generate a package for every matching requested target
	for _, targ := range project.Targets() {
		// See: https://github.com/golang/go/wiki/CommonMistakes#using-reference-to-loop-iterator-variable
		targ := targ

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
				targ.Architecture().Name() == opts.Architecture,

			// If only the --plat flag is supplied and the target's platform matches
			len(opts.Platform) > 0 &&
				len(opts.Architecture) == 0 &&
				targ.Platform().Name() == opts.Platform,

			// If both the --arch and --plat flag are supplied and match the target
			len(opts.Platform) > 0 &&
				len(opts.Architecture) > 0 &&
				targ.Architecture().Name() == opts.Architecture &&
				targ.Platform().Name() == opts.Platform:

			var format pack.PackageFormat
			name := "packaging " + targ.Name()
			if opts.Format != "auto" {
				format = pack.PackageFormat(opts.Format)
			} else if targ.Format().String() != "" {
				format = targ.Format()
			}
			if format.String() != "auto" {
				name += " (" + format.String() + ")"
			}

			cmdShellArgs, err := shellwords.Parse(opts.Args)
			if err != nil {
				return err
			}

			tree = append(tree, processtree.NewProcessTreeItem(
				name,
				targ.Architecture().Name()+"/"+targ.Platform().Name(),
				func(ctx context.Context) error {
					var err error
					pm := packmanager.G(ctx)

					// Switch the package manager the desired format for this target
					if format != "auto" {
						pm, err = pm.From(format)
						if err != nil {
							return err
						}
					}

					popts := []packmanager.PackOption{
						packmanager.PackArgs(cmdShellArgs...),
						packmanager.PackInitrd(opts.Initrd),
						packmanager.PackKConfig(opts.WithKConfig),
						packmanager.PackName(opts.Name),
						packmanager.PackOutput(opts.Output),
					}

					if ukversion, ok := targ.KConfig().Get(unikraft.UK_FULLVERSION); ok {
						popts = append(popts,
							packmanager.PackWithKernelVersion(ukversion.Value),
						)
					}

					if _, err := pm.Pack(ctx, targ, popts...); err != nil {
						return err
					}

					return nil
				},
			))

		default:
			continue
		}
	}

	model, err := processtree.NewProcessTree(
		ctx,
		[]processtree.ProcessTreeOption{
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
