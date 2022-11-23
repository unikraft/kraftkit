// SPDX-License-Identifier: BSD-3-Clause
//
// Authors: Alexander Jung <alex@unikraft.io>
//
// Copyright (c) 2022, Unikraft GmbH.  All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions
// are met:
//
// 1. Redistributions of source code must retain the above copyright
//    notice, this list of conditions and the following disclaimer.
// 2. Redistributions in binary form must reproduce the above copyright
//    notice, this list of conditions and the following disclaimer in the
//    documentation and/or other materials provided with the distribution.
// 3. Neither the name of the copyright holder nor the names of its
//    contributors may be used to endorse or promote products derived from
//    this software without specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
// AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
// IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
// ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE
// LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR
// CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF
// SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS
// INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN
// CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE)
// ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE
// POSSIBILITY OF SUCH DAMAGE.

package unikraft

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
)

type ComponentType string

const (
	ComponentTypeUnknown ComponentType = "unknown"
	ComponentTypeCore    ComponentType = "core"
	ComponentTypeArch    ComponentType = "arch"
	ComponentTypePlat    ComponentType = "plat"
	ComponentTypeLib     ComponentType = "lib"
	ComponentTypeApp     ComponentType = "app"
)

// Error definitions for common errors used in unikraft.
var ErrComponentTypeUnknown = errors.New("cannot place component of unknown type")

func ComponentTypes() map[string]ComponentType {
	return map[string]ComponentType{
		"core":  ComponentTypeCore,
		"arch":  ComponentTypeArch,
		"archs": ComponentTypeArch,
		"plat":  ComponentTypePlat,
		"plats": ComponentTypePlat,
		"lib":   ComponentTypeLib,
		"libs":  ComponentTypeLib,
		"app":   ComponentTypeApp,
		"apps":  ComponentTypeApp,
	}
}

func (ct ComponentType) Plural() string {
	if ct == ComponentTypeUnknown || ct == ComponentTypeCore {
		return string(ct)
	}

	return string(ct) + "s"
}

// GuessNameAndType attempts to parse the input string, which could be formatted
// such that its type, name and version are present
func GuessTypeNameVersion(input string) (ComponentType, string, string, error) {
	re := regexp.MustCompile(
		`(?i)^` +
			`(?:(?P<type>(?:lib|app|plat|arch)s?)[\-/])?` +
			`(?P<name>[\w\-\_\*]*)` +
			`(?:\:(?P<version>[\w\.\-\_]*))?` +
			`$`,
	)

	match := re.FindStringSubmatch(input)
	if len(match) == 0 {
		return ComponentTypeUnknown, "", "", fmt.Errorf("cannot determine name and type from \"%s\"", input)
	}

	t, n, v := match[1], match[2], match[3]

	if n == "unikraft" {
		t = string(ComponentTypeCore)
	}

	// Is the type recognised?
	if found, ok := ComponentTypes()[t]; ok {
		return found, n, v, nil
	}

	return ComponentTypeUnknown, n, v, nil
}

// PlaceComponent is a universal source of truth for identifying the path to
// place a component
func PlaceComponent(workdir string, t ComponentType, name string) (string, error) {
	// TODO: Should the hidden-file (`.`) be optional?
	switch t {
	case ComponentTypeCore:
		return filepath.Join(workdir, ".unikraft", "unikraft"), nil
	case ComponentTypeApp,
		ComponentTypeLib,
		ComponentTypeArch,
		ComponentTypePlat:
		return filepath.Join(workdir, ".unikraft", t.Plural(), name), nil
	}

	return "", ErrComponentTypeUnknown
}
