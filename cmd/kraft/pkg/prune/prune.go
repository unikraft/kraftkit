// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package prune

import (
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/packmanager"
)

type Prune struct {
	Name              string `long:"name" short:"n" usage:"Specify the package name that has to be pruned" default:""`
	All               bool   `long:"all" short:"a" usage:"Prunes all the packages available on the host machine"`
	NoManifestPackage bool   `long:"no-manifest-package" usage:"Prevent package manager from pruning manifest packages"`
	NoOCIPackage      bool   `long:"no-oci-package" usage:"Prevent package manager from pruning oci packages"`
}

func New() *cobra.Command {
	cmd, err := cmdfactory.New(&Prune{}, cobra.Command{
		Short:   "Prunes a packages locally on the host",
		Use:     "prune [FLAGS] [PACKAGE]",
		Aliases: []string{"pr"},
		Long: heredoc.Doc(`
		Prunes a packages locally on the host
		`),
		Example: heredoc.Doc(`
			# Prunes unikraft package nginx.
			$ kraft pkg prune nginx
			$ kraft pkg prune nginx:stable
			$ kraft pkg prune unikraft.org/nginx`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "pkg",
		},
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *Prune) Pre(cmd *cobra.Command, args []string) error {
	if len(args) == 0 && opts.Name == "" && !opts.All {
		return fmt.Errorf("package name is not specified to prune")
	} else if opts.All && (len(args) > 0 || opts.Name != "") {
		return fmt.Errorf("package name and --all flags cannot be specified at once")
	}
	ctx, err := packmanager.WithDefaultUmbrellaManagerInContext(cmd.Context())
	if err != nil {
		return err
	}

	cmd.SetContext(ctx)

	return nil
}

func (opts *Prune) Run(cmd *cobra.Command, args []string) error {
	var userPackage string
	if len(args) == 0 {
		userPackage = opts.Name
	} else {
		userPackage = args[0]
	}
	var version, packName string

	if !opts.All {
		packNameAndVersion := strings.Split(userPackage, ":")
		if len(packNameAndVersion) > 1 {
			version = packNameAndVersion[1]
		}
		packName = packNameAndVersion[0]
	}

	ctx := cmd.Context()

	if err := packmanager.G(ctx).Prune(ctx,
		packmanager.WithName(packName),
		packmanager.WithVersion(version),
		packmanager.WithAll(opts.All),
		packmanager.WithNoManifestPackage(opts.NoManifestPackage),
		packmanager.WithNoOCIPackage(opts.NoOCIPackage),
	); err != nil {
		return err
	}

	return packmanager.G(ctx).Update(ctx)
}
