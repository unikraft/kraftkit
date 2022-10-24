// SPDX-License-Identifier: BSD-3-Clause
//
// Authors: Cezar Craciunoiu <cezar@unikraft.io>
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

package conf

import (
	"log"
	"os"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	confset "kraftkit.sh/cmd/kraft/config/set"
	confunset "kraftkit.sh/cmd/kraft/config/unset"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cmdfactory"
	"kraftkit.sh/internal/cmdutil"
	"kraftkit.sh/iostreams"
)

type configOptions struct {
	ConfigManager func() (*config.ConfigManager, error)
	Logger        func() (log.Logger, error)
	IO            *iostreams.IOStreams

	// Command-line arguments
}

func ConfigCmd(f *cmdfactory.Factory) *cobra.Command {
	cmd, err := cmdutil.NewCmd(f, "config",
		cmdutil.WithSubcmds(
			confunset.UnsetCmd(f),
			confset.SetCmd(f),
		),
	)
	if err != nil {
		panic("could not initialize 'config' commmand")
	}

	opts := &configOptions{
		ConfigManager: f.ConfigManager,
		IO:            f.IOStreams,
	}

	cmd.Short = "Configure Kraftkit settings and functionality"
	cmd.Use = "config [FLAGS] [SUBCOMMAND]"
	cmd.Args = cmdutil.MaxDirArgs(1)
	cmd.Long = heredoc.Docf(`
		Configure Kraftkit settings and functionality.

		Calling %[1]skraft config%[1]s without any subcommands will print
		the current configuration.
	`, "`")
	cmd.Example = heredoc.Doc(`
		# Print the current configuration
		$ kraft config
	`)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {

		return configRun(opts)
	}

	return cmd
}

func configRun(opts *configOptions) error {
	var err error

	cfgm, err := opts.ConfigManager()
	if err != nil {
		return err
	}

	// Open config file
	data, err := os.ReadFile(cfgm.ConfigFile)
	if err != nil {
		return err
	}

	// Print config file to stdout
	_, err = opts.IO.Out.Write(data)
	if err != nil {
		return err
	}
	return nil
}
