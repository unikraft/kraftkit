// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package lib

import (
	"strings"

	"kraftkit.sh/utils"
)

const (
	EvalCallAddLib           = "$(eval $(call addlib,"
	EvalCallAddLibSwitch     = "$(eval $(call addlib_s,"
	EvalCallAddPlatLib       = "$(eval $(call addplatlib,"
	EvalCallAddPlatLibSwitch = "$(eval $(call addplatlib_s,"
	EvalCallClone            = "$(eval $(call clone,"
	EvalCallFetch            = "$(eval $(call fetch,"
	EvalCallFetch2           = "$(eval $(call fetch2,"
	EvalCallFetchAs          = "$(eval $(call fetchas,"
	EvalCallFetchAs2         = "$(eval $(call fetchas2,"
	EvalCallPatch            = "$(eval $(call patch,"
	EvalCallUnarchive        = "$(eval $(call unarchive,"

	ASFLAGS     = "ASFLAGS"
	ASINCLUDES  = "ASINCLUDES"
	CFLAGS      = "CFLAGS"
	CINCLUDES   = "CINCLUDES"
	CXXFLAGS    = "CXXFLAGS"
	CXXINCLUDES = "CXXINCLUDES"
	OBJS        = "OBJS"
	OBJCFLAGS   = "OBJCFLAGS"
	SRCS        = "SRCS"
)

var matchAppendTypes = []string{
	ASFLAGS,
	ASINCLUDES,
	CFLAGS,
	CINCLUDES,
	CXXFLAGS,
	CXXINCLUDES,
	OBJS,
	OBJCFLAGS,
	SRCS,
}

// MatchAddition is a general-purpose mechanism for matching a standard Unikraft
// `Makefile.uk` file line which consists of an append for an additional file or
// flag to a composite.  A composite is usually a library, e.g. `LIBKVMPLAT`.
//
// This method is EXPERIMENTAL since the format of `Makefile.uk` files may
// include non-standard and Make-specific language traits (so use with caution
// as it is incomplete in its parsing mechanism).  The method is helpful in
// gaining a picture of the source files and flags which are used within a
// specific Unikraft library.
func MatchAddition(line string) (match bool, composite, flag, condition string, matches []string) {
	line = strings.TrimSpace(line)

	if !strings.Contains(line, "+=") {
		return
	}

	left, right, match := strings.Cut(line, "+=")
	if !match {
		return
	}

	_matches := strings.Split(right, " ")
	for _, m := range _matches {
		n := strings.TrimSpace(m)
		if n == "" || n == "\\" {
			continue
		}

		matches = append(matches, n)
	}

	left, condition, _ = strings.Cut(left, "-")
	condition = strings.TrimSpace(condition)
	left = strings.TrimSpace(left)

	parts := strings.Split(left, "_")
	if len(parts) == 1 {
		if utils.Contains(matchAppendTypes, left) {
			flag = left
			return
		}

		composite = left
		return
	}

	_flag := parts[len(parts)-1]
	if utils.Contains(matchAppendTypes, _flag) {
		flag = _flag
		composite = strings.TrimSpace(strings.Join(parts[0:len(parts)-1], "_"))
	} else {
		composite = left
	}

	return
}

// MatchRegistrationLine detects if the line represents a library registration.
//
// The MatchRegistrationLine method should be used on a line-by-line basis when
// parsing a standard Unikraft `Makefile.uk` file.  The input line is checked
// against a number of Make-based methods which are used to register the library
// with the Unikraft core build system.
//
// The method returns `match` as `true` if the line consists of a registration
// along with additional information indicate how the library is registered.
// For example, the library could be platform-specific and so `plat` will return
// a non-nil value consisting of the platform's canonoical name.  The library's
// name `libname` will always return with a value and `condition` will return
// non-nil if a KConfig boolean "switch" can be used to enable the detected
// library.
func MatchRegistrationLine(line string) (match bool, plat *string, libname string, condition *string) {
	line = strings.TrimSpace(line)

	// Match `$(eval $(call addlib,`
	if strings.HasPrefix(line, EvalCallAddLib) {
		line = strings.TrimPrefix(line, EvalCallAddLib)
		libname = line[0 : len(line)-2]
		match = true

		// Match `$(eval $(call addlib_s,`
	} else if strings.HasPrefix(line, EvalCallAddLibSwitch) {
		line = strings.TrimPrefix(line, EvalCallAddLibSwitch)
		split := strings.Split(line, ",")

		// If the `Makefile.uk` file is corrupt, do not match
		if len(split) != 2 {
			match = false
			return
		}

		match = true
		libname = split[0]
		s := split[1][2 : len(split[1])-3]
		condition = &s

		// Match `$(eval $(call addplatlib,`
	} else if strings.HasPrefix(line, EvalCallAddPlatLib) {
		line = strings.TrimPrefix(line, EvalCallAddPlatLib)
		split := strings.Split(line, ",")

		// If the `Makefile.uk` file is corrupt, do not match
		if len(split) != 2 {
			match = false
			return
		}

		match = true
		plat = &split[0]
		libname = split[1][0 : len(split[1])-2]

		// Match `$(eval $(call addplatlib_s,`
	} else if strings.HasPrefix(line, EvalCallAddPlatLibSwitch) {
		line = strings.TrimPrefix(line, EvalCallAddPlatLibSwitch)
		split := strings.Split(line, ",")

		// If the `Makefile.uk` file is corrupt, do not match
		if len(split) != 3 {
			match = false
			return
		}

		match = true
		plat = &split[0]
		libname = split[1][0:len(split[1])]
		split[2] = split[2][2 : len(split[2])-3]
		condition = &split[2]
	}

	return
}
