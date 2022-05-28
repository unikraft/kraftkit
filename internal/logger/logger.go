// SPDX-License-Identifier: BSD-3-Clause
//
// Authors: Alexander Jung <alex@unikraft.io>
//
// Copyright (c) 2022, Unikraft UG.  All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions
// are met:
//
// 1. Redistributions of source code must retain the above copyright
//    notice, this list of conditions and the following disclaimer.
// 2. Redistributions in binary form must reproduce the above copyright
//    notice, this list of conditions and the following disclaimer in the
//    documentation and/or other materials provided with the distribution.
// 3. Neither the name of the copyright holder nor the names of its
//    contributors may be used to endorse or promote products derived from
//    this software without specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
// AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
// IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
// ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE
// LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR
// CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF
// SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS
// INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN
// CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE)
// ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE
// POSSIBILITY OF SUCH DAMAGE.

package logger

import (
	"fmt"
	"os"
	"time"

	"go.unikraft.io/kit/pkg/iostreams"
)

var exit = os.Exit

// logFunc represents a log function
type logFunc func(a ...interface{}) string

// Logger maintains a set of logging functions
// and has a log level that can be modified dynamically
type Logger struct {
	io          *iostreams.IOStreams
	level       LogLevel
	trace       logFunc
	debug       logFunc
	info        logFunc
	warn        logFunc
	err         logFunc
	fatal       logFunc
	ExitOnFatal bool
}

func (l *Logger) now() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

func (l *Logger) log(level string, format string, a ...interface{}) {
	magenta := l.io.ColorScheme().SprintFunc("magenta")

	fmt.Fprintf(l.io.Out, "%s %s %s %s\n",
		magenta(l.now()),
		level,
		magenta(":"),
		fmt.Sprintf(format, a...),
	)
}

// SetLevel updates the logging level for future logs
func (l *Logger) SetLevel(level LogLevel) {
	l.level = level
}

func (l *Logger) Trace(format string, a ...interface{}) {
	if l.level == TRACE {
		l.log(l.trace("TRACE"), format, a...)
	}
}

func (l *Logger) Debug(format string, a ...interface{}) {
	if l.level <= DEBUG {
		l.log(l.debug("DEBUG"), format, a...)
	}
}

func (l *Logger) Info(format string, a ...interface{}) {
	if l.level <= INFO {
		l.log(l.info(" INFO"), format, a...)
	}
}

func (l *Logger) Warn(format string, a ...interface{}) {
	if l.level <= WARN {
		l.log(l.warn(" WARN"), format, a...)
	}
}

func (l *Logger) Error(format string, a ...interface{}) {
	if l.level <= ERROR {
		l.log(l.err("ERROR"), format, a...)
	}
}

func (l *Logger) Fatal(format string, a ...interface{}) {
	l.log(l.fatal("FATAL"), format, a...)

	if l.ExitOnFatal {
		exit(1)
	}
}

// NewLogger creates a new logger
// Default level is INFO
func NewLogger(io *iostreams.IOStreams) *Logger {
	cs := io.ColorScheme()

	return &Logger{
		io:          io,
		level:       INFO,
		trace:       cs.SprintFunc("blue"),
		debug:       cs.SprintFunc("green"),
		info:        cs.SprintFunc("cyan"),
		warn:        cs.SprintFunc("yellow"),
		err:         cs.SprintFunc("red"),
		fatal:       cs.SprintFunc("white+b:red"),
		ExitOnFatal: true,
	}
}
