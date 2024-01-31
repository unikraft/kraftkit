// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Acorn Labs, Inc; All rights reserved.
// Copyright 2022 Unikraft GmbH; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
package cmdfactory

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"kraftkit.sh/log"
)

var (
	caseRegexp    = regexp.MustCompile("([a-z])([A-Z])")
	flagOverrides = make(map[string][]*pflag.Flag)
)

func RegisterFlag(cmdline string, flag *pflag.Flag) {
	flagOverrides[cmdline] = append(flagOverrides[cmdline], flag)
}

type PersistentPreRunnable interface {
	PersistentPre(cmd *cobra.Command, args []string) error
}

type PreRunnable interface {
	Pre(cmd *cobra.Command, args []string) error
}

type Runnable interface {
	Run(ctx context.Context, args []string) error
}

type fieldInfo struct {
	FieldType  reflect.StructField
	FieldValue reflect.Value
}

func fields(obj any) []fieldInfo {
	var objValue reflect.Value
	ptrValue := reflect.ValueOf(obj)
	if ptrValue.Kind() == reflect.Ptr {
		objValue = ptrValue.Elem()
	} else {
		objValue = ptrValue
	}

	var result []fieldInfo

	for i := 0; i < objValue.NumField(); i++ {
		fieldType := objValue.Type().Field(i)
		if fieldType.Anonymous && fieldType.Type.Kind() == reflect.Struct {
			result = append(result, fields(objValue.Field(i).Addr().Interface())...)
		} else if !fieldType.Anonymous {
			result = append(result, fieldInfo{
				FieldValue: objValue.Field(i),
				FieldType:  objValue.Type().Field(i),
			})
		}
	}

	return result
}

func Name(obj any) string {
	ptrValue := reflect.ValueOf(obj)
	objValue := ptrValue.Elem()
	commandName := strings.Replace(objValue.Type().Name(), "Command", "", 1)
	commandName, _ = name(commandName, "", "")
	return commandName
}

func expandRegisteredFlags(cmd *cobra.Command) {
	// Add flag overrides which can be provided by plugins
	for arg, flags := range flagOverrides {
		args := strings.Fields(arg)
		subCmd, _, err := cmd.Traverse(args[1:])
		if err != nil {
			continue
		}

		if subCmd != nil && subCmd.Flags() != nil {
			for _, flag := range flags {
				if subCmd.Flags().Lookup(flag.Name) == nil {
					subCmd.Flags().AddFlag(flag)
				}
			}
		}
	}
}

// filter out registered flags from the given command's args.
func filterOutRegisteredFlags(cmd *cobra.Command, args []string) (filteredArgs []string) {
	for cmdline, flags := range flagOverrides {
		if !isSameCommand(cmd, cmdline) {
			continue
		}

		registeredFlagsNames := map[string]*pflag.Flag{}
		for _, flag := range flags {
			registeredFlagsNames[flag.Name] = flag
		}

		for len(args) > 0 {
			arg := args[0]
			args = args[1:]

			switch {
			// not a flag ("", <val>, -)
			case len(arg) == 0 || arg[0] != '-' || len(arg) == 1:
				filteredArgs = append(filteredArgs, arg)

			// long flag
			case arg[1] == '-' && len(arg) > 2:
				subs := strings.SplitN(arg, "=", 2)

				flagName := strings.TrimPrefix(subs[0], "--")
				if flag, ok := registeredFlagsNames[flagName]; ok {
					if flag.Value.Type() != "bool" && len(subs) == 1 {
						args = args[1:]
					}
					continue
				}

				filteredArgs = append(filteredArgs, arg)

			// short flag
			default:
				subs := strings.SplitN(arg, "=", 2)

				flagName := strings.TrimPrefix(subs[0], "-")
				if _, ok := registeredFlagsNames[flagName]; ok {
					if len(subs) == 1 {
						args = args[1:]
					}
					continue
				}

				filteredArgs = append(filteredArgs, arg)
			}
		}

		return filteredArgs
	}

	return args
}

// returns whether the given cmd is the same as described by the command line string.
// cmdline is expected to be "kraft cmd subcmd ..."
func isSameCommand(cmd *cobra.Command, cmdline string) bool {
	cmdFields := strings.Fields(cmdline)

	if len(cmdFields) == 1 {
		return cmd.Name() == cmdFields[0]
	}

	// checking only the name of the direct parent should be sufficient
	par := cmd.Parent()
	if par == nil {
		return false
	}
	return par.Name() == cmdFields[len(cmdFields)-2] && cmd.Name() == cmdFields[len(cmdFields)-1]
}

// Main executes the given command
func Main(ctx context.Context, cmd *cobra.Command) int {
	// Expand flag all dynamically registered flag overrides.
	expandRegisteredFlags(cmd)

	if err := cmd.ExecuteContext(ctx); err != nil {
		log.G(ctx).Error(err)
		return 1
	}

	return 0
}

// AttributeFlags associates a given struct with public attributes and a set of
// tags with the provided cobra command so as to enable dynamic population of
// CLI flags.
func AttributeFlags(c *cobra.Command, obj any, args ...string) error {
	var (
		envs      []func()
		arrays    = map[string]reflect.Value{}
		slices    = map[string]reflect.Value{}
		maps      = map[string]reflect.Value{}
		optString = map[string]reflect.Value{}
		optBool   = map[string]reflect.Value{}
		optInt    = map[string]reflect.Value{}
	)

	for _, info := range fields(obj) {
		fieldType := info.FieldType
		v := info.FieldValue

		if strings.ToUpper(fieldType.Name[0:1]) != fieldType.Name[0:1] {
			continue
		}

		// Any structure attribute which has the tag `noattribute:"true"` is skipped
		if fieldType.Tag.Get("noattribute") == "true" {
			continue
		}

		name, alias := name(fieldType.Name, fieldType.Tag.Get("long"), fieldType.Tag.Get("short"))
		usage := fieldType.Tag.Get("usage")
		envName := fieldType.Tag.Get("env")
		defValue := fieldType.Tag.Get("default")
		defInt, err := strconv.Atoi(defValue)
		if err != nil {
			defInt = 0
		}
		// https://cs.opensource.google/go/go/+/refs/tags/go1.20.3:src/fmt/fmt_test.go;l=1046-1050
		strValue := fmt.Sprint(v)

		// Set the value from the environmental value, if known, it takes precedent
		// over the provided value which would otherwise come from a configuration
		// file.
		if envName != "" {
			if envValue := os.Getenv(envName); envValue != "" {
				strValue = envValue
			}
		}

		if strValue == "" && defValue != "" {
			strValue = defValue
		}

		flags := c.PersistentFlags()
		if fieldType.Tag.Get("local") == "true" {
			flags = c.Flags()
		}

		switch v.Interface().(type) {
		case time.Duration:
			flags.DurationVarP((*time.Duration)(unsafe.Pointer(v.Addr().Pointer())), name, alias, time.Duration(defInt), usage)
			continue
		}

		switch fieldType.Type.Kind() {
		case reflect.Uint, reflect.Uint64:
			flags.UintVarP((*uint)(unsafe.Pointer(v.Addr().Pointer())), name, alias, uint(defInt), usage)
		case reflect.Int, reflect.Int64:
			flags.IntVarP((*int)(unsafe.Pointer(v.Addr().Pointer())), name, alias, defInt, usage)
		case reflect.String:
			flags.StringVarP((*string)(unsafe.Pointer(v.Addr().Pointer())), name, alias, defValue, usage)
			if err := flags.Set(name, strValue); err != nil {
				return err
			}
		case reflect.Bool:
			flags.BoolVarP((*bool)(unsafe.Pointer(v.Addr().Pointer())), name, alias, false, usage)
			if err := flags.Set(name, strValue); err != nil {
				return err
			}
		case reflect.Slice:
			switch fieldType.Tag.Get("split") {
			case "false":
				arrays[name] = v
				if ptr := (*[]string)(unsafe.Pointer(v.Addr().Pointer())); *ptr != nil {
					flags.StringArrayVarP(ptr, name, alias, *ptr, usage)
				} else {
					flags.StringArrayP(name, alias, nil, usage)
				}
			default:
				slices[name] = v
				if ptr := (*[]string)(unsafe.Pointer(v.Addr().Pointer())); *ptr != nil {
					flags.StringSliceVarP(ptr, name, alias, *ptr, usage)
				} else {
					flags.StringSliceP(name, alias, nil, usage)
				}
			}
		case reflect.Map:
			maps[name] = v
			if ptr := (*[]string)(unsafe.Pointer(v.Addr().Pointer())); *ptr != nil {
				flags.StringSliceVarP(ptr, name, alias, *ptr, usage)
			} else {
				flags.StringSliceP(name, alias, nil, usage)
			}
		case reflect.Pointer:
			switch fieldType.Type.Elem().Kind() {
			case reflect.Int, reflect.Int64:
				optInt[name] = v
				flags.IntP(name, alias, defInt, usage)
				if err := flags.Set(name, strValue); err != nil {
					return err
				}
			case reflect.String:
				optString[name] = v
				flags.StringP(name, alias, defValue, usage)
				if err := flags.Set(name, strValue); err != nil {
					return err
				}
			case reflect.Bool:
				optBool[name] = v
				flags.BoolP(name, alias, false, usage)
				if err := flags.Set(name, strValue); err != nil {
					return err
				}
			}
		case reflect.Struct:
			if !v.CanAddr() {
				continue
			}

			// Recursively set embedded anonymous structs
			if err := AttributeFlags(c, v.Addr().Interface()); err != nil {
				return err
			}
		default:
			// Unknown kind on field " + fieldType.Name + " on " + objValue.Type().Name()
			continue
		}

		hidden := fieldType.Tag.Get("hidden")
		if hidden == "true" {
			if err := flags.MarkHidden(name); err != nil {
				return err
			}
		}
	}

	c.PersistentPreRunE = bind(c.PersistentPreRunE, arrays, slices, maps, optInt, optBool, optString, envs)
	c.PreRunE = bind(c.PreRunE, arrays, slices, maps, optInt, optBool, optString, envs)
	c.RunE = bind(c.RunE, arrays, slices, maps, optInt, optBool, optString, envs)

	return nil
}

// New populates a cobra.Command object by extracting args from struct tags of the
// Runnable obj passed.  Also the Run method is assigned to the RunE of the command.
// name = Override the struct field with
func New(obj Runnable, cmd cobra.Command) (*cobra.Command, error) {
	c := cmd
	if c.Use == "" {
		c.Use = fmt.Sprintf("%s [SUBCOMMAND] [FLAGS]", Name(obj))
	}

	if p, ok := obj.(PersistentPreRunnable); ok {
		c.PersistentPreRunE = p.PersistentPre
	}

	if p, ok := obj.(PreRunnable); ok {
		c.PreRunE = p.Pre
	}

	c.SilenceErrors = true
	c.SilenceUsage = true
	c.DisableFlagsInUseLine = true
	c.InitDefaultHelpFlag()
	c.InitDefaultCompletionCmd()

	if obj != nil {
		c.RunE = func(cmd *cobra.Command, args []string) error {
			return obj.Run(cmd.Context(), args)
		}

		// Parse the attributes of this object into addressable flags for this command
		if err := AttributeFlags(&c, obj); err != nil {
			return nil, err
		}
	}

	// Set help and usage methods
	c.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		rootHelpFunc(cmd, args)
	})
	c.SetUsageFunc(rootUsageFunc)
	c.SetFlagErrorFunc(rootFlagErrorFunc)

	return &c, nil
}

func assignOptBool(app *cobra.Command, maps map[string]reflect.Value) error {
	for k, v := range maps {
		k = contextKey(k)
		if !app.Flags().Lookup(k).Changed {
			continue
		}
		i, err := app.Flags().GetBool(k)
		if err != nil {
			return err
		}
		v.Set(reflect.ValueOf(&i))
	}
	return nil
}

func assignOptString(app *cobra.Command, maps map[string]reflect.Value) error {
	for k, v := range maps {
		k = contextKey(k)
		if !app.Flags().Lookup(k).Changed {
			continue
		}
		i, err := app.Flags().GetString(k)
		if err != nil {
			return err
		}
		v.Set(reflect.ValueOf(&i))
	}
	return nil
}

func assignOptInt(app *cobra.Command, maps map[string]reflect.Value) error {
	for k, v := range maps {
		k = contextKey(k)
		if !app.Flags().Lookup(k).Changed {
			continue
		}
		i, err := app.Flags().GetInt(k)
		if err != nil {
			return err
		}
		v.Set(reflect.ValueOf(&i))
	}
	return nil
}

func assignMaps(app *cobra.Command, maps map[string]reflect.Value) error {
	for k, v := range maps {
		k = contextKey(k)
		s, err := app.Flags().GetStringSlice(k)
		if err != nil {
			continue
		}
		if s != nil {
			values := map[string]string{}
			for _, part := range s {
				parts := strings.SplitN(part, "=", 2)
				if len(parts) == 1 {
					values[parts[0]] = ""
				} else {
					values[parts[0]] = parts[1]
				}
			}
			v.Set(reflect.ValueOf(values))
		}
	}
	return nil
}

func assignSlices(app *cobra.Command, slices map[string]reflect.Value) error {
	for k, v := range slices {
		k = contextKey(k)
		s, err := app.Flags().GetStringSlice(k)
		if err != nil {
			continue
		}
		a := app.Flags().Lookup(k)
		if a.Changed && len(s) == 0 {
			s = []string{""}
		}
		if s != nil {
			v.Set(reflect.ValueOf(s[:]))
		}
	}
	return nil
}

func assignArrays(app *cobra.Command, arrays map[string]reflect.Value) error {
	for k, v := range arrays {
		k = contextKey(k)
		s, err := app.Flags().GetStringArray(k)
		if err != nil {
			continue
		}
		a := app.Flags().Lookup(k)
		if a.Changed && len(s) == 0 {
			s = []string{""}
		}
		if s != nil {
			v.Set(reflect.ValueOf(s[:]))
		}
	}
	return nil
}

func contextKey(name string) string {
	parts := strings.Split(name, ",")
	return parts[len(parts)-1]
}

func name(name, setName, short string) (string, string) {
	if setName != "" {
		return setName, short
	}
	parts := strings.Split(name, "_")
	i := len(parts) - 1
	name = caseRegexp.ReplaceAllString(parts[i], "$1-$2")
	name = strings.ToLower(name)
	result := append([]string{name}, parts[0:i]...)
	for i := 0; i < len(result); i++ {
		result[i] = strings.ToLower(result[i])
	}
	if short == "" && len(result) > 1 {
		short = result[1]
	}
	return result[0], short
}

func bind(next func(*cobra.Command, []string) error,
	arrays map[string]reflect.Value,
	slices map[string]reflect.Value,
	maps map[string]reflect.Value,
	optInt map[string]reflect.Value,
	optBool map[string]reflect.Value,
	optString map[string]reflect.Value,
	envs []func(),
) func(*cobra.Command, []string) error {
	if next == nil {
		return nil
	}
	return func(cmd *cobra.Command, args []string) error {
		for _, envCallback := range envs {
			envCallback()
		}
		if err := assignArrays(cmd, arrays); err != nil {
			return err
		}
		if err := assignSlices(cmd, slices); err != nil {
			return err
		}
		if err := assignMaps(cmd, maps); err != nil {
			return err
		}
		if err := assignOptInt(cmd, optInt); err != nil {
			return err
		}
		if err := assignOptBool(cmd, optBool); err != nil {
			return err
		}
		if err := assignOptString(cmd, optString); err != nil {
			return err
		}

		if next != nil {
			return next(cmd, args)
		}

		return nil
	}
}
