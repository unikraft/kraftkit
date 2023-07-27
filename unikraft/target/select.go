// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package target

import (
	"fmt"
	"sort"

	"github.com/charmbracelet/lipgloss"
	"github.com/erikgeiser/promptkit/selection"
)

// Select is a utility method used in a CLI context to prompt the user
// for a specific application's
func Select(targets []Target) (Target, error) {
	if len(targets) == 1 {
		return targets[0], nil
	}

	names := make([]string, 0, len(targets))
	for _, t := range targets {
		names = append(names, fmt.Sprintf(
			"%s (%s)",
			t.Name(),
			TargetPlatArchName(t),
		))
	}

	sort.Strings(names)

	queryMark := lipgloss.NewStyle().
		Background(lipgloss.Color("12")).
		Foreground(lipgloss.Color("15")).
		Render

	sp := selection.New(queryMark("[?]")+" select target:", names)
	sp.Filter = nil

	result, err := sp.RunPrompt()
	if err != nil {
		return nil, err
	}

	for _, t := range targets {
		if fmt.Sprintf("%s (%s)", t.Name(), TargetPlatArchName(t)) == result {
			return t, nil
		}
	}

	return nil, fmt.Errorf("could not select target")
}

// Filter returns a subset of `targets` based in input strings `arch`,
// `plat` and/or `targ`
func Filter(targets []Target, arch, plat, targ string) []Target {
	var selected []Target

	type condition func(Target, string, string, string) bool

	conditions := []condition{
		// If no arguments are supplied
		func(t Target, arch, plat, targ string) bool {
			return len(targ) == 0 && len(arch) == 0 && len(plat) == 0
		},

		// If only `targ` is supplied and the target name match
		func(t Target, arch, plat, targ string) bool {
			return len(targ) > 0 && len(plat) == 0 && len(arch) == 0 && t.Name() == targ
		},

		// If only `arch` is supplied and the target's arch matches
		func(t Target, arch, plat, targ string) bool {
			return len(targ) == 0 && len(plat) == 0 && len(arch) > 0 && t.Architecture().Name() == arch
		},

		// If only `plat` is supplied and the target's platform matches
		func(t Target, arch, plat, targ string) bool {
			return len(targ) == 0 && len(plat) > 0 && len(arch) == 0 && t.Platform().Name() == plat
		},

		// If both `arch` and `plat` are supplied and match the target
		func(t Target, arch, plat, targ string) bool {
			return len(targ) == 0 && len(plat) > 0 && len(arch) > 0 && t.Platform().Name() == plat && t.Architecture().Name() == arch
		},

		// If both `arch` and `targ` are supplied and match the target
		func(t Target, arch, plat, targ string) bool {
			return len(targ) == 0 && len(targ) > 0 && len(arch) > 0 && t.Name() == targ && t.Architecture().Name() == arch
		},

		// If both `plat` and `targ` are supplied and match the target
		func(t Target, arch, plat, targ string) bool {
			return len(targ) > 0 && len(plat) > 0 && len(arch) == 0 && t.Name() == targ && t.Platform().Name() == plat
		},

		// If all arguments are supplied and match the target
		func(t Target, arch, plat, targ string) bool {
			return len(targ) > 0 && len(plat) > 0 && len(arch) > 0 && t.Name() == targ && t.Platform().Name() == plat && t.Architecture().Name() == arch
		},
	}

	// Filter `targets` by input arguments `arch`, `plat` and/or `targ`
	for _, t := range targets {
		for _, c := range conditions {
			if !c(t, arch, plat, targ) {
				continue
			}

			selected = append(selected, t)
			break
		}
	}

	return selected
}
