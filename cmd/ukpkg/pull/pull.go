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

package pull

import (
	"fmt"
	"os"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"kraftkit.sh/config"

	"kraftkit.sh/internal/cmdfactory"
	"kraftkit.sh/internal/cmdutil"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"
	"kraftkit.sh/pack"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/schema"
	"kraftkit.sh/tui/paraprogress"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/app"
)

type PullOptions struct {
	PackageManager func(opts ...packmanager.PackageManagerOption) (packmanager.PackageManager, error)
	ConfigManager  func() (*config.ConfigManager, error)
	Logger         func() (log.Logger, error)
	IO             *iostreams.IOStreams

	// Command-line arguments
	Manager      string
	Type         string
	WithDeps     bool
	Workdir      string
	NoDeps       bool
	Platform     string
	Architecture string
	AllVersions  bool
	NoChecksum   bool
	NoCache      bool
}

func PullCmd(f *cmdfactory.Factory) *cobra.Command {
	opts := &PullOptions{
		PackageManager: f.PackageManager,
		ConfigManager:  f.ConfigManager,
		Logger:         f.Logger,
		IO:             f.IOStreams,
	}

	cmd, err := cmdutil.NewCmd(f, "pull")
	if err != nil {
		panic("could not initialize root commmand")
	}

	cmd.Short = "Pull a Unikraft unikernel and/or its dependencies"
	cmd.Use = "pull [FLAGS] [PACKAGE|DIR]"
	cmd.Aliases = []string{"p"}
	// cmd.Args = cmdutil(1)
	cmd.Long = heredoc.Doc(`
		Pull a Unikraft unikernel, component microlibrary from a remote location`)
	cmd.Example = heredoc.Doc(`
		# Pull the dependencies for a project in the current working directory
		$ ukpkg pull
		
		# Pull dependencies for a project at a path
		$ ukpkg pull path/to/app

		# Pull a source repository
		$ ukpkg pull github.com/unikraft/app-nginx.git

		# Pull an OCI-packaged Unikraft unikernel
		$ ukpkg pull unikraft.io/nginx:1.21.6

		# Pull from a manifest
		$ ukpkg pull nginx@1.21.6
	`)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		query := ""
		if len(args) > 0 {
			query = strings.Join(args, " ")
		}

		if len(query) == 0 {
			query, err = os.Getwd()
			if err != nil {
				return err
			}
		}

		if err := cmdutil.MutuallyExclusive(
			"the `--with-deps` option is not supported with `--no-deps`",
			opts.WithDeps,
			opts.NoDeps,
		); err != nil {
			return err
		}

		return pullRun(opts, query)
	}

	// TODO: Enable flag if multiple managers are detected?
	cmd.Flags().StringVarP(
		&opts.Manager,
		"manager", "M",
		"auto",
		"Force the handler type (Omittion will attempt auto-detect)",
	)

	cmd.Flags().BoolVarP(
		&opts.WithDeps,
		"with-deps", "d",
		false,
		"Pull dependencies",
	)

	cmd.Flags().StringVarP(
		&opts.Workdir,
		"workdir", "w",
		"",
		"Set a path to working directory to pull components to",
	)

	cmd.Flags().BoolVarP(
		&opts.NoDeps,
		"no-deps", "D",
		true,
		"Do not pull dependencies",
	)

	cmd.Flags().StringVarP(
		&opts.Architecture,
		"arch", "m",
		"",
		"Specify the desired architecture",
	)

	cmd.Flags().StringVarP(
		&opts.Platform,
		"plat", "p",
		"",
		"Specify the desired architecture",
	)

	cmd.Flags().BoolVarP(
		&opts.AllVersions,
		"all-versions", "a",
		false,
		"Pull all versions",
	)

	cmd.Flags().BoolVarP(
		&opts.NoChecksum,
		"no-checksum", "C",
		false,
		"Do not verify package checksum (if available)",
	)

	cmd.Flags().BoolVarP(
		&opts.NoChecksum,
		"no-cache", "Z",
		false,
		"Do not use cache and pull directly from source",
	)

	return cmd
}

func pullRun(opts *PullOptions, query string) error {
	var err error
	var project *app.ApplicationConfig
	var processes []*paraprogress.Process
	var queries []packmanager.CatalogQuery

	workdir := opts.Workdir

	pm, err := opts.PackageManager()
	if err != nil {
		return err
	}

	plog, err := opts.Logger()
	if err != nil {
		return err
	}

	// Force a particular package manager
	if len(opts.Manager) > 0 && opts.Manager != "auto" {
		pm, err = pm.From(opts.Manager)
		if err != nil {
			return err
		}
	}

	// Are we pulling an application directory?  If so, interpret the application
	// so we can get a list of components
	if f, err := os.Stat(query); err == nil && f.IsDir() {
		workdir = query
		projectOpts, err := schema.NewProjectOptions(
			nil,
			schema.WithLogger(plog),
			schema.WithWorkingDirectory(workdir),
			schema.WithDefaultConfigPath(),
			schema.WithResolvedPaths(true),
		)
		if err != nil {
			return err
		}

		// Interpret the application
		project, err := schema.NewApplicationFromOptions(projectOpts)
		if err != nil {
			return err
		}

		// List the components
		for _, c := range project.Components() {
			queries = append(queries, packmanager.CatalogQuery{
				Name:    c.Name(),
				Version: c.Version(),
				Types:   []unikraft.ComponentType{c.Type()},
			})
		}

		// Is this a list (space delimetered) of packages to pull?
	} else {
		for _, c := range strings.Split(query, " ") {
			query := packmanager.CatalogQuery{}
			t, n, v, err := unikraft.GuessTypeNameVersion(c)
			if err != nil {
				continue
			}

			if t != unikraft.ComponentTypeUnknown {
				query.Types = append(query.Types, t)
			}

			if len(n) > 0 {
				query.Name = n
			}

			if len(v) > 0 {
				query.Version = v
			}

			queries = append(queries, query)
		}
	}

	for _, c := range queries {
		next, err := pm.Catalog(c)
		if err != nil {
			return err
		}

		if len(next) == 0 {
			plog.Warnf("could not find %s", c.String())
			continue
		}

		for _, p := range next {
			p := p
			processes = append(processes, paraprogress.NewProcess(
				fmt.Sprintf("pulling %s", p.Options().TypeNameVersion()),
				func(l log.Logger, w func(progress float64)) error {
					// Apply the incoming logger which is tailored to display as a
					// sub-terminal within the fancy processtree.
					p.ApplyOptions(
						pack.WithLogger(l),
					)

					return p.Pull(
						pack.WithPullProgressFunc(w),
						pack.WithPullWorkdir(workdir),
						pack.WithPullLogger(l),
						pack.WithPullChecksum(!opts.NoChecksum),
						pack.WithPullCache(!opts.NoCache),
					)
				},
			))
		}
	}

	model, err := paraprogress.NewParaProgress(
		processes,
		paraprogress.IsParallel(true),
		paraprogress.WithLogger(plog),
	)
	if err != nil {
		return err
	}

	if err := model.Start(); err != nil {
		return err
	}

	if project != nil {
		project.PrintInfo(opts.IO)
	}

	return nil
}
