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

package confunset

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cmdfactory"
	"kraftkit.sh/internal/cmdutil"
)

type UnsetOptions struct {
	ConfigManager func() (*config.ConfigManager, error)

	// Command-line arguments
}

func UnsetCmd(f *cmdfactory.Factory) *cobra.Command {
	opts := &UnsetOptions{
		ConfigManager: f.ConfigManager,
	}

	cmd, err := cmdutil.NewCmd(f, "unset")
	if err != nil {
		panic("could not initialize subcommmand")
	}

	cmd.Short = "Unset a configuration value for kraftkit"
	cmd.Use = "set [FLAGS] KEY"
	cmd.Long = heredoc.Doc(`
		Unset a configuration value for kraftkit
	`)
	cmd.Aliases = []string{"u"}
	cmd.Example = heredoc.Doc(`
		$ kraft config unset 
	`)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return unsetRun(opts, args)
	}

	return cmd
}

func unsetRun(opts *UnsetOptions, args []string) error {
	cfgm, err := opts.ConfigManager()
	if err != nil {
		return err
	}

	// TODO: Unset values from the configuration

	// Save new configuration
	err = cfgm.Write(false)
	if err != nil {
		return err
	}

	return nil
}
