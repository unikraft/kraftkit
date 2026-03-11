package template

import (
	"context"
	"testing"

	"kraftkit.sh/kconfig"
	"kraftkit.sh/unikraft"
)

func TestTemplateConfig_Accessors(t *testing.T) {
	tc := TemplateConfig{
		name:    "my-template",
		version: "0.15.0",
		source:  "https://github.com/unikraft/app-helloworld",
		path:    "/tmp/template",
	}

	if got := tc.Name(); got != "my-template" {
		t.Errorf("Name() = %q, want %q", got, "my-template")
	}
	if got := tc.String(); got != "my-template" {
		t.Errorf("String() = %q, want %q", got, "my-template")
	}
	if got := tc.Version(); got != "0.15.0" {
		t.Errorf("Version() = %q, want %q", got, "0.15.0")
	}
	if got := tc.Source(); got != "https://github.com/unikraft/app-helloworld" {
		t.Errorf("Source() = %q, want %q", got, "https://github.com/unikraft/app-helloworld")
	}
	if got := tc.Path(); got != "/tmp/template" {
		t.Errorf("Path() = %q, want %q", got, "/tmp/template")
	}
	if got := tc.Type(); got != unikraft.ComponentTypeApp {
		t.Errorf("Type() = %v, want %v", got, unikraft.ComponentTypeApp)
	}
}

func TestTemplateConfig_Accessors_ZeroValues(t *testing.T) {
	tc := TemplateConfig{}

	if got := tc.Name(); got != "" {
		t.Errorf("Name() = %q, want empty string", got)
	}
	if got := tc.String(); got != "" {
		t.Errorf("String() = %q, want empty string", got)
	}
	if got := tc.Version(); got != "" {
		t.Errorf("Version() = %q, want empty string", got)
	}
	if got := tc.Source(); got != "" {
		t.Errorf("Source() = %q, want empty string", got)
	}
	if got := tc.Path(); got != "" {
		t.Errorf("Path() = %q, want empty string", got)
	}
	if got := tc.KConfig(); got != nil {
		t.Errorf("KConfig() = %v, want nil", got)
	}
}

func TestTemplateConfig_KConfigTree(t *testing.T) {
	tc := TemplateConfig{name: "my-template"}
	tree, err := tc.KConfigTree(context.Background())
	if err != nil {
		t.Errorf("KConfigTree() unexpected error = %v", err)
	}
	if tree != nil {
		t.Errorf("KConfigTree() = %v, want nil", tree)
	}
}

func TestTemplateConfig_PrintInfo(t *testing.T) {
	tc := TemplateConfig{name: "my-template"}
	got := tc.PrintInfo(context.Background())
	if got == "" {
		t.Errorf("PrintInfo() returned empty string")
	}
}

func TestTemplateConfig_MarshalYAML(t *testing.T) {
	tests := []struct {
		name     string
		tc       TemplateConfig
		wantNil  bool
		wantKeys map[string]any
	}{
		{
			name:    "empty config returns nil",
			tc:      TemplateConfig{},
			wantNil: true,
		},
		{
			name: "name only",
			tc:   TemplateConfig{name: "my-template"},
			wantKeys: map[string]interface{}{
				"name": "my-template",
			},
		},
		{
			name: "version only",
			tc:   TemplateConfig{version: "0.15.0"},
			wantKeys: map[string]interface{}{
				"version": "0.15.0",
			},
		},
		{
			name: "source only",
			tc:   TemplateConfig{source: "https://github.com/unikraft/app-helloworld"},
			wantKeys: map[string]interface{}{
				"source": "https://github.com/unikraft/app-helloworld",
			},
		},
		{
			name: "all fields populated",
			tc: TemplateConfig{
				name:    "my-template",
				version: "0.15.0",
				source:  "https://github.com/unikraft/app-helloworld",
			},
			wantKeys: map[string]interface{}{
				"name":    "my-template",
				"version": "0.15.0",
				"source":  "https://github.com/unikraft/app-helloworld",
			},
		},
		{
			name: "kconfig included when non-empty",
			tc: TemplateConfig{
				name:    "my-template",
				kconfig: kconfig.KeyValueMap{"CONFIG_FOO": &kconfig.KeyValue{Value: "y"}},
			},
			wantKeys: map[string]interface{}{
				"name": "my-template",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.tc.MarshalYAML()
			if err != nil {
				t.Fatalf("MarshalYAML() unexpected error = %v", err)
			}
			if tt.wantNil {
				if got != nil {
					t.Errorf("MarshalYAML() = %v, want nil", got)
				}
				return
			}
			result, ok := got.(map[string]interface{})
			if !ok {
				t.Fatalf("MarshalYAML() result type = %T, want map[string]interface{}", got)
			}
			for k, wantVal := range tt.wantKeys {
				gotVal, exists := result[k]
				if !exists {
					t.Errorf("MarshalYAML() missing key %q", k)
					continue
				}
				if gotVal != wantVal {
					t.Errorf("MarshalYAML()[%q] = %v, want %v", k, gotVal, wantVal)
				}
			}
		})
	}
}

func TestNewTemplateFromOptions(t *testing.T) {
	tmpl, err := NewTemplateFromOptions(
		WithName("my-template"),
		WithVersion("0.15.0"),
		WithSource("https://github.com/unikraft/app-helloworld"),
	)
	if err != nil {
		t.Fatalf("NewTemplateFromOptions() unexpected error = %v", err)
	}
	if tmpl.Name() != "my-template" {
		t.Errorf("Name() = %q, want %q", tmpl.Name(), "my-template")
	}
	if tmpl.Version() != "0.15.0" {
		t.Errorf("Version() = %q, want %q", tmpl.Version(), "0.15.0")
	}
	if tmpl.Source() != "https://github.com/unikraft/app-helloworld" {
		t.Errorf("Source() = %q, want %q", tmpl.Source(), "https://github.com/unikraft/app-helloworld")
	}
}

func TestNewTemplateFromOptions_NoOptions(t *testing.T) {
	tmpl, err := NewTemplateFromOptions()
	if err != nil {
		t.Fatalf("NewTemplateFromOptions() unexpected error = %v", err)
	}
	if tmpl.Name() != "" {
		t.Errorf("Name() = %q, want empty string", tmpl.Name())
	}
}

func TestTransformFromSchema(t *testing.T) {
	tests := []struct {
		name        string
		ctx         context.Context
		input       any
		wantName    string
		wantVersion string
		wantErr     bool
	}{
		{
			// A plain string is not a URL or local path, so parseStringProp
			// treats it as a version, leaving name empty.
			name:        "plain string sets version not name",
			ctx:         context.Background(),
			input:       "0.15.0",
			wantVersion: "0.15.0",
		},
		{
			name: "map input with name",
			ctx:  context.Background(),
			input: map[string]interface{}{
				"name": "my-template",
			},
			wantName: "my-template",
		},
		{
			name: "map input with name and version",
			ctx:  context.Background(),
			input: map[string]interface{}{
				"name":    "my-template",
				"version": "0.15.0",
			},
			wantName:    "my-template",
			wantVersion: "0.15.0",
		},
		{
			// Unrecognised types pass through TranslateFromSchema silently and
			// return an empty TemplateConfig with no error.
			name:  "integer input returns empty config",
			ctx:   context.Background(),
			input: 42,
		},
		{
			name: "with UK_BASE context and map name sets path",
			ctx: unikraft.WithContext(context.Background(), &unikraft.Context{
				UK_BASE: "/tmp/unikraft-base",
			}),
			input: map[string]interface{}{
				"name": "my-template",
			},
			wantName: "my-template",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := TransformFromSchema(tt.ctx, tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("TransformFromSchema() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			tc, ok := got.(TemplateConfig)
			if !ok {
				t.Fatalf("TransformFromSchema() result type = %T, want TemplateConfig", got)
			}
			if tc.Name() != tt.wantName {
				t.Errorf("TransformFromSchema() Name() = %q, want %q", tc.Name(), tt.wantName)
			}
			if tc.Version() != tt.wantVersion {
				t.Errorf("TransformFromSchema() Version() = %q, want %q", tc.Version(), tt.wantVersion)
			}
		})
	}
}
