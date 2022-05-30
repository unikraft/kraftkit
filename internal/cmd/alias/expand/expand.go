// SPDX-License-Identifier: MIT
//
// Copyright (c) 2019 GitHub Inc.
//               2022 Unikraft UG.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package expand

import (
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strings"

	"github.com/google/shlex"
	"go.unikraft.io/kit/internal/config"
	"go.unikraft.io/kit/pkg/findsh"
)

// ExpandAlias processes argv to see if it should be rewritten according to a user's aliases. The
// second return value indicates whether the alias should be executed in a new shell process instead
// of running gh itself.
func ExpandAlias(cfg *config.Config, args []string, findShFunc func() (string, error)) (expanded []string, isShell bool, err error) {
	if len(args) < 2 {
		// the command is lacking a subcommand
		return
	}
	expanded = args[1:]

	expansion, ok := cfg.Aliases[args[0]][args[1]]
	if !ok {
		return
	}

	if strings.HasPrefix(expansion, "!") {
		isShell = true
		if findShFunc == nil {
			findShFunc = findSh
		}
		shPath, shErr := findShFunc()
		if shErr != nil {
			err = shErr
			return
		}

		expanded = []string{shPath, "-c", expansion[1:]}

		if len(args[2:]) > 0 {
			expanded = append(expanded, "--")
			expanded = append(expanded, args[2:]...)
		}

		return
	}

	extraArgs := []string{}
	for i, a := range args[2:] {
		if !strings.Contains(expansion, "$") {
			extraArgs = append(extraArgs, a)
		} else {
			expansion = strings.ReplaceAll(expansion, fmt.Sprintf("$%d", i+1), a)
		}
	}
	lingeringRE := regexp.MustCompile(`\$\d`)
	if lingeringRE.MatchString(expansion) {
		err = fmt.Errorf("not enough arguments for alias: %s", expansion)
		return
	}

	var newArgs []string
	newArgs, err = shlex.Split(expansion)
	if err != nil {
		return
	}

	expanded = append(newArgs, extraArgs...)
	return
}

func findSh() (string, error) {
	shPath, err := findsh.Find()
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			if runtime.GOOS == "windows" {
				return "", errors.New("unable to locate sh to execute the shell alias with. The sh.exe interpreter is typically distributed with Git for Windows")
			}
			return "", errors.New("unable to locate sh to execute shell alias with")
		}
		return "", err
	}
	return shPath, nil
}
