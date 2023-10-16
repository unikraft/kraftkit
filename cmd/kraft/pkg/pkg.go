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
	"github.com/mattn/go-shellwords"
	"github.com/spf13/cobra"

	"kraftkit.sh/config"
	"kraftkit.sh/machine/platform"
	"kraftkit.sh/pack"
	"kraftkit.sh/unikraft"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/log"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/tui/multiselect"
	"kraftkit.sh/tui/processtree"
	"kraftkit.sh/tui/selection"
	"kraftkit.sh/unikraft/app"
	"kraftkit.sh/unikraft/target"

	"kraftkit.sh/cmd/kraft/pkg/list"
	"kraftkit.sh/cmd/kraft/pkg/pull"
	"kraftkit.sh/cmd/kraft/pkg/push"
	"kraftkit.sh/cmd/kraft/pkg/rm"
	"kraftkit.sh/cmd/kraft/pkg/source"
	"kraftkit.sh/cmd/kraft/pkg/unsource"
	"kraftkit.sh/cmd/kraft/pkg/update"
)

type Pkg struct {
	Architecture string `local:"true" long:"arch" short:"m" usage:"Filter the creation of the package by architecture of known targets"`
	Args         string `local:"true" long:"args" short:"a" usage:"Pass arguments that will be part of the running kernel's command line"`
	Dbg          bool   `local:"true" long:"dbg" usage:"Package the debuggable (symbolic) kernel image instead of the stripped image"`
	Force        bool   `local:"true" long:"force-format" usage:"Force the use of a packaging handler format"`
	Format       string `local:"true" long:"as" short:"M" usage:"Force the packaging despite possible conflicts" default:"oci"`
	Initrd       string `local:"true" long:"initrd" short:"i" usage:"Path to init ramdisk to bundle within the package (passing a path will automatically generate a CPIO image)"`
	Kernel       string `local:"true" long:"kernel" short:"k" usage:"Override the path to the unikernel image"`
	Kraftfile    string `long:"kraftfile" short:"K" usage:"Set an alternative path of the Kraftfile"`
	Name         string `local:"true" long:"name" short:"n" usage:"Specify the name of the package"`
	NoKConfig    bool   `local:"true" long:"no-kconfig" usage:"Do not include target .config as metadata"`
	Output       string `local:"true" long:"output" short:"o" usage:"Save the package at the following output"`
	Platform     string `local:"true" long:"plat" short:"p" usage:"Filter the creation of the package by platform of known targets"`
	Target       string `local:"true" long:"target" short:"t" usage:"Package a particular known target"`

	strategy packmanager.MergeStrategy
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
			$ kraft pkg --as oci --name unikraft.org/nginx:latest --with-kconfig`),
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
	cmd.AddCommand(rm.New())
	cmd.AddCommand(source.New())
	cmd.AddCommand(unsource.New())
	cmd.AddCommand(update.New())

	cmd.Flags().Var(
		cmdfactory.NewEnumFlag[packmanager.MergeStrategy](
			append(packmanager.MergeStrategies(), packmanager.StrategyPrompt),
			packmanager.StrategyPrompt,
		),
		"strategy",
		"When a package of the same name exists, use this strategy when applying targets.",
	)

	return cmd
}

func (opts *Pkg) Pre(cmd *cobra.Command, _ []string) error {
	if opts.Name == "" {
		return fmt.Errorf("cannot package without setting --name")
	}

	if (len(opts.Architecture) > 0 || len(opts.Platform) > 0) && len(opts.Target) > 0 {
		return fmt.Errorf("the `--arch` and `--plat` options are not supported in addition to `--target`")
	}

	ctx := cmd.Context()

	opts.strategy = packmanager.MergeStrategy(cmd.Flag("strategy").Value.String())

	if config.G[config.KraftKit](ctx).NoPrompt && opts.strategy == "prompt" {
		return fmt.Errorf("cannot mix --strategy=prompt when --no-prompt is enabled in settings")
	}

	ctx, err := packmanager.WithDefaultUmbrellaManagerInContext(ctx)
	if err != nil {
		return err
	}

	cmd.SetContext(ctx)

	opts.Platform = platform.PlatformByName(opts.Platform).String()

	umbrella, err := packmanager.PackageManagers()
	if err != nil {
		return fmt.Errorf("could not get umbrella package manager: %w", err)
	}

	found := false
	formats := []string{}
	for _, pm := range umbrella {
		formats = append(formats, pm.Format().String())
		if pm.Format().String() == opts.Format {
			found = true
		}
	}

	if !found {
		return fmt.Errorf("unknown packaging format '%s' from choice of %v", opts.Format, formats)
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
	pm := packmanager.G(ctx)

	// Switch the package manager the desired format for this target
	pm, err = pm.From(pack.PackageFormat(opts.Format))
	if err != nil {
		return err
	}

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

	norender := log.LoggerTypeFromString(config.G[config.KraftKit](ctx).Log.Type) != log.FANCY

	exists, err := pm.Catalog(ctx,
		packmanager.WithName(opts.Name),
	)
	baseopts := []packmanager.PackOption{}
	if err == nil && len(exists) >= 1 {
		if opts.strategy == packmanager.StrategyPrompt {
			strategy, err := selection.Select[packmanager.MergeStrategy](
				fmt.Sprintf("package '%s' already exists: how would you like to proceed?", opts.Name),
				packmanager.MergeStrategies()...,
			)
			if err != nil {
				return err
			}

			opts.strategy = *strategy
		}

		switch opts.strategy {
		case packmanager.StrategyExit:
			return fmt.Errorf("package already exists and merge strategy set to exit on conflict")

		// Set the merge strategy as an option that is then passed to the
		// package manager.
		default:
			baseopts = append(baseopts,
				packmanager.PackMergeStrategy(opts.strategy),
			)
		}
	} else {
		baseopts = append(baseopts,
			packmanager.PackMergeStrategy(packmanager.StrategyMerge),
		)
	}

	var selected []target.Target
	if len(opts.Target) > 0 || len(opts.Architecture) > 0 || len(opts.Platform) > 0 && !config.G[config.KraftKit](ctx).NoPrompt {
		selected = target.Filter(project.Targets(), opts.Architecture, opts.Platform, opts.Target)
	} else {
		selected, err = multiselect.MultiSelect[target.Target]("select what to package", project.Targets()...)
		if err != nil {
			return err
		}
	}

	if len(selected) == 0 {
		return fmt.Errorf("nothing selected to package")
	}

	i := 0

	for _, targ := range selected {
		// See: https://github.com/golang/go/wiki/CommonMistakes#using-reference-to-loop-iterator-variable
		targ := targ
		baseopts := baseopts
		name := "packaging " + targ.Name() + " (" + opts.Format + ")"

		cmdShellArgs, err := shellwords.Parse(opts.Args)
		if err != nil {
			return err
		}

		// When i > 0, we have already applied the merge strategy.  Now, for all
		// targets, we actually do wish to merge these because they are part of
		// the same execution lifecycle.
		if i > 0 {
			baseopts = []packmanager.PackOption{
				packmanager.PackMergeStrategy(packmanager.StrategyMerge),
			}
		}

		tree = append(tree, processtree.NewProcessTreeItem(
			name,
			targ.Architecture().Name()+"/"+targ.Platform().Name(),
			func(ctx context.Context) error {
				popts := append(baseopts,
					packmanager.PackArgs(cmdShellArgs...),
					packmanager.PackInitrd(opts.Initrd),
					packmanager.PackKConfig(!opts.NoKConfig),
					packmanager.PackName(opts.Name),
					packmanager.PackOutput(opts.Output),
				)

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

		i++
	}

	if len(tree) == 0 {
		switch true {
		case len(opts.Target) > 0:
			return fmt.Errorf("no matching targets found for: %s", opts.Target)
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
			processtree.IsParallel(false),
			processtree.WithRenderer(norender),
		},
		tree...,
	)
	if err != nil {
		return err
	}

	return model.Start()
}
