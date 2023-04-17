// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package cli

import (
	"fmt"
	"sort"

	"github.com/erikgeiser/promptkit/selection"
	"kraftkit.sh/unikraft/target"
)

// SelectTarget is a utility method used in a CLI context to prompt the user
// for a specific application's target.
func SelectTarget(targets target.Targets) (target.Target, error) {
	names := make([]string, 0, len(targets))
	for _, t := range targets {
		names = append(names, fmt.Sprintf(
			"%s (%s)",
			t.Name(),
			target.TargetPlatArchName(t),
		))
	}

	sort.Strings(names)

	sp := selection.New("select target:", names)
	sp.Filter = nil

	result, err := sp.RunPrompt()
	if err != nil {
		return nil, err
	}

	for _, t := range targets {
		if fmt.Sprintf("%s (%s)", t.Name(), target.TargetPlatArchName(t)) == result {
			return t, nil
		}
	}

	return nil, fmt.Errorf("could not select target")
}

// FilterTargets returns a subset of `targets` based in input strings `arch`,
// `plat` and/or `targ`
func FilterTargets(targets target.Targets, arch, plat, targ string) target.Targets {
	var selected target.Targets

	type condition func(target.Target, string, string, string) bool

	conditions := []condition{
		// If no arguments are supplied
		func(t target.Target, arch, plat, targ string) bool {
			return len(targ) == 0 && len(arch) == 0 && len(plat) == 0
		},

		// If the `targ` is supplied and the target name match
		func(t target.Target, arch, plat, targ string) bool {
			return len(targ) > 0 && t.Name() == targ
		},

		// If only `arch` is supplied and the target's arch matches
		func(t target.Target, arch, plat, targ string) bool {
			return len(arch) > 0 && len(plat) == 0 && t.Architecture().Name() == arch
		},

		// If only `plat`` is supplied and the target's platform matches
		func(t target.Target, arch, plat, targ string) bool {
			return len(plat) > 0 && len(arch) == 0 && t.Platform().Name() == plat
		},

		// If both `arch` and `plat` are supplied and match the target
		func(t target.Target, arch, plat, targ string) bool {
			return len(plat) > 0 && len(arch) > 0 && t.Architecture().Name() == arch && t.Platform().Name() == plat
		},
	}

	// Filter `targets` by input arguments `arch`, `plat` and/or `targ`
	for _, t := range targets {
		for _, c := range conditions {
			if !c(t, arch, plat, targ) {
				continue
			}

			selected = append(selected, t)
		}
	}

	return selected
}
