// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package probe

import (
	"context"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/internal/tableprinter"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/unikraft/app"
)

type Probe struct {
	Config    string `long:"config" short:"c" usage:"Set path to generate .config file to"`
	Kraftfile string `long:"kraftfile" short:"K" usage:"Set path to generate Kraftfile to"`
	Output    string `long:"output" short:"o" usage:"Set output format. Options: table,yaml,json,list" default:"table"`
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&Probe{}, cobra.Command{
		Short: "Read Unikraft and library metadata from an ELF binary",
		Use:   "probe [FLAGS] path/to/unikernel",
		Args:  cobra.ExactArgs(1),
		Example: heredoc.Doc(`
			# Output metadata for a Unikraft unikernel
			$ kraft x probe path/to/unikernel

			# Output the raw metadata from the elf
			$ kraft x probe -r path/to/unikernel

			# Output metadata for a Unikraft unikernel and save the Kraftfile and .config
			$ kraft x probe -K path/to/Kraftfile -c path/to/.config helloworld_qemu-x86_64`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "experimental",
		},
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *Probe) Pre(cmd *cobra.Command, _ []string) error {
	ctx, err := packmanager.WithDefaultUmbrellaManagerInContext(cmd.Context())
	if err != nil {
		return err
	}

	cmd.SetContext(ctx)

	return nil
}

func (opts *Probe) Run(ctx context.Context, args []string) error {
	kernel := args[0]

	app, err := app.NewApplicationFromKernel(ctx, kernel, opts.Config, opts.Kraftfile)
	if err != nil {
		return err
	}

	if opts.Kraftfile != "" {
		if err = app.Save(ctx); err != nil {
			return err
		}
	}

	err = iostreams.G(ctx).StartPager()
	if err != nil {
		log.G(ctx).Errorf("error starting pager: %v", err)
	}

	defer iostreams.G(ctx).StopPager()

	cs := iostreams.G(ctx).ColorScheme()

	table, err := tableprinter.NewTablePrinter(ctx,
		tableprinter.WithMaxWidth(iostreams.G(ctx).TerminalWidth()),
		tableprinter.WithOutputFormatFromString(opts.Output))
	if err != nil {
		return err
	}

	table.AddField("CORE", cs.Bold)
	table.AddField("VERSION", cs.Bold)
	table.AddField("LICENSE", cs.Bold)
	table.AddField("COMPILER", cs.Bold)
	table.AddField("COMPILE DATE", cs.Bold)
	table.AddField("COMPILED BY", cs.Bold)
	table.AddField("COMPILED BY ASSOC", cs.Bold)
	table.EndRow()

	table.AddField("Unikraft", nil)
	table.AddField(app.Unikraft(ctx).Version(), nil)
	table.AddField(app.Unikraft(ctx).License(), nil)
	table.AddField(app.Unikraft(ctx).Compiler(), nil)
	table.AddField(app.Unikraft(ctx).CompileDate(), nil)
	table.AddField(app.Unikraft(ctx).CompiledBy(), nil)
	table.AddField(app.Unikraft(ctx).CompiledByAssoc(), nil)
	table.EndRow()

	err = table.Render(iostreams.G(ctx).Out)
	if err != nil {
		return err
	}

	table, err = tableprinter.NewTablePrinter(ctx,
		tableprinter.WithMaxWidth(iostreams.G(ctx).TerminalWidth()),
		tableprinter.WithOutputFormatFromString(opts.Output))
	if err != nil {
		return err
	}

	table.AddField("LIBRARY", cs.Bold)
	table.AddField("VERSION", cs.Bold)
	table.AddField("LICENSE", cs.Bold)
	table.AddField("COMPILER", cs.Bold)
	table.AddField("COMPILE DATE", cs.Bold)
	table.AddField("COMPILED BY", cs.Bold)
	table.AddField("COMPILED BY ASSOC", cs.Bold)
	table.AddField("COMPILE FLAGS", cs.Bold)
	table.EndRow()

	libs, err := app.Libraries(ctx)
	if err != nil {
		return err
	}

	for _, lib := range libs {
		table.AddField(lib.Name(), nil)
		table.AddField(lib.Version(), nil)
		table.AddField(lib.License(), nil)
		table.AddField(lib.Compiler(), nil)
		table.AddField(lib.CompileDate(), nil)
		table.AddField(lib.CompiledBy(), nil)
		table.AddField(lib.CompiledByAssoc(), nil)

		cFlags := ""
		for _, flag := range lib.CFlags() {
			cFlags += flag.Value + " "
		}
		table.AddField(cFlags, nil)
		table.EndRow()
	}

	return table.Render(iostreams.G(ctx).Out)
}
