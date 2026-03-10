package volume

import (
	"context"
	"testing"
)

func Test_TransformFromSchema(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name       string
		input      interface{}
		wantSource string
		wantDest   string
		wantDriver string
		wantRO     bool
		wantErr    bool
	}{
		{
			name:       "string with source:destination",
			input:      "/host/path:/container/path",
			wantSource: "/host/path",
			wantDest:   "/container/path",
		},
		{
			name:       "string with no colon defaults destination to /",
			input:      "/host/path",
			wantSource: "/host/path",
			wantDest:   "/",
		},
		{
			name:    "string with more than one colon returns error",
			input:   "/a:/b:/c",
			wantErr: true,
		},
		{
			name:       "map with source and destination",
			input:      map[string]interface{}{"source": "/src", "destination": "/dst"},
			wantSource: "/src",
			wantDest:   "/dst",
		},
		{
			name:       "map with driver",
			input:      map[string]interface{}{"source": "/src", "driver": "9pfs"},
			wantSource: "/src",
			wantDriver: "9pfs",
		},
		{
			name:       "map with readonly true",
			input:      map[string]interface{}{"source": "/src", "readonly": true},
			wantSource: "/src",
			wantRO:     true,
		},
		{
			name:       "map with readonly false",
			input:      map[string]interface{}{"source": "/src", "readonly": false},
			wantSource: "/src",
			wantRO:     false,
		},
		{
			name:    "map with non-string driver returns error",
			input:   map[string]interface{}{"driver": 123},
			wantErr: true,
		},
		{
			name:    "map with non-string source returns error",
			input:   map[string]interface{}{"source": 123},
			wantErr: true,
		},
		{
			name:    "map with non-string destination returns error",
			input:   map[string]interface{}{"destination": 123},
			wantErr: true,
		},
		{
			name:    "map with non-bool readonly returns error",
			input:   map[string]interface{}{"readonly": "yes"},
			wantErr: true,
		},
		{
			name:       "empty string input",
			input:      "",
			wantSource: "",
			wantDest:   "/",
		},
		{
			name:  "empty map input returns empty VolumeConfig",
			input: map[string]interface{}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := TransformFromSchema(ctx, tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("TransformFromSchema() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			vol, ok := got.(VolumeConfig)
			if !ok {
				t.Fatalf("TransformFromSchema() result type = %T, want VolumeConfig", got)
			}

			if vol.source != tt.wantSource {
				t.Errorf("source = %q, want %q", vol.source, tt.wantSource)
			}
			if vol.destination != tt.wantDest {
				t.Errorf("destination = %q, want %q", vol.destination, tt.wantDest)
			}
			if vol.driver != tt.wantDriver {
				t.Errorf("driver = %q, want %q", vol.driver, tt.wantDriver)
			}
			if vol.readOnly != tt.wantRO {
				t.Errorf("readOnly = %v, want %v", vol.readOnly, tt.wantRO)
			}
		})
	}
}
