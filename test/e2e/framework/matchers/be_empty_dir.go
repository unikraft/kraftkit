// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package matchers

import (
	"fmt"
	"os"

	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/types"
)

// beAnEmptyDirectoryMatcher asserts that an existing directory is empty.
type beAnEmptyDirectoryMatcher struct {
	err error
}

var _ types.GomegaMatcher = (*beAnEmptyDirectoryMatcher)(nil)

func (matcher *beAnEmptyDirectoryMatcher) Match(actual any) (success bool, err error) {
	actualDirName, ok := actual.(string)
	if !ok {
		return false, fmt.Errorf("BeAnEmptyDirectory matcher expects a directory path")
	}

	dirEntries, err := os.ReadDir(actualDirName)
	if err != nil {
		matcher.err = fmt.Errorf("reading directory entries: %w", err)
		return false, nil
	}

	n := len(dirEntries)
	hasEntries := n > 0

	if hasEntries {
		matcher.err = fmt.Errorf("directory contains %d entries", n)
	}

	return !hasEntries, nil
}

func (matcher *beAnEmptyDirectoryMatcher) FailureMessage(actual any) string {
	return format.Message(actual, fmt.Sprintf("to be an empty directory: %s", matcher.err))
}

func (*beAnEmptyDirectoryMatcher) NegatedFailureMessage(actual any) string {
	return format.Message(actual, "not be an empty directory")
}
