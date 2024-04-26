// SPDX-License-Identifier: MIT
//
// Copyright (c) 2019 GitHub Inc.
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

package iostreams

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/mgutz/ansi"
)

var (
	Magenta  = ansi.ColorFunc("magenta")
	Cyan     = ansi.ColorFunc("cyan")
	Red      = ansi.ColorFunc("red")
	Yellow   = ansi.ColorFunc("yellow")
	Blue     = ansi.ColorFunc("blue")
	Green    = ansi.ColorFunc("green")
	Gray     = ansi.ColorFunc("black+h")
	Bold     = ansi.ColorFunc("default+b")
	CyanBold = ansi.ColorFunc("cyan+b")

	Gray256 = func(t string) string {
		return fmt.Sprintf("\x1b[%d;5;%dm%s\x1b[m", 38, 242, t)
	}
)

func EnvColorDisabled() bool {
	return os.Getenv("NO_COLOR") != "" || os.Getenv("CLICOLOR") == "0"
}

func EnvColorForced() bool {
	return os.Getenv("CLICOLOR_FORCE") != "" && os.Getenv("CLICOLOR_FORCE") != "0"
}

func Is256ColorSupported() bool {
	return IsTrueColorSupported() ||
		strings.Contains(os.Getenv("TERM"), "256") ||
		strings.Contains(os.Getenv("COLORTERM"), "256")
}

func IsTrueColorSupported() bool {
	term := os.Getenv("TERM")
	colorterm := os.Getenv("COLORTERM")

	return strings.Contains(term, "24bit") ||
		strings.Contains(term, "truecolor") ||
		strings.Contains(colorterm, "24bit") ||
		strings.Contains(colorterm, "truecolor")
}

func NewColorScheme(enabled, is256enabled bool, trueColor bool) *ColorScheme {
	return &ColorScheme{
		enabled:      enabled,
		is256enabled: is256enabled,
		hasTrueColor: trueColor,
	}
}

type ColorScheme struct {
	enabled      bool
	is256enabled bool
	hasTrueColor bool
}

func (c *ColorScheme) Bold(t string) string {
	if !c.enabled {
		return t
	}
	return Bold(t)
}

func (c *ColorScheme) Boldf(t string, args ...interface{}) string {
	return c.Bold(fmt.Sprintf(t, args...))
}

func (c *ColorScheme) Red(t string) string {
	if !c.enabled {
		return t
	}
	return Red(t)
}

func (c *ColorScheme) Redf(t string, args ...interface{}) string {
	return c.Red(fmt.Sprintf(t, args...))
}

func (c *ColorScheme) Yellow(t string) string {
	if !c.enabled {
		return t
	}
	return Yellow(t)
}

func (c *ColorScheme) Yellowf(t string, args ...interface{}) string {
	return c.Yellow(fmt.Sprintf(t, args...))
}

func (c *ColorScheme) Green(t string) string {
	if !c.enabled {
		return t
	}
	return Green(t)
}

func (c *ColorScheme) Greenf(t string, args ...interface{}) string {
	return c.Green(fmt.Sprintf(t, args...))
}

func (c *ColorScheme) Gray(t string) string {
	if !c.enabled {
		return t
	}
	if c.is256enabled {
		return Gray256(t)
	}
	return Gray(t)
}

func (c *ColorScheme) Grayf(t string, args ...interface{}) string {
	return c.Gray(fmt.Sprintf(t, args...))
}

func (c *ColorScheme) Magenta(t string) string {
	if !c.enabled {
		return t
	}
	return Magenta(t)
}

func (c *ColorScheme) Magentaf(t string, args ...interface{}) string {
	return c.Magenta(fmt.Sprintf(t, args...))
}

func (c *ColorScheme) Cyan(t string) string {
	if !c.enabled {
		return t
	}
	return Cyan(t)
}

func (c *ColorScheme) Cyanf(t string, args ...interface{}) string {
	return c.Cyan(fmt.Sprintf(t, args...))
}

func (c *ColorScheme) CyanBold(t string) string {
	if !c.enabled {
		return t
	}
	return CyanBold(t)
}

func (c *ColorScheme) Blue(t string) string {
	if !c.enabled {
		return t
	}
	return Blue(t)
}

func (c *ColorScheme) Bluef(t string, args ...interface{}) string {
	return c.Blue(fmt.Sprintf(t, args...))
}

func (c *ColorScheme) SuccessIcon() string {
	return c.SuccessIconWithColor(c.Green)
}

func (c *ColorScheme) SuccessIconWithColor(colo func(string) string) string {
	return colo("✓")
}

func (c *ColorScheme) WarningIcon() string {
	return c.Yellow("!")
}

func (c *ColorScheme) FailureIcon() string {
	return c.FailureIconWithColor(c.Red)
}

func (c *ColorScheme) FailureIconWithColor(colo func(string) string) string {
	return colo("X")
}

func (c *ColorScheme) ColorFromString(s string) func(string) string {
	s = strings.ToLower(s)
	var fn func(string) string
	switch s {
	case "bold":
		fn = c.Bold
	case "red":
		fn = c.Red
	case "yellow":
		fn = c.Yellow
	case "green":
		fn = c.Green
	case "gray":
		fn = c.Gray
	case "magenta":
		fn = c.Magenta
	case "cyan":
		fn = c.Cyan
	case "blue":
		fn = c.Blue
	default:
		if c.enabled {
			fn = ansi.ColorFunc(s)
		} else {
			fn = func(s string) string {
				return s
			}
		}
	}

	return fn
}

// SprintFunc returns a new function that returns colorized strings for the
// given arguments with fmt.Sprint(). Useful to put into or mix into other
// string. Windows users should use this in conjunction with color.Output,
// example:
//
//	put := New(FgYellow).SprintFunc()
//	fmt.Fprintf(color.Output, "This is a %s", put("warning"))
func (c *ColorScheme) SprintFunc(s string) func(a ...interface{}) string {
	return func(a ...interface{}) string {
		return c.ColorFromString(s)(fmt.Sprint(a...))
	}
}

func (c *ColorScheme) HexToRGB(hex string, x string) string {
	if !c.enabled || !c.hasTrueColor {
		return x
	}

	r, _ := strconv.ParseInt(hex[0:2], 16, 64)
	g, _ := strconv.ParseInt(hex[2:4], 16, 64)
	b, _ := strconv.ParseInt(hex[4:6], 16, 64)
	return fmt.Sprintf("\033[38;2;%d;%d;%dm%s\033[0m", r, g, b, x)
}
