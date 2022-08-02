// SPDX-License-Identifier: BSD-3-Clause
//
// Authors: Alexander Jung <alex@unikraft.io>
//
// Copyright (c) 2022, Unikraft GmbH.  All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions
// are met:
//
// 1. Redistributions of source code must retain the above copyright
//    notice, this list of conditions and the following disclaimer.
// 2. Redistributions in binary form must reproduce the above copyright
//    notice, this list of conditions and the following disclaimer in the
//    documentation and/or other materials provided with the distribution.
// 3. Neither the name of the copyright holder nor the names of its
//    contributors may be used to endorse or promote products derived from
//    this software without specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
// AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
// IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
// ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE
// LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR
// CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF
// SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS
// INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN
// CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE)
// ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE
// POSSIBILITY OF SUCH DAMAGE.

package main

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"kraftkit.sh/config"
	"kraftkit.sh/schema"

	"kraftkit.sh/internal/cmdfactory"
	"kraftkit.sh/internal/cmdutil"
	"kraftkit.sh/internal/logger"

	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"
	"kraftkit.sh/make"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/ui/paraprogress"
	"kraftkit.sh/unikraft/app"
	"kraftkit.sh/unikraft/component"
	"kraftkit.sh/unikraft/target"

	// Subcommands
	"kraftkit.sh/cmd/ukbuild/configure"
	"kraftkit.sh/cmd/ukbuild/fetch"
	"kraftkit.sh/cmd/ukbuild/menuconfig"
	"kraftkit.sh/cmd/ukbuild/prepare"

	// Additional initializers
	_ "kraftkit.sh/manifest"
)

type ukbuildOptions struct {
	PackageManager func(opts ...packmanager.PackageManagerOption) (packmanager.PackageManager, error)
	ConfigManager  func() (*config.ConfigManager, error)
	Logger         func() (log.Logger, error)
	IO             *iostreams.IOStreams

	// Command-line arguments
	NoCache      bool
	Architecture string
	Platform     string
	DotConfig    string
	Target       string
	KernelDbg    bool
	Fast         bool
	Jobs         int
	NoSyncConfig bool
	SaveBuildLog string
}

func main() {
	f := cmdfactory.New(
		cmdfactory.WithPackageManager(),
	)

	cmd, err := cmdutil.NewCmd(f, "ukbuild",
		cmdutil.WithSubcmds(
			configure.ConfigureCmd(f),
			fetch.FetchCmd(f),
			menuconfig.MenuConfigCmd(f),
			prepare.PrepareCmd(f),
		),
	)
	if err != nil {
		panic("could not initialize 'ukbuild' commmand")
	}

	opts := &ukbuildOptions{
		PackageManager: f.PackageManager,
		ConfigManager:  f.ConfigManager,
		Logger:         f.Logger,
		IO:             f.IOStreams,
	}

	cmd.Short = "Configure and build Unikraft unikernels "
	cmd.Use = "ukbuild [FLAGS] [SUBCOMMAND|DIR]"
	cmd.Args = cmdutil.MaxDirArgs(1)
	cmd.Long = heredoc.Docf(`
		Configure and build Unikraft unikernels.

		The default behaviour of %[1]sukbuild%[1]s is to build a project.  Given no
		arguments, you will be guided through interactive mode.
	`, "`")
	cmd.Example = heredoc.Doc(`
		# Build the current project (cwd)
		$ ukbuild

		# Build path to a Unikraft project
		$ ukbuild path/to/app
	`)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if (len(opts.Architecture) > 0 || len(opts.Platform) > 0) && len(opts.Target) > 0 {
			return fmt.Errorf("the `--arch` and `--plat` options are not supported in addition to `--target`")
		}

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

		return buildRun(opts, workdir)
	}

	cmd.Flags().BoolVarP(
		&opts.NoCache,
		"no-cache", "F",
		false,
		"Force a rebuild even if existing intermediate artifacts already exist",
	)

	cmd.Flags().StringVarP(
		&opts.Architecture,
		"arch", "m",
		"",
		"Filter the creation of the build by architecture of known targets",
	)

	cmd.Flags().StringVarP(
		&opts.Platform,
		"plat", "p",
		"",
		"Filter the creation of the build by platform of known targets",
	)

	cmd.Flags().StringVarP(
		&opts.DotConfig,
		"config", "c",
		"",
		"Override the path to the KConfig `.config` file",
	)

	cmd.Flags().BoolVar(
		&opts.KernelDbg,
		"dbg",
		false,
		"Build the debuggable (symbolic) kernel image instead of the stripped image",
	)

	cmd.Flags().StringVarP(
		&opts.Target,
		"target", "t",
		"",
		"Build a particular known target",
	)

	cmd.Flags().BoolVar(
		&opts.Fast,
		"fast",
		false,
		"Use maximum parallization when performing the build",
	)

	cmd.Flags().IntVarP(
		&opts.Jobs,
		"jobs", "j",
		0,
		"Allow N jobs at once",
	)

	cmd.Flags().StringVar(
		&opts.SaveBuildLog,
		"build-log",
		"",
		"Use the specified file to save the output from the build",
	)

	cmd.Flags().BoolVar(
		&opts.NoSyncConfig,
		"no-sync-config",
		false,
		"Do not synchronize Unikraft's configuration before building",
	)

	os.Exit(int(cmdutil.Execute(f, cmd)))
}

func buildRun(opts *ukbuildOptions, workdir string) error {
	var err error

	cfgm, err := opts.ConfigManager()
	if err != nil {
		return err
	}

	pm, err := opts.PackageManager()
	if err != nil {
		return err
	}

	plog, err := opts.Logger()
	if err != nil {
		return err
	}

	// Initialize at least the configuration options for a project
	projectOpts, err := schema.NewProjectOptions(
		nil,
		schema.WithLogger(plog),
		schema.WithWorkingDirectory(workdir),
		schema.WithDefaultConfigPath(),
		schema.WithPackageManager(&pm),
		schema.WithResolvedPaths(true),
		schema.WithDotConfig(true),
	)
	if err != nil {
		return err
	}

	if !schema.IsWorkdirInitialized(workdir) {
		return fmt.Errorf("cannot build uninitialized project! start with: ukbuild init")
	}

	// Interpret the application
	project, err := schema.NewApplicationFromOptions(projectOpts)
	if err != nil {
		return err
	}

	var targets []target.TargetConfig

	// Filter the targets by CLI selection
	for _, targ := range project.Targets {
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

			targets = append(targets, targ)

		default:
			continue
		}
	}

	if len(targets) == 0 {
		plog.Info("no targets to build")
		return nil
	}

	norender := logger.LoggerTypeFromString(cfgm.Config.Log.Type) != logger.FANCY
	if !norender {
		plog.SetOutput(ioutil.Discard)
	}

	var processes []*paraprogress.Process
	var mopts []make.MakeOption

	if opts.Jobs > 0 {
		mopts = append(mopts, make.WithJobs(opts.Jobs))
	} else {
		mopts = append(mopts, make.WithMaxJobs(opts.Fast))
	}

	for _, targ := range targets {
		// See: https://github.com/golang/go/wiki/CommonMistakes#using-reference-to-loop-iterator-variable
		targ := targ

		processes = append(processes, paraprogress.NewProcess(
			fmt.Sprintf("building %s (%s)", targ.Name(), targ.ArchPlatString()),
			func(l log.Logger, w func(progress float64)) error {
				// Apply the incoming logger which is tailored to display as a
				// sub-terminal within the fancy processtree.
				targ.ApplyOptions(
					component.WithLogger(l),
				)

				return project.Build(
					app.WithBuildLogger(l),
					app.WithBuildTarget(targ),
					app.WithBuildProgressFunc(w),
					app.WithBuildMakeOptions(mopts...),
					app.WithBuildNoSyncConfig(opts.NoSyncConfig),
					app.WithBuildLogFile(opts.SaveBuildLog),
				)
			},
		))
	}

	model, err := paraprogress.NewParaProgress(
		processes,
		// Disable parallelization as:
		//  - The first process may be pulling the container image, which is
		//    necessary for the subsequent build steps;
		//  - The Unikraft build system can re-use compiled files from previous
		//    compilations (if the architecture does not change).
		paraprogress.IsParallel(false),
		paraprogress.WithRenderer(norender),
		paraprogress.WithLogger(plog),
	)
	if err != nil {
		return err
	}

	return model.Start()
}
