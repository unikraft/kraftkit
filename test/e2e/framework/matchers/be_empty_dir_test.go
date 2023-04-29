// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package matchers_test

import (
	"os"
	"testing"

	"kraftkit.sh/test/e2e/framework/matchers"
)

func TestBeAnEmptyDirectoryMatcher(t *testing.T) {
	testCases := []struct {
		desc    string
		files   []fileEntry
		success bool
	}{
		{
			desc:    "Directory is empty",
			files:   []fileEntry{},
			success: true,
		}, {
			desc:    "Directory contains a file",
			files:   []fileEntry{regular("f.txt")},
			success: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			d := newDirectory(t, tc.files...)

			m := matchers.BeAnEmptyDirectory()
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

	t.Run("Directory does not exist", func(t *testing.T) {
		d := newDirectory(t)
		if err := os.RemoveAll(d); err != nil {
			t.Fatal("Failed to remove temporary directory:", err)
		}

		m := matchers.BeAnEmptyDirectory()
		success, err := m.Match(d)
		if err != nil {
			t.Fatal("Failed to run matcher:", err)
		}

		if success {
			t.Error("Expected the matcher to fail")
		}
	})
}
