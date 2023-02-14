// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package component

import (
	"context"
	"fmt"

	"kraftkit.sh/kconfig"
	"kraftkit.sh/unikraft"
)

// Component is the abstract interface for managing the individual microlibrary
type Component interface {
	unikraft.Nameable

	// Source returns the component source
	Source() string

	// Path is the location to this library within the context of a project.
	Path() string

	// KConfigTree returns the component's KConfig configuration menu tree which
	// returns all possible options for the component
	KConfigTree(...*kconfig.KeyValue) (*kconfig.KConfigFile, error)

	// KConfig returns the component's set of file KConfig which is known when the
	// relevant component packages have been retrieved
	KConfig() kconfig.KeyValueMap

	// PrintInfo returns human-readable information about the component
	PrintInfo(context.Context) string
}

// NameAndVersion accepts a component and provids the canonical string
// representation of the component with its name and version
func NameAndVersion(component Component) string {
	return fmt.Sprintf("%s:%s", component.Name(), component.Version())
}
