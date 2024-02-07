// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package remove

import (
	"context"
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/packmanager"
)

type RemoveOptions struct {
	Name   string `long:"name" short:"n" usage:"Specify the package name that has to be pruned" default:""`
	All    bool   `long:"all" short:"a" usage:"Prunes all the packages available on the host machine"`
	Format string `long:"format" short:"f" usage:"Set the package format." default:"any"`
}

// Remove a Unikraft component.
func Remove(ctx context.Context, opts *RemoveOptions, args ...string) error {
	if opts == nil {
		opts = &RemoveOptions{}
	}

	return opts.Run(ctx, args)
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&RemoveOptions{}, cobra.Command{
		Short:   "Removes selected local packages",
		Use:     "remove [FLAGS] [PACKAGE]",
		Args:    cobra.ArbitraryArgs,
		Aliases: []string{"rm"},
		Long:    "Remove a Unikraft component.",
		Example: heredoc.Doc(`
			# Remove all packages
			kraft pkg remove --all

			# Remove only select OCI index packages
			kraft pkg remove --format=oci unikraft.org/nginx:latest
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "pkg",
		},
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *RemoveOptions) Pre(cmd *cobra.Command, args []string) error {
	if len(args) == 0 && opts.Name == "" && !opts.All {
		return fmt.Errorf("package name is not specified to remove or --all flag")
	} else if opts.All && (len(args) > 0 || opts.Name != "") {
		return fmt.Errorf("package name and --all flags cannot be specified at once")
	}

	ctx, err := packmanager.WithDefaultUmbrellaManagerInContext(cmd.Context())
	if err != nil {
		return err
	}

	cmd.SetContext(ctx)

	umbrella, err := packmanager.PackageManagers()
	if err != nil {
		panic(err)
	}

	if opts.Format != "any" {
		var available []string
		found := false
		for _, pm := range umbrella {
			available = append(available, pm.Format().String())

			if pm.Format().String() == opts.Format {
				found = true
			}
		}

		if !found {
			return fmt.Errorf("unknown package format '%s' from choice of %v", opts.Format, available)
		}
	}

	return nil
}

func (opts *RemoveOptions) Run(ctx context.Context, args []string) error {
	umbrella, err := packmanager.PackageManagers()
	if err != nil {
		return fmt.Errorf("could not get registered package managers: %w", err)
	}

	for _, pm := range umbrella {
		if opts.Format != "any" && opts.Format != pm.Format().String() {
			continue
		}

		if opts.All {
			if err = pm.Delete(ctx,
				packmanager.WithAll(opts.All),
				packmanager.WithUpdate(false),
			); err != nil {
				return err
			}
		} else {
			for _, arg := range args {
				if err := pm.Delete(ctx,
					packmanager.WithName(arg),
					packmanager.WithUpdate(false),
				); err != nil {
					return fmt.Errorf("could not complete catalog query: %w", err)
				}
			}
		}
	}

	return nil
}
