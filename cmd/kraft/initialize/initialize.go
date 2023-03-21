// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package initialize

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/manifest"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/schema"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/app"
	"kraftkit.sh/unikraft/core"
	"kraftkit.sh/unikraft/lib"
	"kraftkit.sh/unikraft/target"
)

type Initialize struct {
	Name      string `long:"name" short:"n" usage:"Provide a name for the application"`
	Directory string `long:"directory" short:"d" usage:"Specify the directory to initialize the application in"`
}

func New() *cobra.Command {
	cmd, err := cmdfactory.New(&Initialize{}, cobra.Command{
		Short: "Initialize a new application ",
		Use:   "init [FLAGS] [SUBCOMMAND|DIR]",
		Args:  cmdfactory.MaxDirArgs(1),
		Long: heredoc.Docf(`
			Initialize Unikraft unikernels.

			The default behaviour of %[1]skraft init%[1]s is to generate a new kraftfile.  Given no
			arguments, you will be guided through interactive mode.
		`, "`"),
		Example: heredoc.Doc(`
			# Initialize a new project in the current directory
			$ kraft init

			# Initialize a new project at a path
			$ kraft init path/to/app`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "build",
		},
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *Initialize) Pre(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	pm, err := packmanager.NewUmbrellaManager(ctx)
	if err != nil {
		return err
	}

	cmd.SetContext(packmanager.WithPackageManager(ctx, pm))

	return nil
}

func (opts *Initialize) Run(cmd *cobra.Command, args []string) error {
	var err error

	kraftfilePath := "kraft.yaml"
	if opts.Directory != "" {
		// Check that it is a directory
		if _, err = os.Stat(opts.Directory); os.IsNotExist(err) {
			return fmt.Errorf("directory %s does not exist", opts.Directory)
		}

		kraftfilePath = strings.TrimSuffix(opts.Directory, "/") + "/" + kraftfilePath
	}

	kraftfile := app.KraftfileWithPath(kraftfilePath)

	// Check if the kraftfile already exists
	if err = createKraftfile(kraftfile.Path()); err != nil {
		return err
	}

	if opts.Name == "" {
		opts.Name, err = pickName()

		if err != nil {
			return err
		}
	}

	ctx := cmd.Context()
	pm := packmanager.G(ctx)

	ukernel, err := pickUnikraft(ctx, pm)
	if err != nil {
		return err
	}

	selectedLibs, err := pickLibraries(ctx, pm)
	if err != nil {
		return err
	}

	selectedTargets, err := pickTargets()
	if err != nil {
		return err
	}

	// Hacky way to assure the order of fields in the kraftfile
	project, err := app.NewApplicationFromOptions(
		app.WithName(opts.Name),
		app.WithKraftfile(kraftfile),
		app.WithUnikraft(*ukernel),
	)
	if err != nil {
		return err
	}

	if err = project.Save(); err != nil {
		return err
	}

	project, err = app.NewApplicationFromOptions(
		app.WithName(opts.Name),
		app.WithKraftfile(kraftfile),
		app.WithUnikraft(*ukernel),
		app.WithLibraries(selectedLibs),
		app.WithTargets(selectedTargets),
	)

	if err != nil {
		return err
	}

	if err = project.Save(); err != nil {
		return err
	}

	return nil
}

func pickName() (string, error) {
	var name string
	question := []*survey.Question{{
		Name: "name",
		Prompt: &survey.Input{
			Message: "Application name",
		},
		Validate: survey.Required,
	}}

	err := survey.Ask(question, &name)
	if err != nil {
		return "", err
	}

	return name, nil
}

func pickUnikraft(ctx context.Context, pm packmanager.PackageManager) (*core.UnikraftConfig, error) {
	// Get the list of available unikernels
	ukernels, err := pm.Catalog(ctx, packmanager.WithTypes(unikraft.ComponentTypeCore))
	if err != nil {
		return nil, errors.New("unable to fetch packages. Have you run kraft pkg update?")
	}

	// If there are no ukernels available, skip the selection
	if len(ukernels) == 0 {
		return nil, fmt.Errorf("no unikernels available. Consider updating available packages by running: kraft pkg update")
	}

	var ukernelsNameVersion []string
	for _, ukernel := range ukernels {
		// Write a switch based on the type of Metadata()
		switch ukernel.Metadata().(type) {
		// Case Manifest
		case *manifest.Manifest:
			// Get the manifest
			m := ukernel.Metadata().(*manifest.Manifest)
			// Set the unikernel name
			for _, channel := range m.Channels {
				ukernelsNameVersion = append(ukernelsNameVersion, ukernel.Name()+":"+channel.Name)
			}

		// All other cases
		default:
			ukernelsNameVersion = append(ukernelsNameVersion, ukernel.Name()+":stable")
		}
	}

	ukernelNameVersion := ""

	// Ask the user to select a unikernel
	question := []*survey.Question{{
		Name: "unikernel",
		Prompt: &survey.Select{
			Message: "Select a unikernel",
			Options: ukernelsNameVersion,
		},
		Validate: survey.Required,
	}}

	err = survey.Ask(question, &ukernelNameVersion)

	if err != nil {
		return nil, err
	}

	// Split the name and version
	ukernelNameVersionSplit := strings.Split(ukernelNameVersion, ":")
	ukernelVersion := ukernelNameVersionSplit[1]

	ukernel := core.UnikraftWithVersion(ukernelVersion)

	return &ukernel, nil
}

// Pick the libraries from the package manager
func pickLibraries(ctx context.Context, pm packmanager.PackageManager) (map[string]*lib.LibraryConfig, error) {
	// Get the list of available libraries
	libs, err := pm.Catalog(ctx, packmanager.WithTypes(unikraft.ComponentTypeLib))
	if err != nil {
		return nil, errors.New("unable to fetch packages. Have you run kraft pkg update?")
	}

	var libsNameVersion []string
	for _, lib := range libs {
		switch lib.Metadata().(type) {
		case *manifest.Manifest:
			// Get the manifest
			m := lib.Metadata().(*manifest.Manifest)
			// Set the library name
			for _, channel := range m.Channels {
				libsNameVersion = append(libsNameVersion, lib.Name()+":"+channel.Name)
			}

		default:
			libsNameVersion = append(libsNameVersion, lib.Name()+":stable")
		}
	}

	selectedLibs := make(map[string]*lib.LibraryConfig)
	selectedLibsAnswer := make([]string, 0)

	ok := false

	// If there are no libs available, skip the selection
	if len(libs) == 0 {
		ok = true
		fmt.Printf("No libraries available. Considering updating available packages by running: kraft pkg update\n")
	}

	for !ok {
		ok = true

		for name := range selectedLibs {
			delete(selectedLibs, name)
		}

		q := []*survey.Question{{
			Name: "libs",
			Prompt: &survey.MultiSelect{
				Message: "Select libraries",
				Options: libsNameVersion,
				Default: selectedLibsAnswer,
			},
		}}

		selectedLibsAnswer = make([]string, 0)

		answers := struct {
			Libs []string
		}{}

		if err = survey.Ask(q, &answers); err != nil {
			return nil, err
		}

		// Iterate through selected libraries
		for _, libNameVersion := range answers.Libs {
			// Split the name and version after the last ":"

			parts := strings.Split(libNameVersion, ":")
			name := strings.Join(parts[:len(parts)-1], ":")
			version := parts[len(parts)-1]

			// Add the library to the list of selected libraries
			library := lib.LibraryWithVersion(name, version)
			if selectedLibs[name] != nil {
				ok = false
				fmt.Printf("Library %s selected multiple times\n", name)
			}
			selectedLibs[name] = &library
			selectedLibsAnswer = append(selectedLibsAnswer, libNameVersion)
		}

		if !ok {
			fmt.Println("Please select each library only once")
		}
	}

	return selectedLibs, nil
}

func createKraftfile(path string) error {
	// Check if the kraftfile already exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		file, err := os.Create(path)
		if err != nil {
			fmt.Printf("Error creating file %s: %s\n", path, err)
			return err
		}

		// Write the schema version to the file
		_, err = file.WriteString("specification: \"" + schema.SchemaVersionLatest + "\"\n")
		if err != nil {
			return err
		}
		defer file.Close()
	} else {
		fmt.Printf("File %s already exists\n", path)
		return errors.New("kraftfile already exists")
	}

	return nil
}

// Pick the targets
func pickTargets() ([]*target.TargetConfig, error) {
	availableTargets := []string{
		"linuxu/x86_64",
		"linuxu/arm32",
		"linuxu/arm64",
		"kvm/x86_64",
		"kvm/arm32",
		"kvm/arm64",
		"xen/x86_64",
		"solo5/x86_64",
	}

	selectedTargets := make([]string, 0)

	// Ask the user to select any targets
	question := []*survey.Question{{
		Name: "targets",
		Prompt: &survey.MultiSelect{
			Message: "Select targets",
			Options: availableTargets,
		},
	}}

	err := survey.Ask(question, &selectedTargets)
	if err != nil {
		return nil, err
	}

	targets := make([]*target.TargetConfig, 0)
	for _, t := range selectedTargets {
		// Split the name at the first "/" to get the platform and architecture
		parts := strings.Split(t, "/")
		platformName := parts[0]
		architectureName := parts[1]
		newTarget := target.TargetWithNames(platformName, architectureName)
		targets = append(targets, &newTarget)
	}

	return targets, nil
}
