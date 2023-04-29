// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

// Package matchers contains additional Gomega matchers.
package matchers

import "github.com/onsi/gomega/types"

// BeAnEmptyDirectory succeeds if a file exists and is a directory that does
// not contain any file.
// Actual must be a string representing the absolute path to the directory
// being checked.
func BeAnEmptyDirectory() types.GomegaMatcher {
	return &beAnEmptyDirectoryMatcher{}
}

// ContainFiles succeeds if a directory exists and contains files with the
// provided names.
// Actual must be a string representing the absolute path to the directory
// containing these files.
func ContainFiles(files ...string) types.GomegaMatcher {
	return &containFilesMatcher{fileNames: files}
}

// ContainDirectories succeeds if a directory exists and contains
// sub-directories with the provided names.
// Actual must be a string representing the absolute path to the directory
// containing these sub-directories.
func ContainDirectories(dirs ...string) types.GomegaMatcher {
	return &containDirectoriesMatcher{dirNames: dirs}
}
