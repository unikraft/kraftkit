// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package matchers_test

import (
	"testing"

	"kraftkit.sh/test/e2e/framework/matchers"
)

func TestContainDirectoriesMatcher(t *testing.T) {
	const d1 = "d1"
	const d2 = "d2"

	testCases := []struct {
		desc    string
		files   []fileEntry
		success bool
	}{
		{
			desc:    "All directories exist",
			files:   []fileEntry{dir(d1), dir(d2), dir("other")},
			success: true,
		}, {
			desc:    "One directory is missing",
			files:   []fileEntry{dir(d1), dir("other1"), dir("other2")},
			success: false,
		}, {
			desc:    "All directories are missing",
			files:   []fileEntry{dir("other1"), dir("other2")},
			success: false,
		}, {
			desc:    "One file is regular",
			files:   []fileEntry{dir(d1), regular(d2)},
			success: false,
		}, {
			desc:    "One file is a symbolic link",
			files:   []fileEntry{dir(d1), symlink(d2)},
			success: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			d := newDirectory(t, tc.files...)

			m := matchers.ContainDirectories(d1, d2)
			success, err := m.Match(d)
			if err != nil {
				t.Fatal("Failed to run matcher:", err)
			}

			if tc.success && !success {
				t.Error("Expected the matcher to succeed. Failure was:", m.FailureMessage(d))
			} else if !tc.success && success {
				t.Error("Expected the matcher to fail")
			}
		})
	}
}
