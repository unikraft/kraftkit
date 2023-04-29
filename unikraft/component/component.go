// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package component

import (
	"context"
	"fmt"
	"net/url"
	"os"

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

// TranslateFromSchema is an intermediate method used to convert well-known
// attributes from the Kraftfile schema into a standardized map.  This method is
// primarily used by other components internally which follow a similar format
// of specifying a version, source and list of KConfig options.
func TranslateFromSchema(props interface{}) (map[string]interface{}, error) {
	component := make(map[string]interface{})

	switch entry := props.(type) {
	case string:
		for k, v := range parseStringProp(entry) {
			component[k] = v
		}

	case map[string]interface{}:
		for key, prop := range entry {
			switch key {
			case "version":
				component["version"] = prop.(string)

			case "source":
				for k, v := range parseStringProp(prop.(string)) {
					component[k] = v
				}

			case "kconfig":
				switch tprop := prop.(type) {
				case map[string]interface{}:
					component["kconfig"] = kconfig.NewKeyValueMapFromMap(tprop)
				case []interface{}:
					component["kconfig"] = kconfig.NewKeyValueMapFromSlice(tprop...)
				}
			}
		}
	}

	return component, nil
}

func urlHasVersion(u *url.URL) string {
	if len(u.RawQuery) > 0 {
		// Pre-emptively determine the version by parsing the URL's query parameters
		for _, k := range []string{
			"branch",
			"tag",
			"version",
		} {
			if v := u.Query().Get(k); len(v) > 0 {
				return v
			}
		}
	}

	return ""
}

func parseStringProp(entry string) map[string]interface{} {
	component := make(map[string]interface{})

	if f, err := os.Stat(entry); err == nil && f.IsDir() {
		component["source"] = entry
	} else if u, err := url.Parse(entry); err == nil && u.Host != "" {
		component["source"] = entry

		if v := urlHasVersion(u); len(v) > 0 {
			component["version"] = v
		}

	} else if u, err := url.Parse("https://" + entry); err == nil && u.Host != "" {
		component["source"] = entry

		if v := urlHasVersion(u); len(v) > 0 {
			component["version"] = v
		}

	} else {
		component["version"] = entry
	}

	return component
}
