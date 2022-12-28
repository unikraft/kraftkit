// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.

package info

import (
	"errors"
	"strconv"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/manifest"
	"kraftkit.sh/pack"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/utils"
)

type Info struct {
	Type unikraft.ComponentType
}

func New() *cobra.Command {
	cmd := cmdfactory.New(&Info{}, cobra.Command{
		Short: "Retrieve informations about a package",
		Use:   "info [FLAGS] [package]",
		Long: heredoc.Doc(`
			Retrieve informations about a package.`),
		Aliases: []string{"i"},
		Example: heredoc.Doc(`
			# Retrieve informations about a package
			$ kraft pkg info nginx

			# Retrieve informations about a lib package
			$ kraft pkg info --type lib nginx
		`),
		Annotations: map[string]string{
			"help:group": "pkg",
		},
		Args: cmdfactory.ExactArgs(1, "Must specify a package name"),
	})

	cmd.Flags().VarP(
		cmdfactory.NewEnumFlag([]string{"core", "arch", "plat", "lib", "app"}, ""),
		"type",
		"t",
		"Specify the package type",
	)

	return cmd
}

func (opts *Info) Pre(cmd *cobra.Command, args []string) error {
	opts.Type = unikraft.ComponentTypes()[cmd.Flag("type").Value.String()]
	return nil
}

func (opts *Info) Run(cmd *cobra.Command, args []string) error {
	var err error

	ctx := cmd.Context()
	pm := packmanager.G(ctx)

	query := args[0]

	var packages []pack.Package

	if opts.Type == "" {
		packages, err = pm.Catalog(ctx, packmanager.CatalogQuery{
			Name: query,
		})
	} else {
		packages, err = pm.Catalog(ctx, packmanager.CatalogQuery{
			Name:  query,
			Types: []unikraft.ComponentType{opts.Type},
		})
	}

	if err != nil {
		return err
	}

	if len(packages) == 0 {
		return errors.New("no package found")
	}

	table := utils.NewTablePrinter(ctx)

	for _, pkg := range packages {
		cs := iostreams.G(ctx).ColorScheme()
		table.AddField("Name:", nil, cs.Bold)
		table.AddField(pkg.Name(), nil, nil)
		table.EndRow()

		table.AddField("Type:", nil, cs.Bold)
		table.AddField(string(pkg.Type()), nil, nil)
		table.EndRow()

		table.AddField("Version:", nil, cs.Bold)
		table.AddField(pkg.Version(), nil, nil)
		table.EndRow()

		table.AddField("Format:", nil, cs.Bold)
		table.AddField(pkg.Format(), nil, nil)
		table.EndRow()

		metadata := pkg.Metadata()

		// Parse metadata based on type
		switch metadata.(type) {
		case *manifest.Manifest:
			m := metadata.(*manifest.Manifest)

			addManifestToTable(table, m, cs)
		}

		table.AddField("----------------------------------------------------------", nil, cs.Bold)
		table.EndRow()
	}

	if err != nil {
		return err
	}

	return table.Render()
}

func addManifestToTable(table utils.TablePrinter, m *manifest.Manifest, cs *iostreams.ColorScheme) error {
	// Print all information from the manifest

	table.AddField("-------------------------MANIFEST-------------------------", nil, cs.Bold)
	table.EndRow()

	if m == nil {
		return nil
	}

	table.AddField("Name:", nil, cs.Bold)
	table.AddField(m.Name, nil, nil)
	table.EndRow()

	table.AddField("Type:", nil, cs.Bold)
	table.AddField(string(m.Type), nil, nil)
	table.EndRow()

	table.AddField("Manifest:", nil, cs.Bold)
	table.AddField(m.Manifest, nil, nil)
	table.EndRow()

	table.AddField("Description:", nil, cs.Bold)
	table.AddField(m.Description, nil, nil)
	table.EndRow()

	table.AddField("Origin", nil, cs.Bold)
	table.AddField(m.Origin, nil, nil)
	table.EndRow()

	table.AddField("Provider:", nil, cs.Bold)
	table.AddField(m.Provider.String(), nil, nil)
	table.EndRow()

	if len(m.Channels) != 0 {
		table.AddField("                     +++Channels+++:", nil, cs.Bold)
		table.EndRow()
	}

	for _, channel := range m.Channels {
		table.AddField("Name:", nil, cs.Bold)
		table.AddField(channel.Name, nil, nil)
		table.EndRow()

		table.AddField("Default:", nil, cs.Bold)
		table.AddField(strconv.FormatBool(channel.Default), nil, nil)
		table.EndRow()

		table.AddField("Latest:", nil, cs.Bold)
		table.AddField(channel.Latest, nil, nil)
		table.EndRow()

		table.AddField("Manifest:", nil, cs.Bold)
		table.AddField(channel.Manifest, nil, nil)
		table.EndRow()

		table.AddField("Resource:", nil, cs.Bold)
		table.AddField(channel.Resource, nil, nil)
		table.EndRow()

		table.AddField("Sha256:", nil, cs.Bold)
		table.AddField(channel.Sha256, nil, nil)
		table.EndRow()

		table.AddField("Local:", nil, cs.Bold)
		table.AddField(channel.Local, nil, nil)
		table.EndRow()

		table.AddField("+++++++++++++++++++++++++++++++++++++++++", nil, cs.Bold)
		table.EndRow()
	}

	if len(m.Versions) != 0 {
		table.AddField("+++Versions+++:", nil, cs.Bold)
		table.EndRow()
	}

	for _, version := range m.Versions {
		table.AddField("Version:", nil, cs.Bold)
		table.AddField(version.Version, nil, nil)
		table.EndRow()

		table.AddField("Resource:", nil, cs.Bold)
		table.AddField(version.Resource, nil, nil)
		table.EndRow()

		table.AddField("Sha256:", nil, cs.Bold)
		table.AddField(version.Sha256, nil, nil)
		table.EndRow()

		table.AddField("Type:", nil, cs.Bold)
		table.AddField(string(version.Type), nil, nil)
		table.EndRow()

		table.AddField("Unikraft:", nil, cs.Bold)
		table.AddField(version.Unikraft, nil, nil)
		table.EndRow()

		table.AddField("Local:", nil, cs.Bold)
		table.AddField(version.Local, nil, nil)
		table.EndRow()

		table.AddField("+++++++++++++++++++++++++++++++++++++++++", nil, cs.Bold)
		table.EndRow()
	}

	return nil
}
