// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package yamlmerger

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// helper: parse a YAML string into a DocumentNode
func parseYAML(t *testing.T, src string) *yaml.Node {
	t.Helper()
	var node yaml.Node
	if err := yaml.Unmarshal([]byte(src), &node); err != nil {
		t.Fatalf("failed to parse YAML: %v", err)
	}
	return &node
}

// helper: serialize a yaml.Node back to string
func toYAML(t *testing.T, node *yaml.Node) string {
	t.Helper()
	out, err := yaml.Marshal(node)
	if err != nil {
		t.Fatalf("failed to marshal YAML: %v", err)
	}
	return string(out)
}

func TestRecursiveMerge_MappingNode(t *testing.T) {
	cases := []struct {
		name    string
		from    string
		into    string
		wantKey string
		wantVal string
	}{
		{
			name:    "key in from not in into gets added",
			from:    "a: 1\nb: 2",
			into:    "a: 1",
			wantKey: "b",
			wantVal: "2",
		},
		{
			name:    "key present in both overwrites with from value",
			from:    "a: 999",
			into:    "a: 1",
			wantKey: "a",
			wantVal: "999",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			from := parseYAML(t, tc.from)
			into := parseYAML(t, tc.into)

			if err := RecursiveMerge(from, into); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			mapping := into.Content[0]
			found := false
			for i := 0; i < len(mapping.Content)-1; i += 2 {
				if mapping.Content[i].Value == tc.wantKey {
					found = true
					if mapping.Content[i+1].Value != tc.wantVal {
						t.Errorf("key %q: got value %q, want %q",
							tc.wantKey, mapping.Content[i+1].Value, tc.wantVal)
					}
				}
			}
			if !found {
				t.Errorf("key %q not found in merged result", tc.wantKey)
			}
		})
	}
}

func TestRecursiveMerge_ScalarNode_Overwrite(t *testing.T) {
	from := &yaml.Node{
		Kind:  yaml.ScalarNode,
		Value: "new",
	}
	into := &yaml.Node{
		Kind:  yaml.ScalarNode,
		Value: "old",
	}

	if err := RecursiveMerge(from, into); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if into.Value != "new" {
		t.Errorf("expected scalar to be overwritten to %q, got %q", "new", into.Value)
	}
}

func TestRecursiveMerge_SequenceNode(t *testing.T) {
	cases := []struct {
		name    string
		from    string
		into    string
		wantLen int
	}{
		{
			name:    "unique items from 'from' are appended",
			from:    "items:\n  - c\n  - d",
			into:    "items:\n  - a\n  - b",
			wantLen: 4,
		},
		{
			name:    "duplicate items are not appended",
			from:    "items:\n  - a\n  - b",
			into:    "items:\n  - a\n  - b",
			wantLen: 2,
		},
		{
			name:    "partial overlap only adds new items",
			from:    "items:\n  - b\n  - c",
			into:    "items:\n  - a\n  - b",
			wantLen: 3,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			from := parseYAML(t, tc.from)
			into := parseYAML(t, tc.into)

			if err := RecursiveMerge(from, into); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			mapping := into.Content[0]
			var seqNode *yaml.Node
			for i := 0; i < len(mapping.Content)-1; i += 2 {
				if mapping.Content[i].Value == "items" {
					seqNode = mapping.Content[i+1]
				}
			}
			if seqNode == nil {
				t.Fatal("could not find 'items' key in merged result")
			}
			if len(seqNode.Content) != tc.wantLen {
				t.Errorf("expected %d items, got %d", tc.wantLen, len(seqNode.Content))
			}
		})
	}
}

func TestRecursiveMerge_SequenceOfMappings(t *testing.T) {
	from := parseYAML(t, `
items:
  - name: b
`)
	into := parseYAML(t, `
items:
  - name: a
`)

	if err := RecursiveMerge(from, into); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := toYAML(t, into)

	if !strings.Contains(out, "name: a") || !strings.Contains(out, "name: b") {
		t.Errorf("expected both mappings in sequence, got:\n%s", out)
	}
}

func TestRecursiveMerge_DifferentKindsReturnsError(t *testing.T) {
	from := &yaml.Node{Kind: yaml.MappingNode}
	into := &yaml.Node{Kind: yaml.SequenceNode}

	err := RecursiveMerge(from, into)
	if err == nil {
		t.Error("expected error when merging nodes of different kinds, got nil")
	}
}

func TestRecursiveMerge_UnsupportedKindReturnsError(t *testing.T) {
	from := &yaml.Node{Kind: yaml.AliasNode}
	into := &yaml.Node{Kind: yaml.AliasNode}

	err := RecursiveMerge(from, into)
	if err == nil {
		t.Error("expected error for unsupported node kind, got nil")
	}
}

func TestRecursiveMerge_NestedMapping(t *testing.T) {
	from := parseYAML(t, "outer:\n  inner_b: 2")
	into := parseYAML(t, "outer:\n  inner_a: 1")

	if err := RecursiveMerge(from, into); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := toYAML(t, into)
	if !strings.Contains(out, "inner_a: 1") ||
		!strings.Contains(out, "inner_b: 2") {
		t.Errorf("expected both inner_a and inner_b in merged output, got:\n%s", out)
	}
}
