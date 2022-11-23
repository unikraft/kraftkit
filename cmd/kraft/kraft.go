// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
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
