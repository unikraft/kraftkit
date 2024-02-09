// SPDX-License-Identifier: BSD-3-Clause
//
// Authors: Cezar Craciunoiu <cezar.craciunoiu@gmail.com>
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

package set

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/log"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/unikraft/app"
)

type SetOptions struct {
	Kraftfile string `long:"kraftfile" short:"K" usage:"Set an alternative path of the Kraftfile"`
	Workdir   string `long:"workdir" short:"w" usage:"Work on a unikernel at a path"`
}

// Set a KConfig variable in a Unikraft project.
func Set(ctx context.Context, opts *SetOptions, args ...string) error {
	if opts == nil {
		opts = &SetOptions{}
	}

	return opts.Run(ctx, args)
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&SetOptions{}, cobra.Command{
		Short:   "Set a variable for a Unikraft project",
		Hidden:  true,
		Use:     "set [OPTIONS] [param=value ...]",
		Aliases: []string{"s"},
		Long: heredoc.Doc(`
			set a variable for a Unikraft project
		`),
		Example: heredoc.Doc(`
			# Set variables in the cwd project
			$ kraft set LIBDEVFS_DEV_STDOUT=/dev/null LWIP_TCP_SND_BUF=4096

			# Set variables in a project at a path
			$ kraft set -w path/to/app LIBDEVFS_DEV_STDOUT=/dev/null LWIP_TCP_SND_BUF=4096
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

func (*SetOptions) Pre(cmd *cobra.Command, _ []string) error {
	ctx, err := packmanager.WithDefaultUmbrellaManagerInContext(cmd.Context())
	if err != nil {
		return err
	}

	cmd.SetContext(ctx)

	return nil
}

func (opts *SetOptions) Run(ctx context.Context, args []string) error {
	var err error

	log.G(ctx).Warnf("This command is DEPRECATED and should not be used")

	workdir := ""
	confOpts := []string{}

	// Skip if nothing can be set
	if len(args) == 0 {
		return fmt.Errorf("no options to set")
	}

	// Set the working directory (remove the argument if it exists)
	if opts.Workdir != "" {
		workdir = opts.Workdir
	} else {
		workdir, err = os.Getwd()
		if err != nil {
			return err
		}
	}

	// Set the configuration options, skip the first one if needed
	for _, arg := range args {
		if !strings.ContainsRune(arg, '=') || strings.HasSuffix(arg, "=") {
			return fmt.Errorf("invalid or malformed argument: %s", arg)
		}

		confOpts = append(confOpts, arg)
	}

	// Check if dotconfig exists in workdir
	dotconfig := fmt.Sprintf("%s/.config", workdir)

	// Check if the file exists
	// TODO: offer option to start in interactive mode
	if _, err := os.Stat(dotconfig); os.IsNotExist(err) {
		return fmt.Errorf("dotconfig file does not exist: %s", dotconfig)
	}

	popts := []app.ProjectOption{
		app.WithProjectWorkdir(workdir),
		app.WithProjectConfig(confOpts),
	}

	if len(opts.Kraftfile) > 0 {
		popts = append(popts, app.WithProjectKraftfile(opts.Kraftfile))
	} else {
		popts = append(popts, app.WithProjectDefaultKraftfiles())
	}

	// Initialize at least the configuration options for a project
	project, err := app.NewProjectFromOptions(ctx, popts...)
	if err != nil {
		return err
	}

	return project.Set(ctx, nil)
}
