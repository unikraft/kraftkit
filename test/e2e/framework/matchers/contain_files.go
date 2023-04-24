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

// containFilesMatcher asserts that a directory contains files with provided
// names.
type containFilesMatcher struct {
	fileNames []string
	err       error
}

var _ types.GomegaMatcher = (*containFilesMatcher)(nil)

func (matcher *containFilesMatcher) Match(actual any) (success bool, err error) {
	actualDirName, ok := actual.(string)
	if !ok {
		return false, fmt.Errorf("ContainFiles matcher expects a directory path")
	}

	dirEntries, err := os.ReadDir(actualDirName)
	if err != nil {
		matcher.err = fmt.Errorf("reading directory entries: %w", err)
		return false, nil
	}

	if n, nExpect := len(dirEntries), len(matcher.fileNames); n < nExpect {
		matcher.err = fmt.Errorf("directory contains less entries (%d) than provided files names (%d)", n, nExpect)
		return false, nil
	}

	for _, fn := range matcher.fileNames {
		fi, err := os.Stat(filepath.Join(actualDirName, fn))
		if err != nil {
			matcher.err = fmt.Errorf("reading file info: %w", err)
			return false, nil
		}

		if !fi.Mode().IsRegular() {
			matcher.err = fmt.Errorf("file %q is not regular (type: %s)", fi.Name(), fi.Mode().Type())
			return false, nil
		}
	}

	return true, nil
}

func (matcher *containFilesMatcher) FailureMessage(actual any) string {
	return format.Message(actual, fmt.Sprintf("to contain the files with the provided names: %s", matcher.err))
}

func (*containFilesMatcher) NegatedFailureMessage(actual any) string {
	return format.Message(actual, "not contain the files with the provided names")
}
