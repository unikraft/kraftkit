package utils

import "testing"

func TestRewrapAsKraftCloudPackage(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "short name", input: "base", want: "index.unikraft.io/official/base"},
		{name: "namespaced name", input: "acme/base", want: "index.unikraft.io/acme/base"},
		{name: "legacy Unikraft registry", input: "unikraft.org/official/base", want: "index.unikraft.io/official/base"},
		{name: "Unikraft registry", input: "unikraft.io/official/base", want: "index.unikraft.io/official/base"},
		{name: "GitHub Container Registry", input: "ghcr.io/acme/base", want: "ghcr.io/acme/base"},
		{name: "registry with port", input: "localhost:5000/acme/base", want: "localhost:5000/acme/base"},
		{name: "registry URL", input: "https://localhost:5000/acme/base", want: "localhost:5000/acme/base"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RewrapAsKraftCloudPackage(tt.input); got != tt.want {
				t.Fatalf("RewrapAsKraftCloudPackage(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
