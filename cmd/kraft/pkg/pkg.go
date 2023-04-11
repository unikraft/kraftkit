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
	"kraftkit.sh/pack"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/log"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/tui/processtree"
	"kraftkit.sh/unikraft/app"

	"kraftkit.sh/cmd/kraft/pkg/list"
	"kraftkit.sh/cmd/kraft/pkg/pull"
	"kraftkit.sh/cmd/kraft/pkg/source"
	"kraftkit.sh/cmd/kraft/pkg/unsource"
	"kraftkit.sh/cmd/kraft/pkg/update"
)

type Pkg struct {
	Architecture string   `local:"true" long:"arch" short:"m" usage:"Filter the creation of the package by architecture of known targets"`
	Dbg          bool     `local:"true" long:"dbg" usage:"Package the debuggable (symbolic) kernel image instead of the stripped image"`
	Force        bool     `local:"true" long:"force-format" usage:"Force the use of a packaging handler format"`
	Format       string   `local:"true" long:"as" short:"M" usage:"Force the packaging despite possible conflicts" default:"auto"`
	Initrd       string   `local:"true" long:"initrd" short:"i" usage:"Path to init ramdisk to bundle within the package (passing a path will automatically generate a CPIO image)"`
	Kernel       string   `local:"true" long:"kernel" short:"k" usage:"Override the path to the unikernel image"`
	Name         string   `local:"true" long:"name" short:"n" usage:"Specify the name of the package"`
	Output       string   `local:"true" long:"output" short:"o" usage:"Save the package at the following output"`
	Platform     string   `local:"true" long:"plat" short:"p" usage:"Filter the creation of the package by platform of known targets"`
	Target       string   `local:"true" long:"target" short:"t" usage:"Package a particular known target"`
	Volumes      []string `local:"true" long:"volume" short:"v" usage:"Additional volumes to bundle within the package"`
	WithKConfig  bool     `local:"true" long:"with-kconfig" usage:"Include the target .config"`
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
			cmdfactory.AnnotationHelpGroup: "pkg",
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
		ctx,
		app.WithProjectWorkdir(workdir),
		app.WithProjectDefaultKraftfiles(),
	)
	if err != nil {
		return err
	}

	var tree []*processtree.ProcessTreeItem

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
						packmanager.PackKConfig(opts.WithKConfig),
						packmanager.PackOutput(opts.Output),
						packmanager.PackInitrd(opts.Initrd),
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
