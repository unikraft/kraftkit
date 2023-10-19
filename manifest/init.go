// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package manifest

import (
	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/packmanager"
	// "kraftkit.sh/packmanager"
)

var (
	// useGit is a local variable used within the context of the manifest package
	// and is dynamically injected as a CLI option.
	useGit = false

	// gitCloneDepth is used during the cloning process to indicate the clone
	// depth.
	gitCloneDepth = -1
)

func RegisterPackageManager() func(u *packmanager.UmbrellaManager) error {
	return func(u *packmanager.UmbrellaManager) error {
		return u.RegisterPackageManager(ManifestFormat, NewManifestManager)
	}
}

func RegisterFlags() {
	// Register additional command-line flags
	cmdfactory.RegisterFlag(
		"kraft pkg pull",
		cmdfactory.BoolVarP(
			&useGit,
			"git", "g",
			false,
			"Use Git when pulling sources",
		),
	)

	cmdfactory.RegisterFlag(
		"kraft pkg pull",
		cmdfactory.IntVar(
			&gitCloneDepth,
			"git-depth",
			-1,
			"Set the Git clone depth",
		),
	)

	cmdfactory.RegisterFlag(
		"kraft build",
		cmdfactory.BoolVarP(
			&useGit,
			"git", "g",
			false,
			"Use Git when pulling sources",
		),
	)

	cmdfactory.RegisterFlag(
		"kraft build",
		cmdfactory.IntVar(
			&gitCloneDepth,
			"git-depth",
			-1,
			"Set the Git clone depth",
		),
	)
}
