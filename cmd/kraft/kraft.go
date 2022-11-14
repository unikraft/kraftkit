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

	"kraftkit.sh/internal/cmdfactory"
	"kraftkit.sh/internal/cmdutil"

	"kraftkit.sh/cmd/kraft/build"
	"kraftkit.sh/cmd/kraft/events"
	"kraftkit.sh/cmd/kraft/pkg"
	"kraftkit.sh/cmd/kraft/ps"
	"kraftkit.sh/cmd/kraft/rm"
	"kraftkit.sh/cmd/kraft/run"
	"kraftkit.sh/cmd/kraft/stop"

	// Additional initializers
	_ "kraftkit.sh/manifest"
)

func main() {
	f := cmdfactory.New(
		cmdfactory.WithPackageManager(),
	)
	cmd, err := cmdutil.NewCmd(f, "kraft",
		cmdutil.WithSubcmds(
			pkg.PkgCmd(f),
			build.BuildCmd(f),
			ps.PsCmd(f),
			rm.RemoveCmd(f),
			run.RunCmd(f),
			stop.StopCmd(f),
			events.EventsCmd(f),
		),
	)
	if err != nil {
		panic("could not initialize root command")
	}

	cmd.Short = "Build and use highly customized and ultra-lightweight unikernels"
	cmd.Long = heredoc.Docf(`

       .
      /^\     Build and use highly customized and ultra-lightweight unikernels.
     :[ ]:    
     | = |
    /|/=\|\   Documentation:    https://kraftkit.sh/
   (_:| |:_)  Issues & support: https://github.com/unikraft/kraftkit/issues
      v v 
      ' '`)

	os.Exit(int(cmdutil.Execute(f, cmd)))
}
