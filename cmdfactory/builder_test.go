// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Acorn Labs, Inc; All rights reserved.
// Copyright 2022 Unikraft GmbH; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
package cmdfactory

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func TestFilterOutRegisteredFlags(t *testing.T) {
	flagOverridesOrig := copyFlagOverrides()
	t.Cleanup(func() { flagOverrides = flagOverridesOrig })

	flagOverrides = map[string][]*pflag.Flag{
		"kraft cmd1":         makeLongFlags("cmd1-override1", "cmd1-override2"),
		"kraft cmd2":         makeLongFlags("cmd2-override1", "cmd2-override2"),
		"kraft cmd1 subcmd1": makeLongFlags("subcmd1-override1", "subcmd1-override2"),
		"kraft cmd1 subcmd2": makeLongFlags("subcmd2-override1", "subcmd2-override2"),
	}

	cmd := makeCommand("kraft", "cmd1", "subcmd1")

	testCases := []struct {
		desc   string
		args   []string
		expect []string
	}{
		{
			desc:   "args do not contain registered flags",
			args:   []string{"-v", "-w", "wval", "-x=xval", "--y", "yval", "--z=zval"},
			expect: []string{"-v", "-w", "wval", "-x=xval", "--y", "yval", "--z=zval"},
		},
		{
			desc:   "args contain registered flags in long format",
			args:   []string{"--subcmd1-override1", "val1", "--subcmd1-override2=val2", "--y", "yval", "--z=zval"},
			expect: []string{"--y", "yval", "--z=zval"},
		},
		{
			// unikraft/kraftkit#552
			desc:   "args contain flags with empty values",
			args:   []string{"--subcmd1-override1", "", "--subcmd1-override2=", "-v", "-w", "", "-x=", "--y", "", "--z="},
			expect: []string{"-v", "-w", "", "-x=", "--y", "", "--z="},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			args := filterOutRegisteredFlags(cmd, tc.args)

			if !equalArgs(args, tc.expect) {
				t.Errorf("Expected filtered args\n%q\ngot\n%q", tc.expect, args)
			}
		})
	}
}

func equalArgs(got, expect []string) bool {
	if len(got) != len(expect) {
		return false
	}

	for i := 0; i < len(got); i++ {
		if got[i] != expect[i] {
			return false
		}
	}

	return true
}

// makeCommand produces a command with the given hierarchy of subcommands, and
// returns the deepest command.
func makeCommand(hierarchy ...string) *cobra.Command {
	var lastRoot *cobra.Command

	for _, cmdName := range hierarchy {
		newRoot := &cobra.Command{Use: cmdName + " [-F file | -D dir]... [-f format] something"}
		if lastRoot != nil {
			lastRoot.AddCommand(newRoot)
		}
		lastRoot = newRoot
	}

	return lastRoot
}

// makeLongFlags returns string flags with the given names.
func makeLongFlags(names ...string) []*pflag.Flag {
	flags := make([]*pflag.Flag, 0, len(names))

	for _, n := range names {
		var strVal string
		flags = append(flags, StringVar(&strVal, n, "default", "a test flag"))
	}

	return flags
}

// copyFlagOverrides returns a copy of the global flagOverrides slice.
func copyFlagOverrides() map[string][]*pflag.Flag {
	flagOverridesCpy := make(map[string][]*pflag.Flag, len(flagOverrides))
	for cmdline, flags := range flagOverrides {
		flagOverridesCpy[cmdline] = flags
	}
	return flagOverridesCpy
}
