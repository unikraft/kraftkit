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

package update

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"kraftkit.sh/config"
	"kraftkit.sh/internal/cmdfactory"
	"kraftkit.sh/internal/cmdutil"
	"kraftkit.sh/internal/logger"
	"kraftkit.sh/log"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/tui/processtree"
)

type UpdateOptions struct {
	PackageManager func(opts ...packmanager.PackageManagerOption) (packmanager.PackageManager, error)
	ConfigManager  func() (*config.ConfigManager, error)
	Logger         func() (log.Logger, error)

	// Command-line arguments
	Manager string
}

func UpdateCmd(f *cmdfactory.Factory) *cobra.Command {
	opts := &UpdateOptions{
		PackageManager: f.PackageManager,
		ConfigManager:  f.ConfigManager,
		Logger:         f.Logger,
	}

	cmd, err := cmdutil.NewCmd(f, "update")
	if err != nil {
		panic("could not initialize subcommand")
	}

	cmd.Short = "Retrieve new lists of Unikraft components, libraries and packages"
	cmd.Use = "update [FLAGS]"
	cmd.Long = heredoc.Doc(`
		Retrieve new lists of Unikraft components, libraries and packages
	`)
	cmd.Aliases = []string{"u"}
	cmd.Example = heredoc.Doc(`
		$ kraft pkg update
	`)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return updateRun(opts)
	}

	// TODO: Enable flag if multiple managers are detected?
	cmd.Flags().StringVarP(
		&opts.Manager,
		"manager", "M",
		"manifest",
		"Force the handler type",
	)

	return cmd
}

func updateRun(opts *UpdateOptions) error {
	plog, err := opts.Logger()
	if err != nil {
		return err
	}

	cfgm, err := opts.ConfigManager()
	if err != nil {
		return err
	}

	pm, err := opts.PackageManager()
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

	parallel := !cfgm.Config.NoParallel
	norender := logger.LoggerTypeFromString(cfgm.Config.Log.Type) != logger.FANCY
	if norender {
		parallel = false
	}

	model, err := processtree.NewProcessTree(
		[]processtree.ProcessTreeOption{
			// processtree.WithVerb("Updating"),
			processtree.IsParallel(parallel),
			processtree.WithRenderer(norender),
			processtree.WithLogger(plog),
		},
		[]*processtree.ProcessTreeItem{
			processtree.NewProcessTreeItem(
				"Updating...",
				"",
				func(l log.Logger) error {
					// Apply the incoming logger which is tailored to display as a
					// sub-terminal within the fancy processtree.
					pm.ApplyOptions(
						packmanager.WithLogger(l),
					)

					return pm.Update()
				},
			),
		}...,
	)
	if err != nil {
		return err
	}

	if err := model.Start(); err != nil {
		return err
	}

	return nil
}
