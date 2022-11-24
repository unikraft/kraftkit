// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package log

import "strings"

// LoggerType controls how log statements are output
type LoggerType uint

// Logger types
const (
	QUIET LoggerType = iota
	BASIC
	FANCY
	JSON
)

func LoggerTypeFromString(name string) LoggerType {
	name = strings.ToLower(name)
	switch name {
	case "quiet":
		return QUIET
	case "basic":
		return BASIC
	case "fancy":
		return FANCY
	case "json":
		return JSON
	default:
		return BASIC
	}
}

func LoggerTypeToString(t LoggerType) string {
	switch t {
	case QUIET:
		return "quiet"
	case BASIC:
		return "basic"
	case FANCY:
		return "fancy"
	case JSON:
		return "json"
	default:
		return "basic"
	}
}
