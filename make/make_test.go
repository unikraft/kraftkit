package make

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"kraftkit.sh/exec"
)

type parseExportTestCase struct {
	name     string
	input    interface{}
	expected *export
	err      error
}

func TestParseExport(t *testing.T) {
	tests := []parseExportTestCase{
		{
			name: "Export tag without omitempty",
			input: struct {
				outputDir string `export:"O"`
			}{},
			expected: &export{
				export:    "O",
				omitempty: false,
			},
			err: nil,
		},
		{
			name: "Export tag with omitempty",
			input: struct {
				appDir string `export:"A,omitempty"`
			}{},
			expected: &export{
				export:    "A",
				omitempty: true,
			},
			err: nil,
		},
		{
			name: "Export tag with omitempty and default value",
			input: struct {
				configPath string `export:"C,omitempty" default:"/app/dir/config/"`
			}{},
			expected: &export{
				export:    "C",
				omitempty: true,
				def:       "/app/dir/config/",
			},
			err: nil,
		},
		{
			name: "Export tag only with default value",
			input: struct {
				verbosity string `export:"V" default:"Verbose"`
			}{},
			expected: &export{
				export:    "V",
				omitempty: false,
				def:       "Verbose",
			},
			err: nil,
		},
		{
			name: "Struct with no tags",
			input: struct {
				outputDir string
			}{},
			expected: &export{},
			err:      nil,
		},
		{
			name: "Struct with tags other than export",
			input: struct {
				OutputDir string `import:"I, omitempty" json:"output" default:"dir"`
			}{},
			expected: &export{},
			err:      nil,
		},
	}

	testRunner(tests, t)
}

// core.MakeArgs cannot be imported here due to import cycle
// (core imports make package). This inline struct mirrors core.MakeArgs
// tags to verify parseExport behavior against the same tag conventions.
func TestParseExport_MakeArgs(t *testing.T) {
	t.Run("Check MakeArgs export tags", func(t *testing.T) {
		t.Parallel()
		var actual, expected []export
		input := struct {
			OutputDir      string `export:"O,omitempty"`
			ApplicationDir string `export:"A"`
			PlatformDirs   string `export:"P,omitempty"`
			LibraryDirs    string `export:"L,omitempty"`
			Name           string `export:"N,omitempty"`
			ConfigPath     string `export:"C,omitempty"`
		}{}
		expected = []export{
			{export: "O", omitempty: true},
			{export: "A", omitempty: false},
			{export: "P", omitempty: true},
			{export: "L", omitempty: true},
			{export: "N", omitempty: true},
			{export: "C", omitempty: true},
		}
		tags, err := makeTag(input)
		if err != nil {
			t.Fatal("could export tags:", err)
		}

		for _, tag := range tags {
			export, err := parseExport(tag)
			if err != nil {
				t.Fatal("could not parse export tag:", err)
			}
			actual = append(actual, *export)
		}

		if !reflect.DeepEqual(actual, expected) {
			t.Errorf("got %v\nwant %v", actual, expected)
		}
	})
}

func TestParseExport_Errors(t *testing.T) {
	tests := []parseExportTestCase{
		{
			name: "Struct with Export tag with no export name",
			input: struct {
				outputDir string `export:",omitempty" default:"def"`
			}{},
			expected: nil,
			err:      fmt.Errorf("export tag cannot be empty"),
		},
	}

	testRunner(tests, t)
}

// Note: errors from exec.NewExecutable, and exec.NewSequential
// are unreachable in practice given NewFromInterface
// always sets opts.bin before calling and passes MakeOptions by value.
func TestNewFromInterface_Errors(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		mopts    []MakeOption
		expected *Make
		err      error
	}{
		{
			name:     "Non-struct type passed",
			input:    24,
			mopts:    []MakeOption{},
			expected: nil,
			err:      fmt.Errorf("expected struct type, got non-struct type: int"),
		},
		{
			name: "Args passed by reference",
			input: &struct {
				name string `export:"-"`
			}{},
			mopts:    []MakeOption{WithTarget("build", "clean")},
			expected: nil,
			err:      fmt.Errorf("cannot derive interface arguments from pointer: passed by reference"),
		},
		{
			name: "Args with empty export tag name",
			input: struct {
				name string `export:",omitempty"`
			}{},
			mopts:    []MakeOption{WithTarget("build", "clean"), WithDebug(true)},
			expected: nil,
			err:      fmt.Errorf("could not parse export tag: export tag cannot be empty"),
		},
		{
			name: "Error from NewMakeOptions",
			input: struct {
				outputDir string `export:"O,omitempty"`
				bin       string `export:"B"`
			}{},
			mopts:    []MakeOption{func(mo *MakeOptions) error { return fmt.Errorf("intentional make error") }},
			expected: nil,
			err:      fmt.Errorf("could not apply option: intentional make error"),
		},
		{
			name: "Error from ExecOptions",
			input: struct {
				outputDir string `export:"O,omitempty"`
			}{},
			mopts: []MakeOption{WithExecOptions(func(eo *exec.ExecOptions) error {
				return fmt.Errorf("intentional exec error")
			})},
			expected: nil,
			err:      fmt.Errorf("could not apply option: intentional exec error"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			actual, err := NewFromInterface(test.input, test.mopts...)
			if !reflect.DeepEqual(actual, test.expected) {
				t.Fatalf("got %v, want %v", actual, test.expected)
			}

			assertError(t, err, test.err)
		})
	}
}

func TestNewFromInterface(t *testing.T) {
	tests := []struct {
		name          string
		args          interface{}
		mopts         []MakeOption
		expected      *MakeOptions
		hasCPW        bool
		hasOnProgress bool
		wantExecLen   int
	}{
		{
			name: "Make with only mainExec",
			args: struct {
				outputDir  string `export:"O,omitempty"`
				configPath string `export:"B,omitempty"`
				name       string `export:"N"`
			}{
				outputDir: "dir",
				name:      "name",
			},
			mopts: []MakeOption{WithTarget("build", "clean"), WithDebug(true), WithBinPath("/tmp/bin")},
			expected: mustMakeOptions(
				t,
				WithTarget("build", "clean"),
				WithDebug(true),
				WithVar("O", "dir"),
				WithVar("N", "name"),
				WithBinPath("/tmp/bin"),
			),
			hasCPW:        false,
			hasOnProgress: false,
			wantExecLen:   0,
		},
		{
			name: "Make with onProgress and mainExec",
			args: struct {
				outputDir  string `export:"O,omitempty"`
				configPath string `export:"C" default:"/path/to/config.yml"`
				name       string `export:"N,omitempty"`
			}{
				outputDir:  "dir",
				configPath: "",
			},
			mopts: []MakeOption{
				WithTarget("clean"),
				WithProgressFunc(func(f float64) { fmt.Println("progress: ", f) }),
				WithExecOptions(exec.WithDetach(true)),
			},
			expected: mustMakeOptions(
				t,
				WithTarget("clean"),
				WithProgressFunc(func(f float64) { fmt.Println("progress: ", f) }),
				WithVar("O", "dir"),
				WithVar("C", "/path/to/config.yml"),
				WithExecOptions(exec.WithDetach(true)),
			),
			hasCPW:        true,
			hasOnProgress: true,
			wantExecLen:   2,
		},
		{
			name: "Make with onProgress, mainExec and justPrint",
			args: struct {
				outputDir  string `export:"O,omitempty"`
				configPath string `export:"C"`
			}{
				outputDir:  "/path/to/dir",
				configPath: "/path/to/config",
			},
			mopts: []MakeOption{
				WithDebug(true),
				WithProgressFunc(func(f float64) { fmt.Println("progress: ", f) }),
				WithJustPrint(true),
				WithExecOptions(exec.WithDetach(true)),
			},
			expected: mustMakeOptions(
				t,
				WithDebug(true),
				WithProgressFunc(func(f float64) { fmt.Println("progress: ", f) }),
				WithJustPrint(true),
				WithVar("O", "/path/to/dir"),
				WithVar("C", "/path/to/config"),
				WithExecOptions(exec.WithDetach(true)),
			),
			hasCPW:        false,
			hasOnProgress: true,
			wantExecLen:   1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			actual, err := NewFromInterface(test.args, test.mopts...)
			if err != nil {
				t.Fatal(err)
			}
			test.expected.bin = actual.opts.bin

			// func fields cannot be compared with reflect.DeepEqual
			// assert meaningful properties separately then nil out before comparison
			if test.hasOnProgress {
				if actual.opts.onProgress == nil {
					t.Fatal("got nil onProgress, expected non-nil")
				}
			} else {
				if actual.opts.onProgress != nil {
					t.Fatal("got non-nil onProgress, expected nil")
				}
			}

			if len(actual.opts.eopts) != test.wantExecLen {
				t.Fatalf("got %d, want %d", len(actual.opts.eopts), test.wantExecLen)
			}
			test.expected.eopts = nil
			actual.opts.eopts = nil
			test.expected.onProgress = nil
			actual.opts.onProgress = nil

			if !reflect.DeepEqual(actual.opts, test.expected) {
				t.Fatalf("got %#v, want %#v", actual.opts, test.expected)
			}

			if test.hasCPW && actual.cpw == nil {
				t.Fatal("got nil cpw, expected non-nil")
			}

			if !test.hasCPW && actual.cpw != nil {
				t.Fatal("got non-nil cpw, expected nil")
			}

			if actual.seq == nil {
				t.Fatal("got nil, want non-nil")
			}
		})
	}
}

func TestMakeExecute(t *testing.T) {
	m, err := NewFromInterface(struct{}{}, WithBinPath("/tmp/make"))
	err = m.Execute(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func mustMakeOptions(t *testing.T, mopts ...MakeOption) *MakeOptions {
	t.Helper()
	opts, err := NewMakeOptions(mopts...)
	if err != nil {
		t.Fatal("failed to create make options:", err)
	}
	return opts
}

func testRunner(tests []parseExportTestCase, t *testing.T) {
	t.Helper()
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			tags, err := makeTag(test.input)
			if err != nil {
				t.Fatalf("cannot extract tags: %v", err)
			}
			for _, tag := range tags {
				testTags(tag, test.expected, test.err, t)
			}
		})
	}
}

func testTags(tag reflect.StructTag, expected *export, expectedErr error, t *testing.T) {
	t.Helper()
	actual, err := parseExport(tag)

	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("expected %v, got %v", expected, actual)
	}
	assertError(t, err, expectedErr)
}

func assertError(t *testing.T, err error, expectedErr error) {
	t.Helper()
	if err != nil && expectedErr == nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if err == nil && expectedErr != nil {
		t.Fatalf("expected %v, got no error", expectedErr)
	}

	if expectedErr != nil && err != nil && err.Error() != expectedErr.Error() {
		t.Fatalf("expected error: %v, got: %v", expectedErr, err)
	}
}

func makeTag(input interface{}) ([]reflect.StructTag, error) {
	t := reflect.TypeOf(input)

	if t.Kind() != reflect.Struct {
		return []reflect.StructTag{}, fmt.Errorf("expected struct, got %s", t.Kind())
	}

	var tags []reflect.StructTag

	if t.NumField() == 0 {
		return tags, nil
	}

	for i := 0; i < t.NumField(); i++ {
		tags = append(tags, t.Field(i).Tag)
	}
	return tags, nil
}
