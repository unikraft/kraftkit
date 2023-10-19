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
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
	"kraftkit.sh/kconfig"
	"kraftkit.sh/unikraft"
)

// Component is the abstract interface for managing the individual microlibrary
type Component interface {
	unikraft.Nameable
	yaml.Marshaler

	// Source returns the component source
	Source() string

	// Path is the location to this library within the context of a project.
	Path() string

	// KConfigTree returns the component's KConfig configuration menu tree which
	// returns all possible options for the component
	KConfigTree(context.Context, ...*kconfig.KeyValue) (*kconfig.KConfigFile, error)

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
	var err error
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
				switch tprop := prop.(type) {
				case string:
					component["version"] = tprop
				case int:
					component["version"] = strconv.Itoa(tprop)
				default:
					component["version"] = fmt.Sprint(prop)
				}

			case "source":
				for k, v := range parseStringProp(prop.(string)) {
					component[k] = v
				}

			case "kconfig":
				switch tprop := prop.(type) {
				case map[string]interface{}:
					component["kconfig"], err = kconfig.NewKeyValueMapFromMap(tprop)
				case []interface{}:
					component["kconfig"], err = kconfig.NewKeyValueMapFromSlice(tprop...)
				}
				if err != nil {
					return nil, err
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

		// Remove known extensions
		u.Path = strings.TrimSuffix(u.Path, ".git")
		u.Path = strings.TrimSuffix(u.Path, ".tar.gz")

		// Remove any leading /
		u.Path = strings.TrimPrefix(u.Path, "/")

		split := strings.Split(u.Path, "/")
		var name string
		if len(split) > 1 {
			name = split[len(split)-1]
		} else {
			name = u.Path
		}

		_, name, _, err = unikraft.GuessTypeNameVersion(name)
		if err == nil {
			component["name"] = name
		}

	} else {
		component["version"] = entry
	}

	return component
}
