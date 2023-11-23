// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package make

import (
	"fmt"

	"kraftkit.sh/exec"
)

// MakeOptions represents all the command-line arguments which can be passed to
// the invocation of GNU Make.
type MakeOptions struct {
	alwaysMake             bool     `flag:"-B"`
	checkSymlinkTimes      bool     `flag:"-L"`
	debug                  bool     `flag:"-d"`
	directory              string   `flag:"-C"`
	envOverrides           bool     `flag:"-e"`
	evaluates              []string `flag:"-E"`
	files                  []string `flag:"-f"`
	ignoreErrors           bool     `flag:"-i"`
	includeDirs            []string `flag:"-I"`
	jobs                   *int     `flag:"-j,omitvalueif=0"`
	justPrint              bool     `flag:"-n"`
	keepGoing              bool     `flag:"-k"`
	loadAverage            *int     `flag:"-l"`
	newFiles               []string `flag:"-W"`
	noBuiltinRules         bool     `flag:"-r"`
	noBuiltinVariables     bool     `flag:"-R"`
	noPrintDirectory       bool     `flag:"--no-print-directory"`
	oldFiles               []string `flag:"-o"`
	printDataBase          bool     `flag:"-p"`
	printDirectory         bool     `flag:"-w"`
	question               bool     `flag:"-q"`
	silent                 bool     `flag:"-s"`
	syncOutput             bool     `flag:"-O"`
	touch                  bool     `flag:"-t"`
	trace                  bool     `flag:"--trace"`
	version                bool     `flag:"-v"`
	warnUndefinedVariables bool     `flag:"--warn-undefined-variables"`

	bin        string
	targets    []string
	vars       map[string]string
	onProgress func(float64)
	eopts      []exec.ExecOption
}

type MakeOption func(mo *MakeOptions) error

// NewMakeOptions
func NewMakeOptions(mopts ...MakeOption) (*MakeOptions, error) {
	mo := &MakeOptions{}

	for _, o := range mopts {
		if err := o(mo); err != nil {
			return nil, fmt.Errorf("could not apply option: %v", err)
		}
	}

	return mo, nil
}

// Vars returns serialized slice of Make variables which are passed as arguments
// to make along with all CLI flags
func (mo *MakeOptions) Vars() []string {
	var vars []string

	for k, v := range mo.vars {
		vars = append(vars, k+"="+v)
	}

	vars = append(vars, mo.targets...)

	return vars
}

// Unconditionally make all targets.  Equivalent to calling the flags
// -B|--always-make
func WithAlwaysMake(alwaysMake bool) MakeOption {
	return func(mo *MakeOptions) error {
		mo.alwaysMake = alwaysMake
		return nil
	}
}

// Change to Directory before doing anything.  Equivalent to calling the flags
// -C|--directory
func WithDirectory(dir string) MakeOption {
	return func(mo *MakeOptions) error {
		mo.directory = dir
		return nil
	}
}

// Print lots of debugging information.  Equivalent to calling the flags
// -d|--debug
func WithDebug(debug bool) MakeOption {
	return func(mo *MakeOptions) error {
		mo.debug = debug
		return nil
	}
}

// Environment variables override makefiles.  Equivalent to calling the flags
// -e|--environment-overrides
func WithEnvOverrides(envOverrides bool) MakeOption {
	return func(mo *MakeOptions) error {
		mo.envOverrides = envOverrides
		return nil
	}
}

// WithVar sets a variable and its value before invoking make.
func WithVar(key, val string) MakeOption {
	return func(mo *MakeOptions) error {
		if mo.vars == nil {
			mo.vars = make(map[string]string)
		}

		mo.vars[key] = val

		return nil
	}
}

// WithVars sets a map of additional variables before invoking make.
func WithVars(vars map[string]string) MakeOption {
	return func(mo *MakeOptions) error {
		if mo.vars == nil {
			mo.vars = make(map[string]string)
		}

		for key, val := range vars {
			mo.vars[key] = val
		}

		return nil
	}
}

// Evaluate strings as makefile statements.  Equivalent to calling the flags
// -E|--eval
func WithEvaluates(evaluates string) MakeOption {
	return func(mo *MakeOptions) error {
		if mo.evaluates == nil {
			mo.evaluates = make([]string, 0)
		}

		mo.evaluates = append(mo.evaluates, evaluates)

		return nil
	}
}

// Read files as a makefile.  Equivalent to calling the flags
// -f|--file|--makefile
func WithFile(file string) MakeOption {
	return func(mo *MakeOptions) error {
		if mo.files == nil {
			mo.files = make([]string, 0)
		}

		mo.files = append(mo.files, file)

		return nil
	}
}

// Ignore errors from recipes.  Equivalent to calling the flags
// -i|--ignore-errors
func WithIgnoreErrors(ignoreErrors bool) MakeOption {
	return func(mo *MakeOptions) error {
		mo.ignoreErrors = ignoreErrors
		return nil
	}
}

// Search directories for included makefiles.  Equivalent to calling the flags
// `-I|--include-dir
func WithIncludeDir(dir string) MakeOption {
	return func(mo *MakeOptions) error {
		if mo.includeDirs == nil {
			mo.includeDirs = make([]string, 0)
		}

		mo.includeDirs = append(mo.includeDirs, dir)

		return nil
	}
}

// Allow N jobs at once; infinite jobs with no arg.  Equivalent to calling the
// flags -j|--jobs with a value
func WithJobs(jobs int) MakeOption {
	return func(mo *MakeOptions) error {
		mo.jobs = &jobs
		return nil
	}
}

// Allow N jobs at once; infinite jobs with no arg.  Equivalent to calling the
// flags -j|--jobs with a value
func WithMaxJobs(maxJobs bool) MakeOption {
	return func(mo *MakeOptions) error {
		if maxJobs {
			zero := 0
			mo.jobs = &zero
		} else {
			mo.jobs = nil
		}

		return nil
	}
}

// Keep going when some targets can't be made.  Equivalent to calling the flags
// -k|--keep-going
func WithKeepGoing(keepGoing bool) MakeOption {
	return func(mo *MakeOptions) error {
		mo.keepGoing = keepGoing
		return nil
	}
}

// Don't start multiple jobs unless load is below N.  Equivalent to calling the
// flags -l|--load-average|--max-load
func WithLoadAverage(loadAverage int) MakeOption {
	return func(mo *MakeOptions) error {
		mo.loadAverage = &loadAverage
		return nil
	}
}

// Use the latest mtime between symlinks and target.  Equivalent to calling the
// flags -L|--check-symlink-times
func WithCheckSymlinkTimes(cst bool) MakeOption {
	return func(mo *MakeOptions) error {
		mo.checkSymlinkTimes = cst
		return nil
	}
}

// Don't actually run any recipe; just print them.  Equivalent to calling the
// flags -n|--just-print|--dry-run|--recon
func WithJustPrint(justPrint bool) MakeOption {
	return func(mo *MakeOptions) error {
		mo.justPrint = justPrint
		return nil
	}
}

// Consider these files to be very old and don't remake them.  Equivalent to
// calling the flags -o|--old-file|--assume-old
func WithOldFile(file string) MakeOption {
	return func(mo *MakeOptions) error {
		if mo.oldFiles == nil {
			mo.oldFiles = make([]string, 0)
		}

		mo.oldFiles = append(mo.oldFiles, file)

		return nil
	}
}

// Print make's internal database.  Equivalent to calling the flags
// -p|--print-data-base
func WithPrintDataBase(printDataBase bool) MakeOption {
	return func(mo *MakeOptions) error {
		mo.printDataBase = printDataBase
		return nil
	}
}

// Run no recipe; exit status says if up to date.  Equivalent to calling the
// flags -q|--question
func WithQuestion(question bool) MakeOption {
	return func(mo *MakeOptions) error {
		mo.question = question
		return nil
	}
}

// Disable the built-in implicit rules.  Equivalent to calling the flags
// -r|--no-builtin-rules
func WithNoBuiltinRules(nbr bool) MakeOption {
	return func(mo *MakeOptions) error {
		mo.noBuiltinRules = nbr
		return nil
	}
}

// Disable the built-in variable settings.  Equivalent to calling the flags
// -R|--no-builtin-variables
func WithNoBuiltinVariables(nbv bool) MakeOption {
	return func(mo *MakeOptions) error {
		mo.noBuiltinVariables = nbv
		return nil
	}
}

// Turn off directory printing, even if it was turned on implicitly.  Equivalent
// to calling the flags --no-print-directory
func WithNoPrintDirectory(npd bool) MakeOption {
	return func(mo *MakeOptions) error {
		mo.noPrintDirectory = npd
		return nil
	}
}

// Don't echo recipes.  Equivalent to calling the flags -s|--silent|--quiet
func WithSilent(silent bool) MakeOption {
	return func(mo *MakeOptions) error {
		mo.silent = silent
		return nil
	}
}

// Synchronize the output.
func WithSyncOutput(sync bool) MakeOption {
	return func(mo *MakeOptions) error {
		mo.syncOutput = sync
		return nil
	}
}

// Touch targets instead of remaking them.  Equivalent to calling the flags
// -t|--touch
func WithTouch(touch bool) MakeOption {
	return func(mo *MakeOptions) error {
		mo.touch = touch
		return nil
	}
}

// Print tracing information.  Equivalent to calling the flag --trace
func WithTrace(trace bool) MakeOption {
	return func(mo *MakeOptions) error {
		mo.trace = trace
		return nil
	}
}

// Print the version number of make and exit.  Equivalent to calling the flags
// -v|--version
func WithVersion(version bool) MakeOption {
	return func(mo *MakeOptions) error {
		mo.version = version
		return nil
	}
}

// Print the current directory.  Equivalent to calling the flags
// -w|--print-directory
func WithPrintDirectory(printDirectory bool) MakeOption {
	return func(mo *MakeOptions) error {
		mo.printDirectory = printDirectory
		return nil
	}
}

// Consider files to be infinitely new.  Equivalent to calling the flags
// -W|--what-if|--new-file|--assume-new
func WithAssumeNew(file string) MakeOption {
	return func(mo *MakeOptions) error {
		if mo.newFiles == nil {
			mo.newFiles = make([]string, 0)
		}

		mo.newFiles = append(mo.newFiles, file)

		return nil
	}
}

// Warn when an undefined variable is referenced.  Equivalent to calling the
// flag --warn-undefined-variables
func WithWarnUndefinedVariables(wuv bool) MakeOption {
	return func(mo *MakeOptions) error {
		mo.warnUndefinedVariables = wuv
		return nil
	}
}

// The targets to make (omittion will invoke all targets).  Equivalent to
// calling the flags
func WithTarget(target ...string) MakeOption {
	return func(mo *MakeOptions) error {
		if len(target) == 0 {
			return nil
		}

		if mo.targets == nil {
			mo.targets = make([]string, 0)
		}

		mo.targets = append(mo.targets, target...)

		return nil
	}
}

// WithProgressFunc sets an optional progress function which is used as a
// callback during the ultimate invocation of make which can be calculated by
// invoking make's "just print" option
func WithProgressFunc(onProgress func(float64)) MakeOption {
	return func(mo *MakeOptions) error {
		mo.onProgress = onProgress
		return nil
	}
}

// WithExecOptions offers configuration options to the underlying process
// executor
func WithExecOptions(eopts ...exec.ExecOption) MakeOption {
	return func(mo *MakeOptions) error {
		if mo.eopts == nil {
			mo.eopts = make([]exec.ExecOption, 0)
		}

		mo.eopts = append(mo.eopts, eopts...)

		return nil
	}
}

// WithBinPath sets an alternative path to the GNU Make binary executable
func WithBinPath(path string) MakeOption {
	return func(mo *MakeOptions) error {
		mo.bin = path
		return nil
	}
}
