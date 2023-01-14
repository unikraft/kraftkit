// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package initialize

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"os"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/log"
	"kraftkit.sh/pack"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/tui/paraprogress"
	"kraftkit.sh/unikraft"
)

type Init struct {
	Name         string `long:"name" short:"n" usage:"" default:""`
	Architecture string `long:"arch" short:"m" usage:"" default:"x86_64"`
	Platform     string `long:"plat" short:"p" usage:"" default:"kvm"`
	Template     string `long:"template" short:"t" usage:"" default:""`
}

func New() *cobra.Command {
	return cmdfactory.New(&Init{}, cobra.Command{
		Short: "Initialize an empty application",
		Use:   "init [DIR]",
		Long: heredoc.Doc(`
			 Initialize an empty application`),
		Example: heredoc.Doc(`
			# Initialize an application 
			$ kraft init -n app
		`),
		Annotations: map[string]string{
			"help:group": "init",
		},
		Args: cobra.ExactArgs(1),
	})
}

func (opts *Init) Run(cmd *cobra.Command, args []string) error {
	path := args[0]

	if opts.Template == "" {
		return opts.createNewApp(cmd, path)
	} else {
		return opts.createAppFromTemplate(cmd, path)
	}
}

func (opts *Init) createNewApp(cmd *cobra.Command, path string) error {
	if err := os.Mkdir(path, os.ModePerm); err != nil {
		return fmt.Errorf("could not create directory: %v", err)
	}

	if opts.Name == "" {
		// Get the last part of the path as the name
		opts.Name = strings.Split(path, "/")[len(strings.Split(path, "/"))-1]
	}

	newFile, err := os.OpenFile(path+"/Kraftfile", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.ModePerm)
	if err != nil {
		return fmt.Errorf("could not create file: %v", err)
	}

	tmp, err := template.New("Kraftfile").Parse(kraftfileTemplate())
	if err != nil {
		return fmt.Errorf("could not parse template: %v", err)
	}

	tmp.Execute(newFile, opts)
	newFile.Close()

	return nil
}

func (opts *Init) createAppFromTemplate(cmd *cobra.Command, path string) error {
	// Search for the package that contains the template app
	ctx := cmd.Context()
	pm := packmanager.G(ctx)

	// Get the package that contains the template app
	packages, err := pm.Catalog(ctx, packmanager.CatalogQuery{
		Name:  opts.Template,
		Types: []unikraft.ComponentType{unikraft.ComponentTypeApp},
	})
	if err != nil {
		return err
	}

	if len(packages) == 0 {
		return errors.New("could not find the template app")
	}

	// Create a temporary directory for pulling

	dir, err := os.MkdirTemp(".", ".tmp")
	if err != nil {
		return fmt.Errorf("could not create temporary directory: %v", err)
	}

	// Download the package
	proc := paraprogress.NewProcess(
		fmt.Sprintf("pulling %s", packages[0].Options().TypeNameVersion()),
		func(ctx context.Context, w func(progress float64)) error {
			return packages[0].Pull(
				ctx,
				pack.WithPullWorkdir(dir),
			)
		},
	)

	parallel := !config.G(ctx).NoParallel
	norender := log.LoggerTypeFromString(config.G(ctx).Log.Type) != log.FANCY

	paramodel, err := paraprogress.NewParaProgress(
		ctx,
		[]*paraprogress.Process{proc},
		paraprogress.IsParallel(parallel),
		paraprogress.WithRenderer(norender),
		paraprogress.WithFailFast(true),
	)
	if err != nil {
		return err
	}

	if err := paramodel.Start(); err != nil {
		return fmt.Errorf("could not pull all components: %v", err)
	}

	pulledPath := dir + "/.unikraft/apps/"

	// Move the template app to the current directory
	if err := os.Rename(pulledPath+opts.Template, path); err != nil {
		return fmt.Errorf("could not move the template app: %v", err)
	}

	// Remove the tmp directory
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("could not remove the temporary directory: %v", err)
	}

	return nil
}

func kraftfileTemplate() string {
	return `specification: v0.5

unikraft: stable

libraries:
	newlib: stable

targets:
	- name: {{.Name}} 
	  architecture: {{.Architecture}}
	  platform: {{.Platform}} 
`
}
