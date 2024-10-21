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
	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"

	"kraftkit.sh/config"
	"kraftkit.sh/log"
	"kraftkit.sh/machine/platform"
	"kraftkit.sh/pack"
	"kraftkit.sh/tui/processtree"
	"kraftkit.sh/tui/selection"
	"kraftkit.sh/unikraft/app"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/packmanager"

	"kraftkit.sh/internal/cli/kraft/pkg/info"
	"kraftkit.sh/internal/cli/kraft/pkg/list"
	"kraftkit.sh/internal/cli/kraft/pkg/pull"
	"kraftkit.sh/internal/cli/kraft/pkg/push"
	"kraftkit.sh/internal/cli/kraft/pkg/remove"
	"kraftkit.sh/internal/cli/kraft/pkg/source"
	"kraftkit.sh/internal/cli/kraft/pkg/unsource"
	"kraftkit.sh/internal/cli/kraft/pkg/update"
)

type PkgOptions struct {
	Architecture string                    `local:"true" long:"arch" short:"m" usage:"Filter the creation of the package by architecture of known targets (x86_64/arm64/arm)"`
	Args         []string                  `local:"true" long:"args" short:"a" usage:"Pass arguments that will be part of the running kernel's command line"`
	Compress     bool                      `local:"true" long:"compress" short:"c" usage:"Compress the initrd package (experimental)"`
	Dbg          bool                      `local:"true" long:"dbg" usage:"Package the debuggable (symbolic) kernel image instead of the stripped image"`
	Env          []string                  `local:"true" long:"env" short:"e" usage:"Set environment variables to be packed into the package"`
	Force        bool                      `local:"true" long:"force-format" usage:"Force the use of a packaging handler format"`
	Format       string                    `local:"true" long:"as" short:"M" usage:"Force the packaging despite possible conflicts" default:"oci"`
	Kernel       string                    `local:"true" long:"kernel" short:"k" usage:"Override the path to the unikernel image"`
	Kraftfile    string                    `long:"kraftfile" short:"K" usage:"Set an alternative path of the Kraftfile"`
	Labels       []string                  `local:"true" long:"label" short:"l" usage:"Set labels to be packed into the package (k=v)"`
	Name         string                    `local:"true" long:"name" short:"n" usage:"Specify the name of the package"`
	NoKConfig    bool                      `local:"true" long:"no-kconfig" usage:"Do not include target .config as metadata"`
	NoPull       bool                      `local:"true" long:"no-pull" usage:"Do not pull package dependencies before packaging"`
	Output       string                    `local:"true" long:"output" short:"o" usage:"Save the package at the following output"`
	Platform     string                    `local:"true" long:"plat" short:"p" usage:"Filter the creation of the package by platform of known targets (fc/qemu/xen/cloud)"`
	Project      app.Application           `noattribute:"true"`
	Push         bool                      `local:"true" long:"push" short:"P" usage:"Push the package on if successfully packaged"`
	Rootfs       string                    `local:"true" long:"rootfs" usage:"Specify a path to use as root file system (can be volume or initramfs)"`
	Runtime      string                    `local:"true" long:"runtime" short:"r" usage:"Set the runtime to use for the package"`
	Strategy     packmanager.MergeStrategy `noattribute:"true"`
	Target       string                    `local:"true" long:"target" short:"t" usage:"Package a particular known target"`
	Workdir      string                    `local:"true" long:"workdir" short:"w" usage:"Set an alternative working directory (default is cwd)"`

	packopts []packmanager.PackOption
	pm       packmanager.PackageManager
}

// Pkg a Unikraft project.
func Pkg(ctx context.Context, opts *PkgOptions, args ...string) ([]pack.Package, error) {
	var err error

	if opts == nil {
		opts = &PkgOptions{}
	}

	if opts.Workdir == "" {
		if len(args) == 0 {
			opts.Workdir, err = os.Getwd()
			if err != nil {
				return nil, err
			}
		}
	}

	if len(args) != 0 {
		opts.Workdir = args[0]
	}

	if opts.Name == "" {
		return nil, fmt.Errorf("cannot package without setting --name")
	}

	if (len(opts.Architecture) > 0 || len(opts.Platform) > 0) && len(opts.Target) > 0 {
		return nil, fmt.Errorf("the `--arch` and `--plat` options are not supported in addition to `--target`")
	}

	if config.G[config.KraftKit](ctx).NoPrompt && opts.Strategy == packmanager.StrategyPrompt {
		return nil, fmt.Errorf("cannot mix --strategy=prompt when --no-prompt is enabled in settings")
	}

	opts.Platform = platform.PlatformByName(opts.Platform).String()

	if len(opts.Format) > 0 {
		// Switch the package manager the desired format for this target
		opts.pm, err = packmanager.G(ctx).From(pack.PackageFormat(opts.Format))
		if err != nil {
			return nil, err
		}
	} else {
		opts.pm = packmanager.G(ctx)
	}

	var exists []pack.Package

	paramodel, err := processtree.NewProcessTree(
		ctx,
		[]processtree.ProcessTreeOption{
			processtree.IsParallel(false),
			processtree.WithRenderer(
				log.LoggerTypeFromString(config.G[config.KraftKit](ctx).Log.Type) != log.FANCY,
			),
			processtree.WithFailFast(true),
			processtree.WithHideOnSuccess(true),
		},
		processtree.NewProcessTreeItem(
			fmt.Sprintf("searching for %s", opts.Name),
			"",
			func(ctx context.Context) error {
				exists, err = opts.pm.Catalog(ctx,
					packmanager.WithName(opts.Name),
				)
				return err
			},
		),
	)
	if err != nil {
		return nil, fmt.Errorf("could not start the process tree: %w", err)
	}

	err = paramodel.Start()
	if err != nil {
		return nil, fmt.Errorf("could not wait for image be available: %w", err)
	}

	if err == nil && len(exists) > 0 {
		if opts.Strategy == packmanager.StrategyPrompt {
			strategy, err := selection.Select[packmanager.MergeStrategy](
				fmt.Sprintf("package '%s' already exists: how would you like to proceed?", opts.Name),
				packmanager.MergeStrategies()...,
			)
			if err != nil {
				return nil, err
			}

			opts.Strategy = *strategy
		}

		switch opts.Strategy {
		case packmanager.StrategyAbort:
			return nil, fmt.Errorf("package already exists and merge strategy set to exit on conflict")

		// Set the merge strategy as an option that is then passed to the
		// package manager.
		default:
			opts.packopts = append(opts.packopts,
				packmanager.PackMergeStrategy(opts.Strategy),
			)
		}
	} else {
		opts.packopts = append(opts.packopts,
			packmanager.PackMergeStrategy(packmanager.StrategyMerge),
		)
	}

	var pkgr packager

	packagers := packagers()

	// Iterate through the list of built-in builders which sequentially tests
	// the current context and Kraftfile match specific requirements towards
	// performing a type of build.
	for _, candidate := range packagers {
		log.G(ctx).
			WithField("packager", candidate.String()).
			Trace("checking compatibility")

		capable, err := candidate.Packagable(ctx, opts, args...)
		if capable && err == nil {
			pkgr = candidate
			break
		}

		log.G(ctx).
			WithError(err).
			WithField("packager", candidate.String()).
			Trace("incompatbile")
	}

	if pkgr == nil {
		return nil, fmt.Errorf("could not determine what or how to package from the given context")
	}

	log.G(ctx).WithField("packager", pkgr.String()).Debug("using")

	packs, err := pkgr.Pack(ctx, opts, args...)
	if err != nil {
		return nil, fmt.Errorf("could not package: %w", err)
	}

	if opts.Push {
		var processes []*processtree.ProcessTreeItem

		for _, p := range packs {
			p := p

			processes = append(processes, processtree.NewProcessTreeItem(
				"pushing",
				humanize.Bytes(uint64(p.Size())),
				func(ctx context.Context) error {
					return p.Push(ctx)
				},
			))
		}
		model, err := processtree.NewProcessTree(
			ctx,
			[]processtree.ProcessTreeOption{
				processtree.IsParallel(!config.G[config.KraftKit](ctx).NoParallel),
				processtree.WithRenderer(log.LoggerTypeFromString(config.G[config.KraftKit](ctx).Log.Type) != log.FANCY),
				processtree.WithFailFast(true),
			},
			processes...,
		)
		if err != nil {
			return packs, err
		}

		if err := model.Start(); err != nil {
			return packs, err
		}
	}

	return packs, nil
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&PkgOptions{}, cobra.Command{
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
			$ kraft pkg --as oci --name unikraft.org/nginx:latest	
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "pkg",
		},
	})
	if err != nil {
		panic(err)
	}

	cmd.AddCommand(info.New())
	cmd.AddCommand(list.NewCmd())
	cmd.AddCommand(pull.NewCmd())
	cmd.AddCommand(push.NewCmd())
	cmd.AddCommand(remove.NewCmd())
	cmd.AddCommand(source.NewCmd())
	cmd.AddCommand(unsource.NewCmd())
	cmd.AddCommand(update.NewCmd())

	cmd.Flags().Var(
		cmdfactory.NewEnumFlag[packmanager.MergeStrategy](
			append(packmanager.MergeStrategies(), packmanager.StrategyPrompt),
			packmanager.StrategyOverwrite,
		),
		"strategy",
		"When a package of the same name exists, use this strategy when applying targets.",
	)

	return cmd
}

func (opts *PkgOptions) Pre(cmd *cobra.Command, args []string) error {
	ctx, err := packmanager.WithDefaultUmbrellaManagerInContext(cmd.Context())
	if err != nil {
		return err
	}

	cmd.SetContext(ctx)

	opts.Strategy = packmanager.MergeStrategy(cmd.Flag("strategy").Value.String())

	return nil
}

func (opts *PkgOptions) Run(ctx context.Context, args []string) error {
	if _, err := Pkg(ctx, opts, args...); err != nil {
		return err
	}

	return nil
}
