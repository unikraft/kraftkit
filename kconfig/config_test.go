// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package kconfig

import (
	"testing"
)

func TestNewKeyValueMapFromSlice(t *testing.T) {
	tests := []struct {
		name    string
		args    []interface{}
		want    KeyValueMap
		wantErr bool
	}{
		{
			name: "empty input",
			args: nil,
			want: KeyValueMap{},
		},
		{
			name: "single string",
			args: []interface{}{"CONFIG_FOO=y"},
			want: KeyValueMap{
				"CONFIG_FOO": {Key: "CONFIG_FOO", Value: "y"},
			},
		},
		{
			name: "multiple variadic strings",
			args: []interface{}{"CONFIG_FOO=y", "CONFIG_BAR=n", "CONFIG_BAZ=42"},
			want: KeyValueMap{
				"CONFIG_FOO": {Key: "CONFIG_FOO", Value: "y"},
				"CONFIG_BAR": {Key: "CONFIG_BAR", Value: "n"},
				"CONFIG_BAZ": {Key: "CONFIG_BAZ", Value: "42"},
			},
		},
		{
			name: "value with equals sign",
			args: []interface{}{"CONFIG_STR=hello=world"},
			want: KeyValueMap{
				"CONFIG_STR": {Key: "CONFIG_STR", Value: "hello=world"},
			},
		},
		{
			// core regression: []string with multiple elements used to fail
			name: "[]string with multiple elements",
			args: []interface{}{[]string{"CONFIG_FOO=y", "CONFIG_BAR=m"}},
			want: KeyValueMap{
				"CONFIG_FOO": {Key: "CONFIG_FOO", Value: "y"},
				"CONFIG_BAR": {Key: "CONFIG_BAR", Value: "m"},
			},
		},
		{
			name: "[]string with single element",
			args: []interface{}{[]string{"CONFIG_FOO=y"}},
			want: KeyValueMap{
				"CONFIG_FOO": {Key: "CONFIG_FOO", Value: "y"},
			},
		},
		{
			name: "empty []string",
			args: []interface{}{[]string{}},
			want: KeyValueMap{},
		},
		{
			name: "mixed variadic strings and []string",
			args: []interface{}{"CONFIG_FOO=y", []string{"CONFIG_BAR=m", "CONFIG_BAZ=n"}},
			want: KeyValueMap{
				"CONFIG_FOO": {Key: "CONFIG_FOO", Value: "y"},
				"CONFIG_BAR": {Key: "CONFIG_BAR", Value: "m"},
				"CONFIG_BAZ": {Key: "CONFIG_BAZ", Value: "n"},
			},
		},
		{
			name:    "[]int8 slice",
			args:    []interface{}{[]int8{1, 2}},
			wantErr: true,
		},
		{
			name:    "[]int16 slice",
			args:    []interface{}{[]int16{1, 2}},
			wantErr: true,
		},
		{
			name:    "[]int32 slice",
			args:    []interface{}{[]int32{1, 2}},
			wantErr: true,
		},
		{
			name:    "[]int64 slice",
			args:    []interface{}{[]int64{1, 2}},
			wantErr: true,
		},
		{
			name:    "[]uint slice",
			args:    []interface{}{[]uint{1, 2}},
			wantErr: true,
		},
		{
			name:    "[]uint8 slice",
			args:    []interface{}{[]uint8{1, 2}},
			wantErr: true,
		},
		{
			name:    "[]uint16 slice",
			args:    []interface{}{[]uint16{1, 2}},
			wantErr: true,
		},
		{
			name:    "[]uint32 slice",
			args:    []interface{}{[]uint32{1, 2}},
			wantErr: true,
		},
		{
			name:    "[]uint64 slice",
			args:    []interface{}{[]uint64{1, 2}},
			wantErr: true,
		},
		{
			name:    "string missing equals",
			args:    []interface{}{"CONFIG_FOO"},
			wantErr: true,
		},
		{
			name:    "string with empty value",
			args:    []interface{}{"CONFIG_FOO="},
			wantErr: true,
		},
		{
			name:    "[]string element with empty value",
			args:    []interface{}{[]string{"CONFIG_FOO=y", "CONFIG_BAR="}},
			wantErr: true,
		},
		{
			name:    "[]string element missing equals",
			args:    []interface{}{[]string{"CONFIG_FOO=y", "CONFIG_BAR"}},
			wantErr: true,
		},
		{
			name:    "unsupported type",
			args:    []interface{}{3.14},
			wantErr: true,
		},
		{
			name:    "unsupported type in []interface{}",
			args:    []interface{}{[]interface{}{true}},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewKeyValueMapFromSlice(tt.args...)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewKeyValueMapFromSlice() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if len(got) != len(tt.want) {
				t.Errorf("NewKeyValueMapFromSlice() returned %d entries, want %d", len(got), len(tt.want))
				return
			}
			for key, wantKV := range tt.want {
				gotKV, ok := got[key]
				if !ok {
					t.Errorf("NewKeyValueMapFromSlice() missing key %q", key)
					continue
				}
				if gotKV.Key != wantKV.Key || gotKV.Value != wantKV.Value {
					t.Errorf("NewKeyValueMapFromSlice()[%q] = {Key:%q Value:%q}, want {Key:%q Value:%q}",
						key, gotKV.Key, gotKV.Value, wantKV.Key, wantKV.Value)
				}
			}
		})
	}
}
