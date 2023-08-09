// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package pkg

import (
	"context"
	"fmt"
	"os"
	"strings"

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
	"kraftkit.sh/unikraft/component"
	"kraftkit.sh/unikraft/target"

	"kraftkit.sh/cmd/kraft/pkg/list"
	"kraftkit.sh/cmd/kraft/pkg/pull"
	"kraftkit.sh/cmd/kraft/pkg/push"
	"kraftkit.sh/cmd/kraft/pkg/source"
	"kraftkit.sh/cmd/kraft/pkg/unsource"
	"kraftkit.sh/cmd/kraft/pkg/update"
)

type Pkg struct {
	Architecture string   `local:"true" long:"arch" short:"m" usage:"Filter the creation of the package by architecture of known targets"`
	Args         string   `local:"true" long:"args" short:"a" usage:"Pass arguments that will be part of the running kernel's command line"`
	Dbg          bool     `local:"true" long:"dbg" usage:"Package the debuggable (symbolic) kernel image instead of the stripped image"`
	Force        bool     `local:"true" long:"force-format" usage:"Force the use of a packaging handler format"`
	Format       string   `local:"true" long:"as" short:"M" usage:"Force the packaging despite possible conflicts" default:"auto"`
	Initrd       string   `local:"true" long:"initrd" short:"i" usage:"Path to init ramdisk to bundle within the package (passing a path will automatically generate a CPIO image)"`
	Kernel       string   `local:"true" long:"kernel" short:"k" usage:"Override the path to the unikernel image"`
	Kraftfile    string   `long:"kraftfile" usage:"Set an alternative path of the Kraftfile"`
	Name         string   `local:"true" long:"name" short:"n" usage:"Specify the name of the package"`
	Output       string   `local:"true" long:"output" short:"o" usage:"Save the package at the following output"`
	Platform     string   `local:"true" long:"plat" short:"p" usage:"Filter the creation of the package by platform of known targets"`
	Targets      []string `local:"true" long:"target" short:"t" usage:"Package a particular known target"`
	WithKConfig  bool     `local:"true" long:"with-kconfig" usage:"Include the target .config"`
}

func New() *cobra.Command {
	cmd, err := cmdfactory.New(&Pkg{}, cobra.Command{
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
		`, "`"),
		Example: heredoc.Doc(`
			# Package a project as an OCI archive and embed the target's KConfig.
			$ kraft pkg --as oci --name unikraft.org/nginx:latest --with-kconfig

			# Package a project with a given target name
			$ kraft pkg --as oci --name unikraft.org/nginx:latest --target my_target

			# Package a project with a given platform and architecture
			$ kraft pkg --as oci --name unikraft.org/nginx:latest --plat qemu --arch x86_64

			# Package a project with multiple targets
			$ kraft pkg --as oci --name unikraft.org/nginx:latest --target my_target --target my_other_target
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
	if (len(opts.Architecture) > 0 || len(opts.Platform) > 0) && len(opts.Targets) > 0 {
		return fmt.Errorf("the `--arch` and `--plat` options are not supported in addition to `--target`")
	}

	ctx, err := packmanager.WithDefaultUmbrellaManagerInContext(cmd.Context())
	if err != nil {
		return err
	}

	cmd.SetContext(ctx)

	opts.Platform = platform.PlatformByName(opts.Platform).String()

	return nil
}

func matchOption(targets []string, option string) bool {
	for _, target := range targets {
		name := target
		if strings.Contains(target, "/") {
			name = string(platform.PlatformByName(strings.Split(target, "/")[0])) +
				"/" +
				strings.Split(target, "/")[1]
		}
		if name == option {
			return true
		}
	}

	return false
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

	popts := []app.ProjectOption{
		app.WithProjectWorkdir(workdir),
	}

	if len(opts.Kraftfile) > 0 {
		popts = append(popts, app.WithProjectKraftfile(opts.Kraftfile))
	} else {
		popts = append(popts, app.WithProjectDefaultKraftfiles())
	}

	// Interpret the project directory
	project, err := app.NewProjectFromOptions(ctx, popts...)
	if err != nil {
		return err
	}

	var tree []*processtree.ProcessTreeItem
	var targets []target.Target
	var packageOpts []packmanager.PackOption

	parallel := !config.G[config.KraftKit](ctx).NoParallel
	norender := log.LoggerTypeFromString(config.G[config.KraftKit](ctx).Log.Type) != log.FANCY

	// Generate a package with all matching requested targets
	for _, targ := range project.Targets() {
		// See: https://github.com/golang/go/wiki/CommonMistakes#using-reference-to-loop-iterator-variable
		targ := targ

		switch true {
		case
			// If no arguments are supplied
			len(opts.Targets) == 0 &&
				len(opts.Architecture) == 0 &&
				len(opts.Platform) == 0,

			// If the --target flag is supplied and the target name match
			len(opts.Targets) > 0 &&
				matchOption(opts.Targets, targ.Name()),

			// If the --target flag is supplied and the target is a plat/arch combo
			len(opts.Targets) > 0 &&
				matchOption(opts.Targets, target.TargetPlatArchName(targ)),

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

			cmdShellArgs, err := shellwords.Parse(opts.Args)
			if err != nil {
				return err
			}

			packageOpts = append(packageOpts,
				packmanager.PackArgs(cmdShellArgs...),
				packmanager.PackInitrd(opts.Initrd),
				packmanager.PackKConfig(opts.WithKConfig),
				packmanager.PackName(opts.Name),
				packmanager.PackOutput(opts.Output),
			)

			if ukversion, ok := targ.KConfig().Get(unikraft.UK_FULLVERSION); ok {
				packageOpts = append(packageOpts,
					packmanager.PackWithKernelVersion(ukversion.Value),
				)
			}

			targets = append(targets, targ)

		default:
			continue
		}
	}

	var components []component.Component
	var targetNames string

	// Convert targets from []target.Target to []component.Component
	for _, targ := range targets {
		components = append(components, targ)
		targetNames += targ.Name() + " "
	}

	tree = append(tree, processtree.NewProcessTreeItem(
		opts.Output,
		targetNames,
		func(ctx context.Context) error {
			pm := packmanager.G(ctx)

			// Switch the package manager the desired format for this target
			if opts.Format != "auto" {
				pm, err = pm.From(pack.PackageFormat(opts.Format))
				if err != nil {
					return err
				}
			}

			if _, err := pm.Pack(ctx, components, packageOpts...); err != nil {
				return err
			}

			return nil
		},
	))

	if len(tree) == 0 {
		switch true {
		case len(opts.Targets) > 0:
			return fmt.Errorf("no matching targets found for: %s", opts.Targets)
		case len(opts.Architecture) > 0 && len(opts.Platform) == 0:
			return fmt.Errorf("no matching targets found for architecture: %s", opts.Architecture)
		case len(opts.Architecture) == 0 && len(opts.Platform) > 0:
			return fmt.Errorf("no matching targets found for platform: %s", opts.Platform)
		default:
			return fmt.Errorf("no matching targets found for: %s/%s", opts.Platform, opts.Architecture)
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
