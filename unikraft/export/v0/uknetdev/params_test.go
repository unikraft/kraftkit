// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package uknetdev

import (
	"testing"
)

func TestExportedParams(t *testing.T) {
	params := ExportedParams()
	if len(params) != 1 {
		t.Fatalf("expected 1 param, got %d", len(params))
	}
	if got := params[0].Name(); got != "netdev.ip" {
		t.Errorf("got %q, want %q", got, "netdev.ip")
	}
}

func TestNewParamIp(t *testing.T) {
	p := NewParamIp()
	if got := p.Name(); got != "netdev.ip" {
		t.Errorf("got %q, want %q", got, "netdev.ip")
	}
}

func TestNetdevIp_String(t *testing.T) {
	tests := []struct {
		name  string
		entry NetdevIp
		want  string
	}{
		{
			name: "full entry",
			entry: NetdevIp{
				CIDR:     "172.44.0.2/24",
				Gateway:  "172.44.0.1",
				DNS0:     "8.8.8.8",
				DNS1:     "8.8.4.4",
				Hostname: "myhost",
				Domain:   "example.com",
			},
			want: "172.44.0.2/24:172.44.0.1:8.8.8.8:8.8.4.4:myhost:example.com",
		},
		{
			name:  "empty entry",
			entry: NetdevIp{},
			want:  ":::::",
		},
		{
			name: "partial entry",
			entry: NetdevIp{
				CIDR:    "10.0.0.1/24",
				Gateway: "10.0.0.254",
			},
			want: "10.0.0.1/24:10.0.0.254::::",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.entry.String(); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
