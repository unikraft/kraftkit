// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Acorn Labs, Inc; All rights reserved.
// Copyright 2022 Unikraft GmbH; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
package cmdfactory

import (
	"os"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func TestAttributeFlags_StructFields(t *testing.T) {
	attributeFlags := func(t *testing.T, obj any, args ...string) {
		t.Helper()

		rootCmd, subCmds := []string{"kraft"}, []string{"cmd1", "subcmd1"}
		cmd := makeCommand(append(rootCmd, subCmds...)...)

		allArgs := append(args, subCmds...)
		os.Args = append([]string{os.Args[1]}, allArgs...)

		// AttributeFlags also populates cmd's private flag fields.
		if err := AttributeFlags(cmd, obj, allArgs...); err != nil {
			t.Fatal("Failed to associate flags with struct fields:", err)
		}

		// Execute command to invoke cmd.RunE. This executes all middlewares injected
		// by bind(), which role is to assign parsed flag values to struct fields.
		if _, err := cmd.ExecuteC(); err != nil {
			t.Fatal("Failed to execute command:", err)
		}
	}

	type TestObj struct {
		String string            `long:"string" usage:"String arg"`
		Int    int               `long:"int" usage:"Integer arg"`
		Bool   bool              `long:"bool" usage:"Boolean arg"`
		Slice  []string          `long:"slice" usage:"Slice arg"`
		Map    map[string]string `long:"map" usage:"Map arg"`
		Nested struct {
			String string            `long:"n-string" usage:"Nested string arg"`
			Int    int               `long:"n-int" usage:"Nested integer arg"`
			Bool   bool              `long:"n-bool" usage:"Nested boolean arg"`
			Slice  []string          `long:"n-slice" usage:"Nested slice arg"`
			Map    map[string]string `long:"n-map" usage:"Nested map arg"`
		}
	}

	t.Run("String fields", func(t *testing.T) {
		obj := &TestObj{}
		attributeFlags(t, obj, "--string=val", "--n-string=n-val")
		if expect, got := "val", obj.String; expect != got {
			t.Errorf("Unexpected value for string struct field after flags attribution. Expected %q, got %q", expect, got)
		}
		if expect, got := "n-val", obj.Nested.String; expect != got {
			t.Errorf("Unexpected value for nested string struct field after flags attribution. Expected %q, got %q", expect, got)
		}
	})

	t.Run("Integer fields", func(t *testing.T) {
		obj := &TestObj{}
		attributeFlags(t, obj, "--int=1", "--n-int=2")
		if expect, got := 1, obj.Int; expect != got {
			t.Errorf("Unexpected value for int struct field after flags attribution. Expected %d, got %d", expect, got)
		}
		if expect, got := 2, obj.Nested.Int; expect != got {
			t.Errorf("Unexpected value for nested int struct field after flags attribution. Expected %d, got %d", expect, got)
		}
	})

	t.Run("Boolean fields", func(t *testing.T) {
		obj := &TestObj{}
		attributeFlags(t, obj, "--bool=true", "--n-bool=true")
		if expect, got := true, obj.Bool; expect != got {
			t.Errorf("Unexpected value for bool struct field after flags attribution. Expected %t, got %t", expect, got)
		}
		if expect, got := true, obj.Nested.Bool; expect != got {
			t.Errorf("Unexpected value for nested bool struct field after flags attribution. Expected %t, got %t", expect, got)
		}
	})

	t.Run("Slice fields", func(t *testing.T) {
		obj := &TestObj{}
		attributeFlags(t, obj, "--slice=val1", "--slice=val2", "--n-slice=val1,val2")
		if expect, got := []string{"val1", "val2"}, obj.Slice; !equalSlices(got, expect) {
			t.Errorf("Unexpected value for slice struct field after flags attribution. Expected %v, got %v", expect, got)
		}
		if expect, got := []string{"val1", "val2"}, obj.Nested.Slice; !equalSlices(got, expect) {
			t.Errorf("Unexpected value for nested slice struct field after flags attribution. Expected %v, got %v", expect, got)
		}
	})

	t.Run("Map fields", func(t *testing.T) {
		obj := &TestObj{}
		attributeFlags(t, obj, "--map=key=val", "--n-map=key=val")
		if expect, got := map[string]string{"key": "val"}, obj.Map; !equalMaps(got, expect) {
			t.Errorf("Unexpected value for map struct field after flags attribution. Expected %v, got %v", expect, got)
		}
		if expect, got := map[string]string{"key": "val"}, obj.Nested.Map; !equalMaps(got, expect) {
			t.Errorf("Unexpected value for nested map struct field after flags attribution. Expected %v, got %v", expect, got)
		}
	})
}

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

			if !equalSlices(args, tc.expect) {
				t.Errorf("Expected filtered args\n%q\ngot\n%q", tc.expect, args)
			}
		})
	}
}

// makeCommand produces a command with the given hierarchy of subcommands, and
// returns the deepest command.
func makeCommand(hierarchy ...string) *cobra.Command {
	var lastRoot *cobra.Command

	for _, cmdName := range hierarchy {
		newRoot := &cobra.Command{
			Use:  cmdName + " [-F file | -D dir]... [-f format] something",
			RunE: func(*cobra.Command, []string) error { return nil },
		}
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

func equalSlices(got, expect []string) bool {
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

func equalMaps(got, expect map[string]string) bool {
	if len(got) != len(expect) {
		return false
	}

	for k, gv := range got {
		if ev, ok := expect[k]; !ok || ev != gv {
			return false
		}
	}

	return true
}
