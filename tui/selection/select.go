// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package selection

import (
	"fmt"
	"sort"

	"github.com/charmbracelet/lipgloss"
	"github.com/erikgeiser/promptkit/selection"
)

var queryMark = lipgloss.NewStyle().
	Background(lipgloss.Color("12")).
	Foreground(lipgloss.Color("15")).
	Render

// Select is a utility method used in a CLI context to prompt the user
// given a slice of options based on the generic type.
func Select[T fmt.Stringer](question string, options ...T) (*T, error) {
	if len(options) == 1 {
		return &options[0], nil
	}

	strings := make([]string, 0, len(options))
	for _, option := range options {
		strings = append(strings, option.String())
	}

	sort.Strings(strings)

	sp := selection.New(queryMark("[?]")+" "+question, strings)
	sp.Filter = nil

	result, err := sp.RunPrompt()
	if err != nil {
		return nil, err
	}

	for _, t := range options {
		if t.String() == result {
			return &t, nil
		}
	}

	return nil, fmt.Errorf("could not perform selection")
}
