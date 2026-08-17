// SPDX-License-Identifier: MIT
//
// Copyright (c) 2026 Unikraft GmbH.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this list of conditions shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package yamlmerger

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestRecursiveMerge_ScalarOverwrite(t *testing.T) {
	var from, into yaml.Node
	if err := yaml.Unmarshal([]byte("key: new\n"), &from); err != nil {
		t.Fatalf("unmarshal from: %v", err)
	}
	if err := yaml.Unmarshal([]byte("key: old\n"), &into); err != nil {
		t.Fatalf("unmarshal into: %v", err)
	}

	if err := RecursiveMerge(&from, &into); err != nil {
		t.Fatalf("RecursiveMerge: %v", err)
	}

	out, err := yaml.Marshal(&into)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(out)
	if !strings.Contains(got, "new") {
		t.Errorf("expected merged output to contain %q, got %q", "new", got)
	}
	if strings.Contains(got, "old") {
		t.Errorf("expected merged output not to contain %q, got %q", "old", got)
	}
}
