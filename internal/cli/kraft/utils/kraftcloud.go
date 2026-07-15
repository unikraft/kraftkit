package utils

import (
	"strings"
)

// RewrapAsKraftCloudPackage returns the equivalent package name as a
// KraftCloud package.
func RewrapAsKraftCloudPackage(name string) string {
	if _, reference, ok := strings.Cut(name, "://"); ok {
		name = reference
	}

	name = strings.Replace(name, "unikraft.org/", "index.unikraft.io/", 1)

	if strings.HasPrefix(name, "unikraft.io") {
		name = "index." + name
		return name
	}

	registry, _, hasPath := strings.Cut(name, "/")
	if hasPath && (strings.Contains(registry, ".") || strings.Contains(registry, ":") || registry == "localhost") {
		return name
	}

	if hasPath {
		name = "index.unikraft.io/" + name
	} else {
		name = "index.unikraft.io/official/" + name
	}

	return name
}
