// SPDX-License-Identifier: MIT
// Copyright (c) 2017, Denis Parchenko.
// Copyright (c) 2022, Unikraft GmbH. All rights reserved.
package log

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/sirupsen/logrus"
	"golang.org/x/term"
)

const defaultTimestampFormat = time.RFC3339

var (
	baseTimestamp      time.Time    = time.Now()
	defaultColorScheme *ColorScheme = &ColorScheme{
		InfoLevel: lipgloss.NewStyle().Background(lipgloss.Color("8")).Foreground(lipgloss.AdaptiveColor{
			Light: "15",
			Dark:  "0",
		}).Render,
		WarnLevel: lipgloss.NewStyle().Background(lipgloss.Color("11")).Foreground(lipgloss.AdaptiveColor{
			Light: "15",
			Dark:  "0",
		}).Render,
		ErrorLevel: lipgloss.NewStyle().Background(lipgloss.Color("9")).Foreground(lipgloss.AdaptiveColor{
			Light: "15",
			Dark:  "0",
		}).Render,
		FatalLevel: lipgloss.NewStyle().Background(lipgloss.Color("9")).Foreground(lipgloss.AdaptiveColor{
			Light: "15",
			Dark:  "0",
		}).Render,
		PanicLevel: lipgloss.NewStyle().Background(lipgloss.Color("9")).Foreground(lipgloss.AdaptiveColor{
			Light: "15",
			Dark:  "0",
		}).Render,
		DebugLevel: lipgloss.NewStyle().Background(lipgloss.Color("12")).Foreground(lipgloss.AdaptiveColor{
			Light: "15",
			Dark:  "0",
		}).Render,
		TraceLevel: lipgloss.NewStyle().Background(lipgloss.Color("0")).Foreground(lipgloss.Color("15")).Render,
		Prefix: lipgloss.NewStyle().Background(lipgloss.Color("8")).Foreground(lipgloss.AdaptiveColor{
			Light: "15",
			Dark:  "0",
		}).Render,
		Timestamp: lipgloss.NewStyle().Render,
	}
	noColorsColorScheme *ColorScheme = &ColorScheme{
		InfoLevel:  lipgloss.NewStyle().Render,
		WarnLevel:  lipgloss.NewStyle().Render,
		ErrorLevel: lipgloss.NewStyle().Render,
		FatalLevel: lipgloss.NewStyle().Render,
		PanicLevel: lipgloss.NewStyle().Render,
		DebugLevel: lipgloss.NewStyle().Render,
		TraceLevel: lipgloss.NewStyle().Render,
		Prefix:     lipgloss.NewStyle().Render,
		Timestamp:  lipgloss.NewStyle().Render,
	}
)

func miniTS() int {
	return int(time.Since(baseTimestamp) / time.Second)
}

type renderFunc func(...string) string

type ColorScheme struct {
	InfoLevel  renderFunc
	WarnLevel  renderFunc
	ErrorLevel renderFunc
	FatalLevel renderFunc
	PanicLevel renderFunc
	DebugLevel renderFunc
	TraceLevel renderFunc
	Prefix     renderFunc
	Timestamp  renderFunc
}

type TextFormatter struct {
	// Set to true to bypass checking for a TTY before outputting colors.
	ForceColors bool

	// Force disabling colors. For a TTY colors are enabled by default.
	DisableColors bool

	// Force formatted layout, even for non-TTY output.
	ForceFormatting bool

	// Disable timestamp logging. useful when output is redirected to logging
	// system that already adds timestamps.
	DisableTimestamp bool

	// Enable logging the full timestamp when a TTY is attached instead of just
	// the time passed since beginning of execution.
	FullTimestamp bool

	// Timestamp format to use for display when a full timestamp is printed.
	TimestampFormat string

	// The fields are sorted by default for a consistent output. For applications
	// that log extremely frequently and don't use the JSON formatter this may not
	// be desired.
	DisableSorting bool

	// Wrap empty fields in quotes if true.
	QuoteEmptyFields bool

	// Can be set to the override the default quoting character "
	// with something else. For example: ', or `.
	QuoteCharacter string

	// Pad msg field with spaces on the right for display.
	// The value for this parameter will be the size of padding.
	// Its default value is zero, which means no padding will be applied for msg.
	SpacePadding int

	// Color scheme to use.
	colorScheme *ColorScheme

	// Whether the logger's out is to a terminal.
	isTerminal bool

	sync.Once
}

func (f *TextFormatter) init(entry *logrus.Entry) {
	if len(f.QuoteCharacter) == 0 {
		f.QuoteCharacter = "\""
	}
	if entry.Logger != nil {
		f.isTerminal = f.checkIfTerminal(entry.Logger.Out)
	}
}

func (f *TextFormatter) checkIfTerminal(w io.Writer) bool {
	switch v := w.(type) {
	case *os.File:
		return term.IsTerminal(int(v.Fd()))
	default:
		return false
	}
}

func (f *TextFormatter) SetColorScheme(colorScheme *ColorScheme) {
	f.colorScheme = colorScheme
}

func (f *TextFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	var b *bytes.Buffer
	var keys []string = make([]string, 0, len(entry.Data))
	for k := range entry.Data {
		keys = append(keys, k)
	}
	lastKeyIdx := len(keys) - 1

	if !f.DisableSorting {
		sort.Strings(keys)
	}
	if entry.Buffer != nil {
		b = entry.Buffer
	} else {
		b = &bytes.Buffer{}
	}

	prefixFieldClashes(entry.Data)

	f.Do(func() { f.init(entry) })

	isFormatted := f.ForceFormatting || f.isTerminal

	timestampFormat := f.TimestampFormat
	if timestampFormat == "" {
		timestampFormat = defaultTimestampFormat
	}
	if isFormatted {
		isColored := (f.ForceColors || f.isTerminal) && !f.DisableColors
		var colorScheme *ColorScheme
		if isColored {
			if f.colorScheme == nil {
				colorScheme = defaultColorScheme
			} else {
				colorScheme = f.colorScheme
			}
		} else {
			colorScheme = noColorsColorScheme
		}
		f.printColored(b, entry, keys, timestampFormat, colorScheme)
	} else {
		if !f.DisableTimestamp {
			f.appendKeyValue(b, "time", entry.Time.Format(timestampFormat), true)
		}
		f.appendKeyValue(b, "level", entry.Level.String(), true)
		if entry.Message != "" {
			f.appendKeyValue(b, "msg", entry.Message, lastKeyIdx >= 0)
		}
		for i, key := range keys {
			f.appendKeyValue(b, key, entry.Data[key], lastKeyIdx != i)
		}
	}

	b.WriteByte('\n')
	return b.Bytes(), nil
}

func (f *TextFormatter) printColored(b *bytes.Buffer, entry *logrus.Entry, keys []string, timestampFormat string, colorScheme *ColorScheme) {
	var levelColor renderFunc
	var levelText string
	switch entry.Level {
	case logrus.InfoLevel:
		levelText = "i"
		levelColor = colorScheme.InfoLevel
	case logrus.WarnLevel:
		levelText = "W"
		levelColor = colorScheme.WarnLevel
	case logrus.ErrorLevel:
		levelText = "E"
		levelColor = colorScheme.ErrorLevel
	case logrus.FatalLevel:
		levelText = "!"
		levelColor = colorScheme.FatalLevel
	case logrus.PanicLevel:
		levelText = "X"
		levelColor = colorScheme.PanicLevel
	case logrus.TraceLevel:
		levelText = "T"
		levelColor = colorScheme.TraceLevel
	default:
		levelText = "D"
		levelColor = colorScheme.DebugLevel
	}

	level := levelColor(fmt.Sprintf(" %1s ", levelText))
	prefix := ""
	message := entry.Message

	if prefixValue, ok := entry.Data["prefix"]; ok {
		prefix = colorScheme.Prefix(" " + prefixValue.(string) + ":")
	} else {
		prefixValue, trimmedMsg := extractPrefix(entry.Message)
		if len(prefixValue) > 0 {
			prefix = colorScheme.Prefix(" " + prefixValue + ":")
			message = trimmedMsg
		}
	}

	messageFormat := "%s"
	if f.SpacePadding != 0 {
		messageFormat = fmt.Sprintf("%%-%ds", f.SpacePadding)
	}

	if f.DisableTimestamp {
		fmt.Fprintf(b, "%s%s "+messageFormat, level, prefix, message)
	} else {
		var timestamp string
		if !f.FullTimestamp {
			timestamp = fmt.Sprintf("[%04d]", miniTS())
		} else {
			timestamp = entry.Time.Format(timestampFormat)
		}
		fmt.Fprintf(b, "%s %s%s "+messageFormat, level, colorScheme.Timestamp(timestamp), prefix, message)
	}
	for _, k := range keys {
		if k != "prefix" {
			v := entry.Data[k]
			fmt.Fprintf(b, " %s=%+v", levelColor(k), v)
		}
	}
}

func (f *TextFormatter) needsQuoting(text string) bool {
	if f.QuoteEmptyFields && len(text) == 0 {
		return true
	}
	for _, ch := range text {
		if !((ch >= 'a' && ch <= 'z') ||
			(ch >= 'A' && ch <= 'Z') ||
			(ch >= '0' && ch <= '9') ||
			ch == '-' || ch == '.') {
			return true
		}
	}
	return false
}

func extractPrefix(msg string) (string, string) {
	prefix := ""
	regex := regexp.MustCompile(`^\\[(.*?)\\]`)
	if regex.MatchString(msg) {
		match := regex.FindString(msg)
		prefix, msg = match[1:len(match)-1], strings.TrimSpace(msg[len(match):])
	}
	return prefix, msg
}

func (f *TextFormatter) appendKeyValue(b *bytes.Buffer, key string, value interface{}, appendSpace bool) {
	b.WriteString(key)
	b.WriteByte('=')
	f.appendValue(b, value)

	if appendSpace {
		b.WriteByte(' ')
	}
}

func (f *TextFormatter) appendValue(b *bytes.Buffer, value interface{}) {
	switch value := value.(type) {
	case string:
		if !f.needsQuoting(value) {
			b.WriteString(value)
		} else {
			fmt.Fprintf(b, "%s%v%s", f.QuoteCharacter, value, f.QuoteCharacter)
		}
	case error:
		errmsg := value.Error()
		if !f.needsQuoting(errmsg) {
			b.WriteString(errmsg)
		} else {
			fmt.Fprintf(b, "%s%v%s", f.QuoteCharacter, errmsg, f.QuoteCharacter)
		}
	default:
		fmt.Fprint(b, value)
	}
}

// This is to not silently overwrite `time`, `msg` and `level` fields when
// dumping it. If this code wasn't there doing:
//
//	logrus.WithField("level", 1).Info("hello")
//
// would just silently drop the user provided level. Instead with this code
// it'll be logged as:
//
//	{"level": "info", "fields.level": 1, "msg": "hello", "time": "..."}
func prefixFieldClashes(data logrus.Fields) {
	if t, ok := data["time"]; ok {
		data["fields.time"] = t
	}

	if m, ok := data["msg"]; ok {
		data["fields.msg"] = m
	}

	if l, ok := data["level"]; ok {
		data["fields.level"] = l
	}
}
