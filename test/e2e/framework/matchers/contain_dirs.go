// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package matchers

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/types"
)

// containDirectoriesMatcher asserts that a directory contains sub-directories
// with provided names.
type containDirectoriesMatcher struct {
	dirNames []string
	err      error
}

var _ types.GomegaMatcher = (*containDirectoriesMatcher)(nil)

func (matcher *containDirectoriesMatcher) Match(actual any) (success bool, err error) {
	actualDirName, ok := actual.(string)
	if !ok {
		return false, fmt.Errorf("ContainFiles matcher expects a directory path")
	}

	dirEntries, err := os.ReadDir(actualDirName)
	if err != nil {
		matcher.err = fmt.Errorf("reading directory entries: %w", err)
		return false, nil
	}

	if n, nExpect := len(dirEntries), len(matcher.dirNames); n < nExpect {
		matcher.err = fmt.Errorf("directory contains less entries (%d) than provided sub-directories names (%d)", n, nExpect)
		return false, nil
	}

	for _, fn := range matcher.dirNames {
		fi, err := os.Stat(filepath.Join(actualDirName, fn))
		if err != nil {
			matcher.err = fmt.Errorf("reading file info: %w", err)
			return false, nil
		}

		if !fi.IsDir() {
			matcher.err = fmt.Errorf("file %q is not a directory (type: %s)", fi.Name(), fi.Mode().Type())
			return false, nil
		}
	}

	return true, nil
}

func (matcher *containDirectoriesMatcher) FailureMessage(actual any) string {
	return format.Message(actual, fmt.Sprintf("to contain the directories with the provided names: %s", matcher.err))
}

func (*containDirectoriesMatcher) NegatedFailureMessage(actual any) string {
	return format.Message(actual, "not contain the directories with the provided names")
}
