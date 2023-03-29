package kconfig

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/pkg/errors"

	"kraftkit.sh/log"
)

const makefilePreamble = `
	CONFIG_UK_BASE				:= $(UK_BASE)
	CONFIG_UK_APP					:= $(UK_APP)
	CONFIG_UK_PLAT        := $(CONFIG_UK_BASE)/plat/
	CONFIG_UK_LIB         := $(CONFIG_UK_BASE)/lib/
	CONFIG_CONFIG_IN      := $(CONFIG_UK_BASE)/Config.uk
	CONFIG                := $(CONFIG_UK_BASE)/support/kconfig
	CONFIGLIB	      := $(CONFIG_UK_BASE)/support/kconfiglib
	UK_CONFIG_OUT         := $(BUILD_DIR)/config
	UK_GENERATED_INCLUDES := $(BUILD_DIR)/include
	KCONFIG_DIR           := $(BUILD_DIR)/kconfig
	UK_FIXDEP             := $(KCONFIG_DIR)/fixdep
	KCONFIG_AUTOCONFIG    := $(KCONFIG_DIR)/auto.conf
	KCONFIG_TRISTATE      := $(KCONFIG_DIR)/tristate.config
	KCONFIG_AUTOHEADER    := $(UK_GENERATED_INCLUDES)/uk/_config.h
	KCONFIG_APP_DIR       := $(CONFIG_UK_APP)
	KCONFIG_LIB_IN        := $(KCONFIG_DIR)/libs.uk
	KCONFIG_DEF_PLATS     := $(shell, find $(CONFIG_UK_PLAT)/* -maxdepth 0 \
				-type d \( -path $(CONFIG_UK_PLAT)/common -o \
				-path $(CONFIG_UK_PLAT)/drivers \
				\) -prune -o  -type d -print)
	KCONFIG_LIB_DIR       := $(shell, find $(CONFIG_UK_LIB)/* -maxdepth 0 -type d) \
				$(CONFIG_UK_BASE)/lib $(ELIB_DIR)
	KCONFIG_PLAT_DIR      := $(KCONFIG_DEF_PLATS) $(EPLAT_DIR) $(CONFIG_UK_PLAT)
	KCONFIG_PLAT_IN       := $(KCONFIG_DIR)/plat.uk

	# Makefile support scripts
	SCRIPTS_DIR := $(CONFIG_UK_BASE)/support/scripts
`

type kconfigPreprocessor struct {
	ctx                     context.Context
	data                    []byte
	file                    string
	current                 string
	col                     int
	line                    int
	inComment               bool
	currentLineIsAssignment bool
	err                     error
	env                     KeyValueMap
	stack                   []*bytes.Buffer
}

var handler = map[string]func(ctx context.Context, args ...string) (string, error){
	"shell": evaluateShell,
	// other handlers defined here
}

func evaluateShell(ctx context.Context, args ...string) (string, error) {
	log.G(ctx).Debugf("Shell: %v", args)
	cmd := exec.Command("sh", "-c", strings.Join(args, " "))
	// TODO: use the proper working directory here!
	var outb, errb bytes.Buffer
	cmd.Stdout = &outb
	cmd.Stderr = &errb

	err := cmd.Run()
	if err != nil {
		return "", errors.Wrap(err, errb.String())
	}

	value := outb.String()
	value = strings.TrimSpace(strings.Replace(value, "\n", " ", -1))
	log.G(ctx).Debugf("Evaluates to : %q", value)
	return value, nil
}

func preambleEnv(env KeyValueMap) KeyValueMap {
	preambleParser := &kconfigPreprocessor{
		ctx:   context.Background(),
		data:  []byte(makefilePreamble),
		file:  "preamble",
		env:   env,
		stack: []*bytes.Buffer{new(bytes.Buffer)},
	}

	_, err := preambleParser.process()
	if err != nil {
		return nil
	}

	return preambleParser.env
}

func newPreprocessor(data []byte, file string, env KeyValueMap) *kconfigPreprocessor {
	return &kconfigPreprocessor{
		ctx:   context.Background(),
		data:  data,
		file:  file,
		env:   env,
		stack: []*bytes.Buffer{new(bytes.Buffer)},
	}
}

func (p *kconfigPreprocessor) process() ([]byte, error) {
	for p.nextLine() {
		p.parseLine()
		if p.currentLineIsAssignment {
			p.handleAssignment()
		}
		p.writeByte('\n')
	}

	if p.err != nil {
		return nil, p.err
	}
	// println(p.stack[0].String())
	return p.stack[0].Bytes(), nil
}

func (p *kconfigPreprocessor) handleAssignment() {
	if p.err != nil {
		return
	}

	if p.inSubstitution() {
		p.failf("assignment in substitution not supported")
		return
	}

	// go backwards through the byte buffer and look for ':='
	top := p.stack[len(p.stack)-1]
	var rhsStartingIndex int
	var lhsStartingIndex int
	for i := top.Len() - 2; i >= 0; i-- {
		if top.Bytes()[i] == ':' && top.Bytes()[i+1] == '=' {
			rhsStartingIndex = i + 2
		}

		if top.Bytes()[i] == '\n' {
			lhsStartingIndex = i + 1
			break
		}
	}

	lhs := strings.TrimSpace(string(top.Bytes()[lhsStartingIndex : rhsStartingIndex-2]))
	rhs := strings.TrimSpace(string(top.Bytes()[rhsStartingIndex:]))

	p.env[lhs] = &KeyValue{Key: lhs, Value: rhs}
	log.G(p.ctx).Debugf("assignment: %s = %s", lhs, rhs)
}

func (p *kconfigPreprocessor) inSubstitution() bool {
	return len(p.stack) > 1
}

func (p *kconfigPreprocessor) parseLine() {
	for p.err == nil && !p.eol() {
		if p.inComment {
			p.char()
			continue
		}

		switch p.peek() {
		case '#':
			p.inComment = true
			p.char()
		case '$':
			p.skip()
			if p.peek() == '(' {
				p.skip()
				p.pushSubstitution()
			} else {
				p.writeByte('$')
			}
		case ':':
			p.char()

			if p.inSubstitution() {
				continue
			}

			if p.peek() == '=' {
				p.char()
				p.currentLineIsAssignment = true
			}

		case ')':
			if p.inSubstitution() {
				p.skip()
				p.popSubstitution()
			} else {
				p.char()
			}
		case '\\':
			p.char()
			p.char()
		default:
			p.char()
		}
	}
}

func (p *kconfigPreprocessor) pushSubstitution() {
	if p.err != nil {
		return
	}

	p.stack = append(p.stack, new(bytes.Buffer))
}

func (p *kconfigPreprocessor) popSubstitution() {
	if p.err != nil {
		return
	}

	if len(p.stack) == 1 {
		// no substitution happening right now
		p.char()
		return
	}

	top := p.stack[len(p.stack)-1]
	p.stack = p.stack[:len(p.stack)-1]
	substitution := top.String()
	substituted := p.evaluate(substitution)
	p.writeString(substituted)
}

func (p *kconfigPreprocessor) evaluate(substitution string) string {
	if p.err != nil {
		return ""
	}

	log.G(p.ctx).Debugf("evaluating substitution %q", substitution)

	splits := strings.Split(strings.TrimSpace(substitution), ",")
	if len(splits) == 1 {
		return p.lookupEnv(substitution)
	}

	// first split is the key command
	command := strings.TrimSpace(splits[0])
	if handler, ok := handler[command]; ok {
		// call the handler
		value, err := handler(p.ctx, splits[1:]...)
		if err != nil {
			p.failf("error evaluating %q: %v", substitution, err)
			return ""
		}
		return value
	}

	p.failf("unknown substitution handler %q", command)
	return ""
}

func (p *kconfigPreprocessor) lookupEnv(key string) string {
	if p.err != nil {
		return ""
	}

	if v, ok := p.env[key]; ok {
		return v.Value
	}

	p.failf("unknown substitution %q", key)
	return ""
}

func (p *kconfigPreprocessor) writeByte(c byte) {
	if p.err != nil {
		return
	}

	if len(p.stack) == 0 {
		p.failf("bug: no preprocessor buffer")
		return
	}
	p.stack[len(p.stack)-1].WriteByte(c)
}

func (p *kconfigPreprocessor) writeString(s string) {
	if p.err != nil {
		return
	}

	if len(p.stack) == 0 {
		p.failf("bug: no preprocessor buffer")
	}

	p.stack[len(p.stack)-1].WriteString(s)
}

// nextLine resets the parser to the next line.
// Automatically concatenates lines split with \ at the end.
func (p *kconfigPreprocessor) nextLine() bool {
	if !p.eol() {
		p.failf("tailing data at the end of line")
		return false
	}

	if p.err != nil || len(p.data) == 0 {
		return false
	}

	p.col = 0
	p.line++
	p.current = p.readNextLine()
	p.inComment = false
	p.currentLineIsAssignment = false

	for p.current != "" && p.current[len(p.current)-1] == '\\' && len(p.data) != 0 {
		p.current = p.current[:len(p.current)-1] + p.readNextLine()
		p.line++
	}

	p.skipSpaces()
	return true
}

func (p *kconfigPreprocessor) eol() bool {
	return p.col == len(p.current)
}

func (p *kconfigPreprocessor) peek() byte {
	if p.err != nil || p.eol() {
		return 0
	}

	return p.current[p.col]
}

func (p *kconfigPreprocessor) skip() byte {
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

func (p *kconfigPreprocessor) char() byte {
	if p.err != nil {
		return 0
	}

	if p.eol() {
		p.failf("unexpected end of line")
		return 0
	}

	v := p.current[p.col]
	p.writeByte(v)
	p.col++
	return v
}

func (p *kconfigPreprocessor) skipSpaces() {
	for p.col < len(p.current) && (p.current[p.col] == ' ' || p.current[p.col] == '\t') {
		p.writeByte(p.current[p.col])
		p.col++
	}
}

func (p *kconfigPreprocessor) failf(msg string, args ...interface{}) {
	if p.err == nil {
		p.err = fmt.Errorf("%v:%v:%v: %v\n%v", p.file, p.line, p.col, fmt.Sprintf(msg, args...), p.current)
	}
}

func (p *kconfigPreprocessor) readNextLine() string {
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
