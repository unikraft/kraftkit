// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package build

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/dustin/go-humanize"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/internal/cli/kraft/utils"
	"kraftkit.sh/internal/fancymap"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/machine/platform"
	"kraftkit.sh/tui"

	"kraftkit.sh/log"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/unikraft/app"
	"kraftkit.sh/unikraft/target"
)

var ErrContextNotBuildable = fmt.Errorf("could not determine what or how to build from the given context")

type BuildOptions struct {
	All          bool           `long:"all" usage:"Build all targets"`
	Architecture string         `long:"arch" short:"m" usage:"Filter the creation of the build by architecture of known targets"`
	DotConfig    string         `long:"config" short:"c" usage:"Override the path to the KConfig .config file"`
	Env          []string       `long:"env" short:"e" usage:"Set environment variables to be built in the unikernel"`
	ForcePull    bool           `long:"force-pull" usage:"Force pulling packages before building"`
	Jobs         int            `long:"jobs" short:"j" usage:"Allow N jobs at once"`
	KernelDbg    bool           `long:"dbg" usage:"Build the debuggable (symbolic) kernel image instead of the stripped image"`
	Kraftfile    string         `long:"kraftfile" short:"K" usage:"Set an alternative path of the Kraftfile"`
	NoCache      bool           `long:"no-cache" short:"F" usage:"Force a rebuild even if existing intermediate artifacts already exist"`
	NoConfigure  bool           `long:"no-configure" usage:"Do not run Unikraft's configure step before building"`
	NoFast       bool           `long:"no-fast" usage:"Do not use maximum parallelization when performing the build"`
	NoFetch      bool           `long:"no-fetch" usage:"Do not run Unikraft's fetch step before building"`
	NoRootfs     bool           `long:"no-rootfs" usage:"Do not build the root file system (initramfs)"`
	NoUpdate     bool           `long:"no-update" usage:"Do not update package index before running the build"`
	Platform     string         `long:"plat" short:"p" usage:"Filter the creation of the build by platform of known targets"`
	PrintStats   bool           `long:"print-stats" usage:"Print build statistics"`
	Rootfs       string         `long:"rootfs" usage:"Specify a path to use as root file system (can be volume or initramfs)"`
	SaveBuildLog string         `long:"build-log" usage:"Use the specified file to save the output from the build"`
	Target       *target.Target `noattribute:"true"`
	TargetName   string         `long:"target" short:"t" usage:"Build a particular known target"`
	Workdir      string         `noattribute:"true"`

	project    app.Application
	statistics map[string]string
}

// Build a Unikraft unikernel.
func Build(ctx context.Context, opts *BuildOptions, args ...string) error {
	var err error

	if opts == nil {
		opts = &BuildOptions{}
	}

	if len(opts.Workdir) == 0 {
		if len(args) == 0 {
			opts.Workdir, err = os.Getwd()
			if err != nil {
				return err
			}
		} else {
			opts.Workdir = args[0]
		}
	}

	popts := []app.ProjectOption{
		app.WithProjectWorkdir(opts.Workdir),
	}

	if len(opts.Kraftfile) > 0 {
		popts = append(popts, app.WithProjectKraftfile(opts.Kraftfile))
	} else {
		popts = append(popts, app.WithProjectDefaultKraftfiles())
	}

	// Initialize at least the configuration options for a project
	opts.project, err = app.NewProjectFromOptions(ctx, popts...)
	if err != nil && errors.Is(err, app.ErrNoKraftfile) {
		return fmt.Errorf("cannot build project directory without a Kraftfile")
	} else if err != nil {
		return fmt.Errorf("could not initialize project directory: %w", err)
	}

	opts.Platform = platform.PlatformByName(opts.Platform).String()
	opts.statistics = map[string]string{}

	var build builder
	builders := builders()

	// Iterate through the list of built-in builders which sequentially tests
	// the current context and Kraftfile match specific requirements towards
	// performing a type of build.
	for _, candidate := range builders {
		log.G(ctx).
			WithField("builder", candidate.String()).
			Trace("checking buildability")

		capable, err := candidate.Buildable(ctx, opts, args...)
		if capable && err == nil {
			build = candidate
			break
		}
	}

	if build == nil {
		return ErrContextNotBuildable
	}

	log.G(ctx).WithField("builder", build.String()).Debug("using")

	if err := build.Prepare(ctx, opts, args...); err != nil {
		return fmt.Errorf("could not complete build: %w", err)
	}

	if opts.Rootfs, _, _, err = utils.BuildRootfs(ctx, opts.Workdir, opts.Rootfs, false, *opts.Target); err != nil {
		return err
	}

	err = build.Build(ctx, opts, args...)
	if err != nil {
		return fmt.Errorf("could not complete build: %w", err)
	}

	if opts.PrintStats {
		return build.Statistics(ctx, opts, args...)
	}

	// NOTE(craciunoiuc): This is currently a workaround to remove empty
	// Makefile.uk files generated wrongly by the build system. Until this
	// is fixed we just delete.
	//
	// See: https://github.com/unikraft/unikraft/issues/1456
	make := filepath.Join(opts.Workdir, "Makefile.uk")
	if finfo, err := os.Stat(make); err == nil && finfo.Size() == 0 {
		err := os.Remove(make)
		if err != nil {
			return fmt.Errorf("removing empty Makefile.uk: %w", err)
		}
	}

	return nil
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&BuildOptions{}, cobra.Command{
		Short:   "Configure and build Unikraft unikernels",
		Use:     "build [FLAGS] [SUBCOMMAND|DIR]",
		Args:    cmdfactory.MaxDirArgs(1),
		Aliases: []string{"bld"},
		Long: heredoc.Docf(`
			Build a Unikraft unikernel.

			The default behaviour of %[1]skraft build%[1]s is to build a project.  Given no
			arguments, you will be guided through interactive mode.
		`, "`"),
		Example: heredoc.Doc(`
			# Build the current project (cwd)
			$ kraft build

			# Build path to a Unikraft project
			$ kraft build path/to/app
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "build",
		},
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *BuildOptions) Pre(cmd *cobra.Command, args []string) error {
	ctx, err := packmanager.WithDefaultUmbrellaManagerInContext(cmd.Context())
	if err != nil {
		return err
	}

	cmd.SetContext(ctx)

	return nil
}

func (opts *BuildOptions) Run(ctx context.Context, args []string) error {
	if err := Build(ctx, opts, args...); err != nil {
		return err
	}

	workdir, err := filepath.Abs(opts.Workdir)
	if err != nil {
		return fmt.Errorf("getting the work directory: %w", err)
	}

	workdir += "/"

	entries := []fancymap.FancyMapEntry{}

	if opts.project.Unikraft(ctx) != nil {
		kernelStat, err := os.Stat((*opts.Target).Kernel())
		if err != nil {
			return fmt.Errorf("getting kernel image size: %w", err)
		}

		kernelPath, err := filepath.Abs((*opts.Target).Kernel())
		if err != nil {
			return fmt.Errorf("getting kernel absolute path: %w", err)
		}

		entries = append(entries, fancymap.FancyMapEntry{
			Key:   "kernel",
			Value: strings.TrimPrefix(kernelPath, workdir),
			Right: fmt.Sprintf("(%s)", humanize.Bytes(uint64(kernelStat.Size()))),
		})
	}

	if opts.Rootfs != "" {
		initrdStat, err := os.Stat(opts.Rootfs)
		if err != nil {
			return fmt.Errorf("getting initramfs size: %w", err)
		}

		initrdPath, err := filepath.Abs(opts.Rootfs)
		if err != nil {
			return fmt.Errorf("getting initramfs absolute path: %w", err)
		}

		entries = append(entries, fancymap.FancyMapEntry{
			Key:   "initramfs",
			Value: strings.TrimPrefix(initrdPath, workdir),
			Right: fmt.Sprintf("(%s)", humanize.Bytes(uint64(initrdStat.Size()))),
		})
	}

	if opts.PrintStats {
		// Sort the statistics map by key
		keys := make([]string, 0, len(opts.statistics))
		for k := range opts.statistics {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			entries = append(entries, fancymap.FancyMapEntry{
				Key:   k,
				Value: opts.statistics[k],
			})
		}
	}

	if !iostreams.G(ctx).IsStdoutTTY() {
		fields := logrus.Fields{}
		for _, entry := range entries {
			fields[entry.Key] = entry.Value
		}
		log.G(ctx).WithFields(fields).Info("build completed successfully")
		return nil
	}

	fancymap.PrintFancyMap(
		iostreams.G(ctx).Out,
		tui.TextGreen,
		"Build completed successfully!",
		entries...,
	)

	fmt.Fprint(iostreams.G(ctx).Out, "Learn how to package your unikernel with: kraft pkg --help\n")

	return nil
}
