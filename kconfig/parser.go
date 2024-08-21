// SPDX-License-Identifier: Apache-2.0
// Copyright 2020 syzkaller project authors. All rights reserved.
// Copyright 2022 Unikraft GmbH. All rights reserved.
// Licensed under the Apache-2.0 License (the "License").
// You may not use this file except in compliance with the License.

package kconfig

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// parser is a helper for parsing simple text protocols tailored for kconfig syntax.
type parser struct {
	data    []byte
	file    string
	current string
	col     int
	line    int
	err     error
	env     KeyValueMap
	baseDir string
}

func newParser(data []byte, baseDir, file string, env KeyValueMap) *parser {
	return &parser{
		data:    data,
		file:    file,
		env:     env,
		baseDir: baseDir,
	}
}

// nextLine resets the parser to the next line.
// Automatically concatenates lines split with \ at the end.
func (p *parser) nextLine() bool {
	if p.err != nil || len(p.data) == 0 {
		return false
	}

	p.col = 0
	p.line++
	p.current = p.readNextLine()

	for p.current != "" && p.current[len(p.current)-1] == '\\' && len(p.data) != 0 {
		p.current = p.current[:len(p.current)-1] + p.readNextLine()
		p.line++
	}

	p.skipSpaces()
	return true
}

func (p *parser) readNextLine() string {
	line := ""
	nextLine := bytes.IndexByte(p.data, '\n')
	if nextLine != -1 {
		line = string(p.data[:nextLine])
		p.data = p.data[nextLine+1:]
	} else {
		line = string(p.data)
		p.data = nil
	}

	return line
}

func (p *parser) skipSpaces() {
	for p.col < len(p.current) && (p.current[p.col] == ' ' || p.current[p.col] == '\t') {
		p.col++
	}
}

func (p *parser) identLevel() int {
	level := 0
	for i := 0; i < p.col; i++ {
		level++
		if p.current[i] == '\t' {
			level = (level + 7) & ^7
		}
	}

	return level
}

func (p *parser) failf(msg string, args ...interface{}) {
	if p.err == nil {
		p.err = fmt.Errorf("%v:%v:%v: %v\n%v", p.file, p.line, p.col, fmt.Sprintf(msg, args...), p.current)
	}
}

func (p *parser) eol() bool {
	return p.col == len(p.current)
}

func (p *parser) char() byte {
	if p.err != nil {
		return 0
	}

	if p.eol() {
		p.failf("unexpected end of line")
		return 0
	}

	v := p.current[p.col]
	p.col++
	return v
}

func (p *parser) peek() byte {
	if p.err != nil || p.eol() {
		return 0
	}

	return p.current[p.col]
}

func (p *parser) ConsumeLine() string {
	res := p.current[p.col:]
	p.col = len(p.current)

	return res
}

func (p *parser) TryConsume(what string) bool {
	if !strings.HasPrefix(p.current[p.col:], what) {
		return false
	}

	p.col += len(what)
	p.skipSpaces()

	return true
}

func (p *parser) MustConsume(what string) {
	if !p.TryConsume(what) {
		p.failf("expected %q", what)
	}
}

func (p *parser) interpolate(b []byte) string {
	var vars []string
	var v []byte
	parsing := false
	str := string(b)

	for i := 0; i < len(b); i++ {
		// Test if we're parsing a variable which starts with `$(` and not a shell
		// invocation which starts with `$(shell `.
		if i+1 <= len(b) && b[i] == '$' && b[i+1] == '(' &&
			(i+8 <= len(b) && string(b[i+2:i+7]) != "shell") {
			i += 2 // skip the '$('
			parsing = true

			// If we're busy accepting a variable name and come to an end `)` we can
			// stop and save.
		} else if parsing && b[i] == ')' {
			parsing = false
			vars = append(vars, string(v))
			v = []byte{}
		}

		// Save the next character to the variable name if we're parsing.
		if parsing {
			v = append(v, b[i])
		}
	}

	for _, v := range vars {
		if v == "" {
			continue
		}

		// Evaluate known variables found from the provided environment
		if !strings.HasPrefix("shell", v) {
			replace, ok := p.env[v]
			if !ok {
				// Try with the prefix `CONFIG_`
				replace, ok = p.env["CONFIG_"+v]
				if !ok {
					continue
				}
			}

			str = strings.ReplaceAll(str, fmt.Sprintf("$(%s)", v), replace.Value)
		}
	}

	// Evaluate shell executions
	if len(str) > 7 && str[0:8] == "$(shell," {
		line := strings.TrimPrefix(str, "$(shell,")
		line = strings.TrimSuffix(line, ")")

		quoted := false
		args := strings.FieldsFunc(line, func(r rune) bool {
			if r == '\'' || r == '"' {
				quoted = !quoted
			}
			return !quoted && r == ' '
		})

		for i := range args {
			args[i] = strings.TrimSpace(args[i])
			args[i] = strings.TrimPrefix(args[i], "'")
			args[i] = strings.TrimSuffix(args[i], "'")
		}

		var buf bytes.Buffer
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = p.baseDir
		cmd.Stdout = bufio.NewWriter(&buf)
		if err := cmd.Run(); err != nil {
			p.failf("could not execute shell: %s", err.Error())
			return ""
		}
		str = strings.TrimSpace(buf.String())
	}

	return str
}

func (p *parser) QuotedString() string {
	var str []byte
	quote := p.char()
	if quote != '"' && quote != '\'' {
		p.failf("expect quoted string")
	}

	for ch := p.char(); ch != quote; ch = p.char() {
		if ch == 0 {
			p.failf("unterminated quoted string")
			break
		}
		if ch == '\\' {
			ch = p.char()
			switch ch {
			case '\'', '"', '\\':
				str = append(str, ch)
			default:
				p.failf("bad quoted character")
			}

			continue
		}

		str = append(str, ch)
		if ch == '$' && p.peek() == '(' {
			str = append(str, p.Shell()...)
		}
	}

	p.skipSpaces()
	return p.interpolate(str)
}

func (p *parser) TryQuotedString() (string, bool) {
	if ch := p.peek(); ch == '"' || ch == '\'' {
		return p.QuotedString(), true
	}

	return "", false
}

func (p *parser) Ident() string {
	var str []byte
	for !p.eol() {
		ch := p.peek()
		if ch >= 'a' && ch <= 'z' ||
			ch >= 'A' && ch <= 'Z' ||
			ch >= '0' && ch <= '9' ||
			ch == '_' || ch == '-' {
			str = append(str, ch)
			p.col++
			continue
		}
		break
	}

	if len(str) == 0 {
		p.failf("expected an identifier")
	}

	p.skipSpaces()
	return string(str)
}

func (p *parser) Shell() string {
	start := p.col
	p.MustConsume("(")
	for !p.eol() && p.peek() != ')' {
		if p.peek() == '"' {
			p.QuotedString()
		} else if p.peek() == '(' {
			p.Shell()
		} else {
			p.col++
		}
	}

	if ch := p.char(); ch != ')' {
		p.failf("shell expression is not terminated")
	}

	res := p.current[start:p.col]
	p.skipSpaces()
	return res
}
