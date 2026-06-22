// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2025, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package list

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/iostreams"
)

type List struct{}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&List{}, cobra.Command{
		Short:   "List KraftKit configuration options",
		Use:     "list [FLAGS]",
		Aliases: []string{"ls"},
		Args:    cobra.NoArgs,
		Long: heredoc.Doc(`
			List all KraftKit configuration options and their current values.
		`),
		Example: heredoc.Doc(`
			# List all configuration options
			$ kraft system list
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "misc",
		},
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *List) Run(ctx context.Context, _ []string) error {
	cfg := config.G[config.KraftKit](ctx)

	pairs := flattenConfig(reflect.ValueOf(cfg).Elem(), "")

	out := iostreams.G(ctx).Out
	for _, pair := range pairs {
		fmt.Fprintln(out, pair)
	}

	return nil
}

func flattenConfig(v reflect.Value, prefix string) []string {
	var pairs []string

	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fieldVal := v.Field(i)

		// Using yaml tag as key name, consistent with kraft system set
		tag := field.Tag.Get("yaml")
		if tag == "" || tag == "-" {
			continue
		}
		tagName := strings.Split(tag, ",")[0]
		if tagName == "-" {
			continue
		}

		key := tagName
		if prefix != "" {
			key = prefix + "." + tagName
		}

		switch fieldVal.Kind() {
		case reflect.Struct:
			pairs = append(pairs, flattenConfig(fieldVal, key)...)

		case reflect.Map:
			for _, mapKey := range fieldVal.MapKeys() {
				mapVal := fieldVal.MapIndex(mapKey)
				if mapVal.Kind() == reflect.Map {
					for _, innerKey := range mapVal.MapKeys() {
						innerVal := mapVal.MapIndex(innerKey)
						pairs = append(pairs, fmt.Sprintf("%s.%s.%s=%v", key, mapKey, innerKey, innerVal))
					}
				} else {
					pairs = append(pairs, fmt.Sprintf("%s.%s=%v", key, mapKey, mapVal))
				}
			}

		default:
			pairs = append(pairs, fmt.Sprintf("%s=%v", key, fieldVal))
		}
	}

	return pairs
}
