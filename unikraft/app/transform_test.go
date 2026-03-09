// SPDX-License-Identifier: Apache-2.0
//
// Copyright 2020 The Compose Specification Authors.
// Copyright 2022 Unikraft GmbH. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at:
//
// 		http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package app

import (
	"context"
	"testing"
)

func Test_toString(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		allowNil bool
		want     interface{}
	}{
		{
			name:     "string value",
			value:    "hello",
			allowNil: false,
			want:     "hello",
		},
		{
			name:     "integer value",
			value:    42,
			allowNil: false,
			want:     "42",
		},
		{
			name:     "nil with allowNil true",
			value:    nil,
			allowNil: true,
			want:     nil,
		},
		{
			name:     "nil with allowNil false",
			value:    nil,
			allowNil: false,
			want:     "",
		},
		{
			name:     "bool value",
			value:    true,
			allowNil: false,
			want:     "true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toString(tt.value, tt.allowNil)
			if got != tt.want {
				t.Errorf("toString(%v, %v) = %v, want %v", tt.value, tt.allowNil, got, tt.want)
			}
		})
	}
}

func Test_toMapStringString(t *testing.T) {
	tests := []struct {
		name     string
		value    map[string]interface{}
		allowNil bool
		want     map[string]interface{}
	}{
		{
			name:     "simple string values",
			value:    map[string]interface{}{"key": "value"},
			allowNil: false,
			want:     map[string]interface{}{"key": "value"},
		},
		{
			name:     "integer values converted to string",
			value:    map[string]interface{}{"port": 8080},
			allowNil: false,
			want:     map[string]interface{}{"port": "8080"},
		},
		{
			name:     "nil value with allowNil true",
			value:    map[string]interface{}{"key": nil},
			allowNil: true,
			want:     map[string]interface{}{"key": nil},
		},
		{
			name:     "nil value with allowNil false becomes empty string",
			value:    map[string]interface{}{"key": nil},
			allowNil: false,
			want:     map[string]interface{}{"key": ""},
		},
		{
			name:     "empty map",
			value:    map[string]interface{}{},
			allowNil: false,
			want:     map[string]interface{}{},
		},
		{
			name:     "multiple keys",
			value:    map[string]interface{}{"a": "1", "b": 2},
			allowNil: false,
			want:     map[string]interface{}{"a": "1", "b": "2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toMapStringString(tt.value, tt.allowNil)
			if len(got) != len(tt.want) {
				t.Errorf("toMapStringString() len = %d, want %d", len(got), len(tt.want))
				return
			}
			for k, wantVal := range tt.want {
				if got[k] != wantVal {
					t.Errorf("toMapStringString()[%q] = %v, want %v", k, got[k], wantVal)
				}
			}
		})
	}
}

func Test_transformValueToMapEntry(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		separator string
		allowNil  bool
		wantKey   string
		wantVal   interface{}
	}{
		{
			name:      "key=value pair",
			value:     "KEY=value",
			separator: "=",
			allowNil:  false,
			wantKey:   "KEY",
			wantVal:   "value",
		},
		{
			name:      "key only with allowNil true returns nil",
			value:     "KEY",
			separator: "=",
			allowNil:  true,
			wantKey:   "KEY",
			wantVal:   nil,
		},
		{
			name:      "key only with allowNil false returns empty string",
			value:     "KEY",
			separator: "=",
			allowNil:  false,
			wantKey:   "KEY",
			wantVal:   "",
		},
		{
			name:      "value with separator in it only splits on first",
			value:     "KEY=val=ue",
			separator: "=",
			allowNil:  false,
			wantKey:   "KEY",
			wantVal:   "val=ue",
		},
		{
			name:      "colon separator",
			value:     "host:port",
			separator: ":",
			allowNil:  false,
			wantKey:   "host",
			wantVal:   "port",
		},
		{
			name:      "empty value after separator",
			value:     "KEY=",
			separator: "=",
			allowNil:  false,
			wantKey:   "KEY",
			wantVal:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotKey, gotVal := transformValueToMapEntry(tt.value, tt.separator, tt.allowNil)
			if gotKey != tt.wantKey {
				t.Errorf("transformValueToMapEntry() key = %q, want %q", gotKey, tt.wantKey)
			}
			if gotVal != tt.wantVal {
				t.Errorf("transformValueToMapEntry() val = %v, want %v", gotVal, tt.wantVal)
			}
		})
	}
}

func Test_transformMappingOrList(t *testing.T) {
	tests := []struct {
		name          string
		mappingOrList interface{}
		sep           string
		allowNil      bool
		wantErr       bool
		wantResult    map[string]interface{}
	}{
		{
			name:          "map input",
			mappingOrList: map[string]interface{}{"KEY": "value"},
			sep:           "=",
			allowNil:      false,
			wantErr:       false,
			wantResult:    map[string]interface{}{"KEY": "value"},
		},
		{
			name:          "list input with key=value",
			mappingOrList: []interface{}{"KEY=value"},
			sep:           "=",
			allowNil:      false,
			wantErr:       false,
			wantResult:    map[string]interface{}{"KEY": "value"},
		},
		{
			name:          "list input with key only and allowNil false",
			mappingOrList: []interface{}{"KEY"},
			sep:           "=",
			allowNil:      false,
			wantErr:       false,
			wantResult:    map[string]interface{}{"KEY": ""},
		},
		{
			name:          "list input with non-string item errors",
			mappingOrList: []interface{}{123},
			sep:           "=",
			allowNil:      false,
			wantErr:       true,
		},
		{
			name:          "invalid type errors",
			mappingOrList: "invalid",
			sep:           "=",
			allowNil:      false,
			wantErr:       true,
		},
		{
			name:          "empty list",
			mappingOrList: []interface{}{},
			sep:           "=",
			allowNil:      false,
			wantErr:       false,
			wantResult:    map[string]interface{}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := transformMappingOrList(tt.mappingOrList, tt.sep, tt.allowNil)
			if (err != nil) != tt.wantErr {
				t.Errorf("transformMappingOrList() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			result := got.(map[string]interface{})
			if len(result) != len(tt.wantResult) {
				t.Errorf("transformMappingOrList() len = %d, want %d", len(result), len(tt.wantResult))
				return
			}
			for k, wantVal := range tt.wantResult {
				if result[k] != wantVal {
					t.Errorf("transformMappingOrList()[%q] = %v, want %v", k, result[k], wantVal)
				}
			}
		})
	}
}

func Test_transformCommand(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		input   interface{}
		want    []string
		wantErr bool
	}{
		{
			name:  "simple command string",
			input: "nginx -g 'daemon off;'",
			want:  []string{"nginx", "-g", "daemon off;"},
		},
		{
			name:  "single word command",
			input: "sh",
			want:  []string{"sh"},
		},
		{
			name:  "already a slice passthrough",
			input: []string{"sh", "-c", "echo hello"},
			want:  nil, // passthrough, not parsed
		},
		{
			name:  "empty string",
			input: "",
			want:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := transformCommand(ctx, tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("transformCommand() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.want == nil {
				return
			}
			gotSlice, ok := got.([]string)
			if !ok {
				t.Errorf("transformCommand() result type = %T, want []string", got)
				return
			}
			if len(gotSlice) != len(tt.want) {
				t.Errorf("transformCommand() = %v, want %v", gotSlice, tt.want)
				return
			}
			for i := range tt.want {
				if gotSlice[i] != tt.want[i] {
					t.Errorf("transformCommand()[%d] = %q, want %q", i, gotSlice[i], tt.want[i])
				}
			}
		})
	}
}

func Test_transformEnv(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		input   interface{}
		want    map[string]string
		wantErr bool
	}{
		{
			name:  "map input",
			input: map[string]interface{}{"FOO": "bar"},
			want:  map[string]string{"FOO": "bar"},
		},
		{
			name:  "list input with key=value",
			input: []interface{}{"FOO=bar", "BAZ=qux"},
			want:  map[string]string{"FOO": "bar", "BAZ": "qux"},
		},
		{
			name:  "list input key only gets empty value",
			input: []interface{}{"FOO"},
			want:  map[string]string{"FOO": ""},
		},
		{
			name:  "integer value in map is converted to string",
			input: map[string]interface{}{"FOO": 123},
			want:  map[string]string{"FOO": "123"},
		},
		{
			name:    "invalid input type errors",
			input:   "invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := transformEnv(ctx, tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("transformEnv() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			result := got.(map[string]string)
			if len(result) != len(tt.want) {
				t.Errorf("transformEnv() len = %d, want %d", len(result), len(tt.want))
				return
			}
			for k, wantVal := range tt.want {
				if result[k] != wantVal {
					t.Errorf("transformEnv()[%q] = %q, want %q", k, result[k], wantVal)
				}
			}
		})
	}
}
