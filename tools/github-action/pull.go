// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package main

import (
	"context"
	"fmt"
	"os"

	"kraftkit.sh/pack"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/unikraft"
)

// pull updates the package index and retrieves missing components necessary for
// performing the build.
func (opts *GithubAction) pull(ctx context.Context) error {
	if err := packmanager.G(ctx).Update(ctx); err != nil {
		return fmt.Errorf("could not update package index: %w", err)
	}

	components, err := opts.project.Components(ctx)
	if err != nil {
		return err
	}

	var missingPacks []pack.Package

	for _, component := range components {
		// Skip "finding" the component if path is the same as the source (which
		// means that the source code is already available as it is a directory on
		// disk.  In this scenario, the developer is likely hacking the particular
		// microlibrary/component.
		if component.Path() == component.Source() {
			continue
		}

		if f, err := os.Stat(component.Source()); err == nil && f.IsDir() {
			continue
		}

		p, err := packmanager.G(ctx).Catalog(ctx,
			packmanager.WithName(component.Name()),
			packmanager.WithTypes(component.Type()),
			packmanager.WithVersion(component.Version()),
			packmanager.WithSource(component.Source()),
			// packmanager.WithAuthConfig(auths),
		)
		if err != nil {
			return err
		}

		if len(p) == 0 {
			return fmt.Errorf("could not find: %s",
				unikraft.TypeNameVersion(component),
			)
		} else if len(p) > 1 {
			return fmt.Errorf("too many options for %s",
				unikraft.TypeNameVersion(component),
			)
		}

		missingPacks = append(missingPacks, p...)
	}

	for _, p := range missingPacks {
		if err := p.Pull(
			ctx,
			pack.WithPullWorkdir(opts.Workdir),
			// pack.WithPullAuthConfig(auths),
		); err != nil {
			return err
		}
	}

	return nil
}
