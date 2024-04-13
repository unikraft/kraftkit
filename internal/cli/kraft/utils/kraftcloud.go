package utils

import (
	"strings"
)

// RewrapAsKraftCloudPackage returns the equivalent package name as a
// KraftCloud package.
func RewrapAsKraftCloudPackage(name string) string {
	name = strings.Replace(name, "unikarft.org/", "index.unikraft.io/", 1)

	if strings.HasPrefix(name, "unikraft.io") {
		name = "index." + name
	} else if strings.Contains(name, "/") && !strings.Contains(name, "unikraft.io") {
		name = "index.unikraft.io/" + name
	} else if !strings.HasPrefix(name, "index.unikraft.io") {
		name = "index.unikraft.io/official/" + name
	}

	return name
}
