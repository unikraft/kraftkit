package make

import (
	"errors"
	"os"
	"reflect"
	"runtime"
	"sort"
	"testing"

	"kraftkit.sh/exec"
)

func TestNewMakeOptions(t *testing.T) {
	tests := []struct {
		name     string
		mopt     []MakeOption
		expected *MakeOptions
	}{
		{
			name:     "Default MakeOptions",
			mopt:     []MakeOption{},
			expected: &MakeOptions{},
		},
		{
			name: "string setters",
			mopt: []MakeOption{
				WithDirectory("/path/to/dir"),
				WithEvaluates("evaluate"),
				WithFile("/path/to/file1"),
				WithIncludeDir("/path/to/include"),
				WithOldFile("/path/to/old_file1"),
				WithAssumeNew("/path/to/new_file"),
				WithBinPath("/path/to/bin"),
				WithFile("/path/to/file2"),
				WithOldFile("/path/to/old_file2"),
			},
			expected: &MakeOptions{
				directory:   "/path/to/dir",
				evaluates:   []string{"evaluate"},
				files:       []string{"/path/to/file1", "/path/to/file2"},
				includeDirs: []string{"/path/to/include"},
				oldFiles:    []string{"/path/to/old_file1", "/path/to/old_file2"},
				newFiles:    []string{"/path/to/new_file"},
				bin:         "/path/to/bin",
			},
		},
		{
			name: "bool setters",
			mopt: []MakeOption{
				WithAlwaysMake(true),
				WithDebug(true),
				WithEnvOverrides(true),
				WithIgnoreErrors(true),
				WithKeepGoing(false),
				WithCheckSymlinkTimes(false),
				WithJustPrint(true),
				WithPrintDataBase(true),
				WithQuestion(false),
				WithNoBuiltinRules(false),
				WithNoBuiltinVariables(true),
				WithNoPrintDirectory(true),
				WithSilent(true),
				WithSyncOutput(true),
				WithTouch(false),
				WithTrace(false),
				WithVersion(false),
				WithPrintDirectory(true),
				WithWarnUndefinedVariables(true),
			},
			expected: &MakeOptions{
				alwaysMake:             true,
				debug:                  true,
				envOverrides:           true,
				ignoreErrors:           true,
				keepGoing:              false,
				checkSymlinkTimes:      false,
				justPrint:              true,
				printDataBase:          true,
				question:               false,
				noBuiltinRules:         false,
				noPrintDirectory:       true,
				noBuiltinVariables:     true,
				silent:                 true,
				syncOutput:             true,
				touch:                  false,
				trace:                  false,
				version:                false,
				printDirectory:         true,
				warnUndefinedVariables: true,
			},
		},
		{
			name: "int setters",
			mopt: []MakeOption{
				WithJobs(1),
				WithLoadAverage(4),
			},
			expected: &MakeOptions{
				jobs:        func(jobs int) *int { return &jobs }(1),
				loadAverage: func(loadAverage int) *int { return &loadAverage }(4),
			},
		},
		{
			name: "Var setters",
			mopt: []MakeOption{
				WithVar("CC", "gcc-14"),
				WithVar("CXX", "g++"),
				WithVar("GOPATH", "/usr/bin/go"),
				WithVar("😀", "😃"),
				WithVars(map[string]string{"CURL": "curl", "MKDIR": "mkdir", "CMAKE": "cmake", "GOOS": "linux", "GOARCH": "amd64"}),
			},
			expected: &MakeOptions{
				vars: map[string]string{
					"CC":     "gcc-14",
					"CXX":    "g++",
					"GOPATH": "/usr/bin/go",
					"😀":      "😃",
					"CURL":   "curl",
					"MKDIR":  "mkdir",
					"CMAKE":  "cmake",
					"GOOS":   "linux",
					"GOARCH": "amd64",
				},
			},
		},
		{
			name:     "Nil Vars setter",
			mopt:     []MakeOption{WithVars(nil)},
			expected: &MakeOptions{vars: map[string]string{}},
		},
		{
			name: "No Max Jobs",
			mopt: []MakeOption{
				WithMaxJobs(false),
			},
			expected: &MakeOptions{
				jobs: nil,
			},
		},
		{
			name: "With Max Jobs",
			mopt: []MakeOption{
				WithMaxJobs(true),
			},
			expected: &MakeOptions{
				jobs: func(jobs int) *int { return &jobs }(runtime.NumCPU()),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual, err := NewMakeOptions(test.mopt...)
			if err != nil {
				t.Fatalf("NewMakeOptions failed: %v", err)
			}

			if !reflect.DeepEqual(actual, test.expected) {
				t.Fatalf("%s: NewMakeOptions failed: got %#v, want %#v", test.name, actual, test.expected)
			}
		})
	}
}

func TestNewMakeOptionsReturnsError(t *testing.T) {
	mopts, err := NewMakeOptions(
		func(mo *MakeOptions) error {
			return errors.New("intentional Error")
		},
	)
	if mopts != nil {
		t.Fatalf("NewMakeOptions failed: got %#v, want nil on error", mopts)
	}

	if err == nil {
		t.Fatalf("NewMakeOptions failed: got nil, want error")
	}

	expectedError := errors.New("could not apply option: intentional Error")
	if err.Error() != expectedError.Error() {
		t.Fatalf("NewMakeOptions failed: got error %#v, want %#v", err, expectedError)
	}
}

func TestNewMakeOptionsWithOnProgress(t *testing.T) {
	tests := []struct {
		name          string
		args          []float64
		hasOnProgress bool
	}{
		{name: "Nil onProgress", args: nil, hasOnProgress: false},
		{name: "zero progress", args: []float64{0.0}, hasOnProgress: true},
		{name: "mid progress", args: []float64{0.5}, hasOnProgress: true},
		{name: "complete", args: []float64{1.0}, hasOnProgress: true},
		{name: "negative value", args: []float64{-1.0}, hasOnProgress: true},
		{name: "over complete", args: []float64{1.5}, hasOnProgress: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var captured float64

			var fn func(float64)

			if test.hasOnProgress {
				fn = func(val float64) { captured = val }
			}

			mopts, err := NewMakeOptions(WithProgressFunc(fn))
			if err != nil {
				t.Fatalf("NewMakeOptions failed: %v", err)
			}

			if test.hasOnProgress && mopts.onProgress == nil {
				t.Fatalf("NewMakeOptions failed: got nil, want non-nil")
			}

			if !test.hasOnProgress && mopts.onProgress != nil {
				t.Fatalf("NewMakeOptions failed: got non-nil, want nil")
			}

			for _, arg := range test.args {
				mopts.onProgress(arg)
				if captured != arg {
					t.Fatalf("NewMakeOptions failed: got %v, want %v", captured, arg)
				}
			}
		})
	}
}

func TestNewMakeOptionsWithExecOptions(t *testing.T) {
	tests := []struct {
		name    string
		eopts   []exec.ExecOption
		wantLen int
	}{
		{
			name:    "Nil ExecOptions",
			eopts:   nil,
			wantLen: 0,
		},
		{
			name:    "Single ExecOptions",
			eopts:   []exec.ExecOption{exec.WithDetach(false)},
			wantLen: 1,
		},
		{
			name:    "Multi ExecOptions",
			eopts:   []exec.ExecOption{exec.WithEnvKey("CC", "gcc-14"), exec.WithDetach(true), exec.WithStdin(os.Stdin)},
			wantLen: 3,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mopts, err := NewMakeOptions(WithExecOptions(test.eopts...))
			if err != nil {
				t.Fatalf("NewMakeOptions failed: %v", err)
			}

			if test.wantLen != len(mopts.eopts) {
				t.Fatalf("NewMakeOptions failed: got length %v, want length %v", len(mopts.eopts), test.wantLen)
			}
		})
	}
	t.Run("Multiple WithExecOptions Call", func(t *testing.T) {
		wantLen := 3
		mopts, err := NewMakeOptions(
			WithExecOptions(exec.WithStdout(os.Stdout)),
			WithExecOptions(exec.WithStderr(os.Stderr)),
			WithExecOptions(exec.WithStdin(os.Stdin)),
		)
		if err != nil {
			t.Fatalf("NewMakeOptions failed: %v", err)
		}

		if wantLen != len(mopts.eopts) {
			t.Fatalf("NewMakeOptions failed: got length %v, want length %v", len(mopts.eopts), wantLen)
		}
	})
}

func TestMakeOptions_Vars(t *testing.T) {
	mopts := &MakeOptions{
		vars: map[string]string{
			"CC":     "gcc-14",
			"CXX":    "g++",
			"GOPATH": "/usr/bin/go",
			"😀":      "😃",
			"CURL":   "curl",
			"MKDIR":  "mkdir",
			"CMAKE":  "cmake",
			"GOOS":   "linux",
			"GOARCH": "amd64",
		},
		targets: []string{"build", "run", "dev", "clean"},
	}
	want := []string{
		"CC=gcc-14",
		"CXX=g++",
		"GOPATH=/usr/bin/go",
		"MKDIR=mkdir", "GOOS=linux",
		"GOARCH=amd64",
		"CURL=curl",
		"CMAKE=cmake",
		"😀=😃",
		"build",
		"clean",
		"run",
		"dev",
	}
	t.Run("Testing MakeOptions Vars", func(t *testing.T) {
		actual := mopts.Vars()
		sort.Strings(actual)
		sort.Strings(want)
		if !reflect.DeepEqual(actual, want) {
			t.Fatalf("MakeOptions Vars failed: got %v, want %v", actual, want)
		}
	})
}

func TestMakeOptions_WithTarget(t *testing.T) {
	tests := []struct {
		name    string
		targets []string
		wantLen int
	}{
		{
			name:    "Empty Targets",
			targets: nil,
			wantLen: 0,
		},
		{
			name:    "Single Target",
			targets: []string{"build"},
			wantLen: 1,
		},
		{
			name:    "Multi Target",
			targets: []string{"build", "run", "dev", "clean"},
			wantLen: 4,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mopts, err := NewMakeOptions(WithTarget(test.targets...))
			if err != nil {
				t.Fatalf("NewMakeOptions failed: %v", err)
			}

			if test.wantLen != len(mopts.targets) {
				t.Fatalf("NewMakeOptions failed: got length %v, want length %v", len(mopts.targets), test.wantLen)
			}

			if !reflect.DeepEqual(mopts.targets, test.targets) {
				t.Fatalf("NewMakeOptions failed: got %v, want %v", mopts.targets, test.targets)
			}
		})
	}
	t.Run("Multiple WithTarget Args", func(t *testing.T) {
		targets := []string{"build", "run", "dev", "clean"}
		mopts, err := NewMakeOptions(WithTarget("build", "run"), WithTarget("dev"), WithTarget("clean"))
		if err != nil {
			t.Fatalf("NewMakeOptions failed: %v", err)
		}

		if len(targets) != len(mopts.targets) {
			t.Fatalf("NewMakeOptions failed: got %v, want %v", len(mopts.targets), len(targets))
		}

		if !reflect.DeepEqual(mopts.targets, targets) {
			t.Fatalf("NewMakeOptions failed: got %v, want %v", mopts.targets, targets)
		}
	})
}
