// SPDX-License-Identifier: Apache-2.0
// Copyright 2020 Docker Compose CLI authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package rainbowprint

import (
	"fmt"
	"strconv"
	"sync"

	"github.com/docker/compose/v2/pkg/api"
)

var names = []string{
	"grey",
	"red",
	"green",
	"yellow",
	"blue",
	"magenta",
	"cyan",
	"white",
}

const (
	// Never use ANSI codes
	Never = "never"

	// Always use ANSI codes
	Always = "always"

	// Auto detect terminal is a tty and can use ANSI codes
	Auto = "auto"
)

// SetANSIMode configure formatter for colored output on ANSI-compliant console
func SetANSIMode(streams api.Streams, ansi string) {
	if !useAnsi(streams, ansi) {
		nextColor = func() colorFunc {
			return monochrome
		}
	}
}

func useAnsi(streams api.Streams, ansi string) bool {
	switch ansi {
	case Always:
		return true
	case Auto:
		return streams.Out().IsTerminal()
	}
	return false
}

// colorFunc use ANSI codes to render colored text on console
type colorFunc func(s string) string

var monochrome = func(s string) string {
	return s
}

func ansiColor(code, s string) string {
	return fmt.Sprintf("%s%s%s", ansi(code), s, ansi("0"))
}

func ansi(code string) string {
	return fmt.Sprintf("\033[%sm", code)
}

func makeColorFunc(code string) colorFunc {
	return func(s string) string {
		return ansiColor(code, s)
	}
}

var (
	nextColor    = rainbowColor
	rainbow      []colorFunc
	currentIndex = 0
	mutex        sync.Mutex
)

func rainbowColor() colorFunc {
	mutex.Lock()
	defer mutex.Unlock()
	result := rainbow[currentIndex]
	currentIndex = (currentIndex + 1) % len(rainbow)
	return result
}

func init() {
	colors := map[string]colorFunc{}
	for i, name := range names {
		colors[name] = makeColorFunc(strconv.Itoa(30 + i))
		colors["intense_"+name] = makeColorFunc(strconv.Itoa(30+i) + ";1")
	}
	rainbow = []colorFunc{
		colors["cyan"],
		colors["yellow"],
		colors["green"],
		colors["magenta"],
		colors["blue"],
		colors["intense_cyan"],
		colors["intense_yellow"],
		colors["intense_green"],
		colors["intense_magenta"],
		colors["intense_blue"],
	}
}
