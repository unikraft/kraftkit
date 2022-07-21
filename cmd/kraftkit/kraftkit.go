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
	"os"

	"github.com/MakeNowJust/heredoc"

	"go.unikraft.io/kit/internal/cmdfactory"
	"go.unikraft.io/kit/internal/cmdutil"
)

func main() {
	f := cmdfactory.New()
	cmd, err := cmdutil.NewCmd(f, "kraftkit")
	if err != nil {
		panic("could not initialize root commmand")
	}

	cmd.Short = "Manage the KraftKit toolsuite"
	cmd.Long = heredoc.Docf(`
    KraftKit is a suite of tools to manage, configure, build and deploy Unikraft
    unikernels.  It helps you use unikernels at all stages of their lifecycle.

    Tools available in the toolsuite include:

      ukpkg ...... Find, retrieve and package Unikraft unikernels.
      ukbuild .... Configure and build a Unikraft unikernel.
      ukrun ...... Run a Unikraft unikernel (OCI-compatible).
      ukcompose .. Run docker-compose.yml services as Unikraft unikernels.
      ukdeploy ... Deploy a Unikraft unikernel.
      kraftkit ... Manage the KraftKit toolsuite. 
    
    The %[1]skraftkit%[1]s CLI program itself allows you to manage updates,
    plugins and authentication to external services used across the toolsuite.`,
		"`")
	cmd.Example = heredoc.Doc(`
    # Check and install updates to the KraftKit toolsuite:
    $ kraftkit update

		# Manage configuration settings for KubeKraft
		$ kraftkit config set ...

    # Install an extension to the KraftKit toolsuite from GitHub:
    $ kraftkit plugin install github-owner/github-repo
  `)

	os.Exit(int(cmdutil.Execute(f, cmd)))
}
