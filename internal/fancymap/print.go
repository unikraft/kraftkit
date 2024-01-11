// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

// Package fancymap provides a utility method which can be used to output a
// nice key-value list to the provided output writer.
package fancymap

import (
	"fmt"
	"io"
	"strings"

	"kraftkit.sh/tui"
)

// FancyMapEntry represents one item in the fancy list.
type FancyMapEntry struct {
	Key   string
	Value string
	Right string
}

// PrintFancyMap uses the provided writer `w` and outputs a pretty printed listed
// based on the list of `entries`.  A `title` and `success` state can be set
// which are prepended to the list.
func PrintFancyMap(w io.Writer, title string, success bool, entries ...FancyMapEntry) {
	keyPad, valPad, bracketPad := 0, 0, 0

	for _, entry := range entries {
		if newLen := len(entry.Key); newLen > keyPad {
			keyPad = newLen + 1
		}
		if newLen := len(entry.Value); newLen > valPad {
			valPad = newLen
		}
		if newLen := len(entry.Right); newLen > bracketPad {
			bracketPad = newLen
		}
	}

	var color func(...string) string
	if success {
		color = tui.TextGreen
	} else {
		color = tui.TextRed
	}

	fmt.Fprintf(w, "\n")
	fmt.Fprint(w, tui.TextLightGray("["))
	fmt.Fprint(w, color("●"))
	fmt.Fprint(w, tui.TextLightGray("]"))
	fmt.Fprintf(w, " ")
	fmt.Fprint(w, title)
	fmt.Fprintf(w, "\n ")
	fmt.Fprint(w, tui.TextLightGray("│"))
	fmt.Fprintf(w, "\n")

	for i, entry := range entries {
		fmt.Fprintf(w, " ")
		anchor := "├"
		if i == len(entries)-1 {
			anchor = "└"
		}
		fmt.Fprint(w, tui.TextLightGray(anchor))
		fmt.Fprint(w, tui.TextLightGray(strings.Repeat("─", keyPad-len(entry.Key))))
		fmt.Fprint(w, " ")
		fmt.Fprint(w, tui.TextLightGray(entry.Key))
		fmt.Fprint(w, ": ")
		fmt.Fprint(w, entry.Value)
		fmt.Fprint(w, " ")
		fmt.Fprint(w, strings.Repeat(" ", valPad-len(entry.Value)))
		if len(entry.Right) > 0 {
			fmt.Fprint(w, entry.Right)
			fmt.Fprint(w, strings.Repeat(" ", bracketPad-len(entry.Right)))
		} else {
			fmt.Fprint(w, strings.Repeat(" ", bracketPad))
		}
		fmt.Fprintf(w, "\n")
	}

	fmt.Fprintf(w, "\n")
}
