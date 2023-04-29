// SPDX-License-Identifier: MIT
// Copyright (c) 2019, 2019 GitHub Inc.
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the MIT License (the "License").
// You may not use this file expect in compliance with the License.
package cmdfactory

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func MinimumArgs(n int, msg string) cobra.PositionalArgs {
	if msg == "" {
		return cobra.MinimumNArgs(1)
	}

	return func(_ *cobra.Command, args []string) error {
		if len(args) < n {
			return FlagErrorf("%s", msg)
		}
		return nil
	}
}

func ExactArgs(n int, msg string) cobra.PositionalArgs {
	return func(_ *cobra.Command, args []string) error {
		if len(args) > n {
			return FlagErrorf("too many arguments")
		}

		if len(args) < n {
			return FlagErrorf("%s", msg)
		}

		return nil
	}
}

func NoArgsQuoteReminder(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return nil
	}

	errMsg := fmt.Sprintf("unknown argument %q", args[0])
	if len(args) > 1 {
		errMsg = fmt.Sprintf("unknown arguments %q", args)
	}

	hasValueFlag := false
	cmd.Flags().Visit(func(f *pflag.Flag) {
		if f.Value.Type() != "bool" {
			hasValueFlag = true
		}
	})

	if hasValueFlag {
		errMsg += "; please quote all values that have spaces"
	}

	return FlagErrorf("%s", errMsg)
}

func MaxDirArgs(n int) cobra.PositionalArgs {
	return func(_ *cobra.Command, args []string) error {
		if len(args) > n {
			return FlagErrorf("expected no more than %d paths received %d", n, len(args))

			// Treat no path as current working directory
		} else if len(args) == 0 {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}

			args = []string{cwd}
		}

		for _, path := range args {
			f, err := os.Stat(path)
			if err != nil || !f.IsDir() {
				return FlagErrorf("path is not a valid directory: %s", path)
			}
		}

		return nil
	}
}
