// SPDX-License-Identifier: MIT
//
// Copyright (c) 2019 GitHub Inc.
//               2022 Unikraft GmbH.
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

package utils

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"kraftkit.sh/internal/set"
)

func Pluralize(num int, thing string) string {
	if num == 1 {
		return fmt.Sprintf("%d %s", num, thing)
	}
	return fmt.Sprintf("%d %ss", num, thing)
}

func fmtDuration(amount int, unit string) string {
	return fmt.Sprintf("about %s ago", Pluralize(amount, unit))
}

func FuzzyAgo(ago time.Duration) string {
	if ago < time.Minute {
		return "less than a minute ago"
	}
	if ago < time.Hour {
		return fmtDuration(int(ago.Minutes()), "minute")
	}
	if ago < 24*time.Hour {
		return fmtDuration(int(ago.Hours()), "hour")
	}
	if ago < 30*24*time.Hour {
		return fmtDuration(int(ago.Hours())/24, "day")
	}
	if ago < 365*24*time.Hour {
		return fmtDuration(int(ago.Hours())/24/30, "month")
	}

	return fmtDuration(int(ago.Hours()/24/365), "year")
}

func FuzzyAgoAbbr(now time.Time, createdAt time.Time) string {
	ago := now.Sub(createdAt)

	if ago < time.Hour {
		return fmt.Sprintf("%d%s", int(ago.Minutes()), "m")
	}
	if ago < 24*time.Hour {
		return fmt.Sprintf("%d%s", int(ago.Hours()), "h")
	}
	if ago < 30*24*time.Hour {
		return fmt.Sprintf("%d%s", int(ago.Hours())/24, "d")
	}

	return createdAt.Format("Jan _2, 2006")
}

func Humanize(s string) string {
	// Replaces - and _ with spaces.
	replace := "_-"
	h := func(r rune) rune {
		if strings.ContainsRune(replace, r) {
			return ' '
		}
		return r
	}

	return strings.Map(h, s)
}

func IsURL(s string) bool {
	return strings.HasPrefix(s, "http:/") || strings.HasPrefix(s, "https:/")
}

func DisplayURL(urlStr string) string {
	u, err := url.Parse(urlStr)
	if err != nil {
		return urlStr
	}
	return u.Hostname() + u.Path
}

// Maximum length of a URL: 8192 bytes
func ValidURL(urlStr string) bool {
	return len(urlStr) < 8192
}

// ListJoinStr joins a slice of strings with a specified delimeter
func ListJoinStr(items []string, delim string) string {
	return strings.Trim(
		strings.Join(strings.Fields(fmt.Sprint(items)), delim), "[]",
	)
}

func Contains(haystack []string, needle string) bool {
	return set.NewStringSet(haystack...).Contains(needle)
}

// HumanizeDuration returns a relative time string based on an input duration
func HumanizeDuration(dur time.Duration) string {
	ms := dur.Nanoseconds() / 1000000
	sec := ms / 1000
	min := sec / 60
	hr := min / 60

	// Get only the excess amt of each component
	ms %= 1000
	sec %= 60
	hr %= 60

	// Express ms to 1 significant digit
	ms /= 100

	if hr >= 1 {
		return fmt.Sprintf("%dh %2dm %2ds", hr, min, sec)
	} else if min >= 10 {
		return fmt.Sprintf("%2dm %2ds", min, sec)
	} else if min >= 1 && sec < 10 {
		return fmt.Sprintf("%dm %ds", min, sec)
	} else if min >= 1 {
		return fmt.Sprintf("%dm %2ds", min, sec)
	}

	return fmt.Sprintf("%d.%ds", sec, ms)
}

// RelativePath resolve a relative to the base path.
func RelativePath(base, path string) string {
	if path[0] == '~' {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, path[1:])
	}

	if filepath.IsAbs(path) {
		return path
	}

	return filepath.Join(base, path)
}
