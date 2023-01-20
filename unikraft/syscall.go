// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package unikraft

import (
	"strconv"
	"strings"
)

// ProvidedSyscall is a simple structure which contains the syscall name and the
// number of arguments which are accepted.
type ProvidedSyscall struct {
	Name  string
	Nargs uint
}

// NewProvidedSyscall converts an exported `UK_PROVIDED_SYSCALL` entry into a
// standard `ProvidedSyscall` structure.
func NewProvidedSyscall(export string) *ProvidedSyscall {
	syscall, snargs, found := strings.Cut(export, "-")
	if !found {
		return nil
	}

	nargs, _ := strconv.ParseUint(snargs, 10, 64)

	return &ProvidedSyscall{
		Name:  syscall,
		Nargs: uint(nargs),
	}
}
