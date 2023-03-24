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
	Run(cmd *cobra.Command, args []string) error
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

		for _, flag := range flags {
			subCmd.Flags().AddFlag(flag)
		}
	}
}

// Main executes the given command
func Main(ctx context.Context, cmd *cobra.Command) {
	// Expand flag all dynamically registered flag overrides.
	expandRegisteredFlags(cmd)

	if err := cmd.ExecuteContext(ctx); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// AttributeFlags associates a given struct with public attributes and a set of
// tags with the provided cobra command so as to enable dynamic population of
// CLI flags.
func AttributeFlags(c *cobra.Command, obj any, args ...string) {
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
		strValue := v.String()

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
		case reflect.Int:
			flags.IntVarP((*int)(unsafe.Pointer(v.Addr().Pointer())), name, alias, defInt, usage)
			flags.Set(name, strValue)
		case reflect.Int64:
			flags.IntVarP((*int)(unsafe.Pointer(v.Addr().Pointer())), name, alias, defInt, usage)
			flags.Set(name, strValue)
		case reflect.String:
			flags.StringVarP((*string)(unsafe.Pointer(v.Addr().Pointer())), name, alias, defValue, usage)
			flags.Set(name, strValue)
		case reflect.Slice:
			switch fieldType.Tag.Get("split") {
			case "false":
				arrays[name] = v
				flags.StringArrayP(name, alias, nil, usage)
			default:
				slices[name] = v
				flags.StringSliceP(name, alias, nil, usage)
			}
		case reflect.Map:
			maps[name] = v
			flags.StringSliceP(name, alias, nil, usage)
		case reflect.Bool:
			flags.BoolVarP((*bool)(unsafe.Pointer(v.Addr().Pointer())), name, alias, false, usage)
			flags.Set(name, strValue)
		case reflect.Pointer:
			switch fieldType.Type.Elem().Kind() {
			case reflect.Int:
				optInt[name] = v
				flags.IntP(name, alias, defInt, usage)
				flags.Set(name, strValue)
			case reflect.String:
				optString[name] = v
				flags.StringP(name, alias, defValue, usage)
				flags.Set(name, strValue)
			case reflect.Bool:
				optBool[name] = v
				flags.BoolP(name, alias, false, usage)
				flags.Set(name, strValue)
			}
		case reflect.Struct:
			if !v.CanAddr() {
				continue
			}

			// Recursively set embedded anonymous structs
			AttributeFlags(c, v.Addr().Interface())
		default:
			// Unknown kind on field " + fieldType.Name + " on " + objValue.Type().Name()
			continue
		}
	}

	// If any arguments are passed, parse them immediately
	if len(args) > 0 {
		c.ParseFlags(args)
	}

	c.PersistentPreRunE = bind(c.PersistentPreRunE, arrays, slices, maps, optInt, optBool, optString, envs)
	c.PreRunE = bind(c.PreRunE, arrays, slices, maps, optInt, optBool, optString, envs)
	c.RunE = bind(c.RunE, arrays, slices, maps, optInt, optBool, optString, envs)
}

// New populates a cobra.Command object by extracting args from struct tags of the
// Runnable obj passed.  Also the Run method is assigned to the RunE of the command.
// name = Override the struct field with
func New(obj Runnable, cmd cobra.Command) *cobra.Command {
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
	c.RunE = obj.Run

	// Parse the attributes of this object into addressable flags for this command
	AttributeFlags(&c, obj)

	// Set help and usage methods
	c.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		rootHelpFunc(cmd, args)
	})
	c.SetUsageFunc(rootUsageFunc)
	c.SetFlagErrorFunc(rootFlagErrorFunc)

	return &c
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
			return err
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
			return err
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
			return err
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
