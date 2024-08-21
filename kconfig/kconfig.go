// SPDX-License-Identifier: Apache-2.0
// Copyright 2020 syzkaller project authors. All rights reserved.
// Copyright 2022 Unikraft GmbH. All rights reserved.

// Package kconfig implements parsing of the Linux kernel Kconfig and .config
// files and provides some algorithms to work with these files. For Kconfig
// reference see:
// https://www.kernel.org/doc/html/latest/kbuild/kconfig-language.html

package kconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// KConfigFile represents a parsed Kconfig file (including includes).
type KConfigFile struct {
	// Root is the main menu
	Root *KConfigMenu `json:"root,omitempty"`

	// All config/menuconfig entries
	Configs map[string]*KConfigMenu `json:"configs,omitempty"`
}

// recursiveWalk accepts an input menu and uses the provided callback against
// all visited nodes.
func recursiveWalk(menu *KConfigMenu, cb func(*KConfigMenu) error) error {
	if err := cb(menu); err != nil {
		return err
	}

	for _, child := range menu.Children {
		if err := recursiveWalk(child, cb); err != nil {
			return err
		}
	}

	return nil
}

// Walk iterates through each node expressed in the KConfig DAG and executes
// the provided callback.
func (file *KConfigFile) Walk(cb func(*KConfigMenu) error) error {
	for _, menu := range file.Configs {
		if err := recursiveWalk(menu, cb); err != nil {
			return err
		}
	}

	return nil
}

// KConfigMenu represents a single hierarchical menu or config.
type KConfigMenu struct {
	// Kind represents the structure type, e.g. config/menu/choice/etc
	Kind MenuKind `json:"kind,omitempty"`

	// Type of menu element, e.g. tristate/bool/string/etc
	Type ConfigType `json:"type,omitempty"`

	// Name without CONFIG_
	Name string `json:"name,omitempty"`

	// Sub-elements for menus
	Children []*KConfigMenu `json:"children,omitempty"`

	// Prompt is the 1-line description of the menu entry.
	Prompt KConfigPrompt `json:"prompt,omitempty"`

	// Help information about the menu item.
	Help string `json:"help,omitempty"`

	// Default value of the entry.
	Default DefaultValue `json:"default,omitempty"`

	// Source of the KConfig file that enabled this menu.
	Source string `json:"source,omitempty"`

	// Parent menu, non-nil for everythign except for mainmenu
	parent      *KConfigMenu
	kconfigFile *KConfigFile // back-link to the owning KConfig
	dependsOn   expr
	visibleIf   expr
	deps        map[string]bool
	depsOnce    sync.Once
}

type KConfigPrompt struct {
	Text      string `json:"text,omitempty"`
	Condition expr   `json:"condition,omitempty"`
}

type DefaultValue struct {
	Value     expr `json:"value,omitempty"`
	Condition expr `json:"condition,omitempty"`
}

type (
	MenuKind   string
	ConfigType string
)

const (
	MenuMain       = MenuKind("main")
	MenuMenuConfig = MenuKind("menuconfig")
	MenuConfig     = MenuKind("config")
	MenuGroup      = MenuKind("group")
	MenuChoice     = MenuKind("choice")
	MenuComment    = MenuKind("comment")
)

const (
	TypeBool     = ConfigType("bool")
	TypeTristate = ConfigType("tristate")
	TypeString   = ConfigType("string")
	TypeInt      = ConfigType("int")
	TypeHex      = ConfigType("hex")
)

// DependsOn returns all transitive configs this config depends on.
func (m *KConfigMenu) DependsOn() map[string]bool {
	m.depsOnce.Do(func() {
		m.deps = make(map[string]bool)
		if m.dependsOn != nil {
			m.dependsOn.collectDeps(m.deps)
		}
		if m.visibleIf != nil {
			m.visibleIf.collectDeps(m.deps)
		}
		var indirect []string
		for cfg := range m.deps {
			dep := m.kconfigFile.Configs[cfg]
			if dep == nil {
				delete(m.deps, cfg)
				continue
			}
			for cfg1 := range dep.DependsOn() {
				indirect = append(indirect, cfg1)
			}
		}
		for _, cfg := range indirect {
			m.deps[cfg] = true
		}
	})
	return m.deps
}

type kconfigParser struct {
	*parser
	includes  []*parser
	stack     []*KConfigMenu
	cur       *KConfigMenu
	baseDir   string
	helpIdent int
}

func Parse(file string, env ...*KeyValue) (*KConfigFile, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("failed to open Kconfig file %v: %v", file, err)
	}
	return ParseData(data, file, env...)
}

func ParseData(data []byte, file string, extra ...*KeyValue) (*KConfigFile, error) {
	env := KeyValueMap{}
	for _, kcv := range extra {
		env[kcv.Key] = kcv
	}
	kp := &kconfigParser{
		parser:  newParser(data, filepath.Dir(file), file, env),
		baseDir: filepath.Dir(file),
	}

	kp.parseFile()
	if kp.err != nil {
		return nil, kp.err
	}

	if len(kp.stack) == 0 {
		return nil, fmt.Errorf("no mainmenu in config")
	}

	root := kp.stack[0]
	kconf := &KConfigFile{
		Root:    root,
		Configs: make(map[string]*KConfigMenu),
	}

	kconf.walk(root, nil, nil)
	return kconf, nil
}

func (kconf *KConfigFile) walk(m *KConfigMenu, dependsOn, visibleIf expr) {
	m.kconfigFile = kconf
	m.dependsOn = exprAnd(dependsOn, m.dependsOn)
	m.visibleIf = exprAnd(visibleIf, m.visibleIf)

	if m.Kind == MenuConfig || m.Kind == MenuMenuConfig {
		kconf.Configs[m.Name] = m
	}

	for _, elem := range m.Children {
		kconf.walk(elem, m.dependsOn, m.visibleIf)
	}
}

func (kp *kconfigParser) parseFile() {
	for kp.nextLine() {
		kp.parseLine()
		if kp.TryConsume("#") {
			_ = kp.ConsumeLine()
		}
	}

	kp.endCurrent()
}

func (kp *kconfigParser) parseLine() {
	if kp.eol() {
		return
	}

	if kp.helpIdent != 0 {
		if kp.identLevel() >= kp.helpIdent {
			_ = kp.ConsumeLine()
			return
		}
		kp.helpIdent = 0
	}

	if kp.TryConsume("#") {
		_ = kp.ConsumeLine()
		return
	}

	// To make this package compatible with Linux, ignore error-if statements
	if kp.TryConsume("$(error-if") {
		_ = kp.ConsumeLine()
		return
	}

	ident := kp.Ident()
	if kp.TryConsume("=") || kp.TryConsume(":=") {
		// Macro definition, see:
		// https://www.kernel.org/doc/html/latest/kbuild/kconfig-macro-language.html
		// We don't use this for anything now.
		kp.ConsumeLine()
		return
	}

	kp.parseMenu(ident)
}

func (kp *kconfigParser) parseMenu(cmd string) {
	switch cmd {
	case "source":
		file, ok := kp.TryQuotedString()
		if !ok {
			file = kp.ConsumeLine()
		}

		kp.includeSource(file)

	case "mainmenu":
		kp.pushCurrent(&KConfigMenu{
			Kind:   MenuMain,
			Prompt: KConfigPrompt{Text: kp.QuotedString()},
			Source: filepath.Clean(kp.file),
		})

	case "comment":
		kp.newCurrent(&KConfigMenu{
			Kind:   MenuComment,
			Prompt: KConfigPrompt{Text: kp.QuotedString()},
			Source: filepath.Clean(kp.file),
		})

	case "menu":
		kp.pushCurrent(&KConfigMenu{
			Kind:   MenuGroup,
			Prompt: KConfigPrompt{Text: kp.QuotedString()},
			Source: filepath.Clean(kp.file),
		})

	case "if":
		kp.pushCurrent(&KConfigMenu{
			Kind:      MenuGroup,
			visibleIf: kp.parseExpr(),
			Source:    filepath.Clean(kp.file),
		})

	case "choice":
		kp.pushCurrent(&KConfigMenu{
			Kind:   MenuChoice,
			Source: filepath.Clean(kp.file),
		})

	case "endmenu", "endif", "endchoice":
		kp.popCurrent()

	case "config":
		kp.newCurrent(&KConfigMenu{
			Kind:   MenuConfig,
			Name:   kp.Ident(),
			Source: filepath.Clean(kp.file),
		})

	case "menuconfig":
		kp.newCurrent(&KConfigMenu{
			Kind:   MenuMenuConfig,
			Name:   kp.Ident(),
			Source: filepath.Clean(kp.file),
		})

	default:
		kp.parseConfigType(cmd)
	}
}

func (kp *kconfigParser) parseConfigType(typ string) {
	cur := kp.current()
	switch typ {

	case "tristate":
		cur.Type = TypeTristate
		kp.tryParsePrompt()

	case "def_tristate":
		cur.Type = TypeTristate
		kp.parseDefaultValue()

	case "bool":
		cur.Type = TypeBool
		kp.tryParsePrompt()

	case "def_bool":
		cur.Type = TypeBool
		kp.parseDefaultValue()

	case "int":
		cur.Type = TypeInt
		kp.tryParsePrompt()

	case "def_int":
		cur.Type = TypeInt
		kp.parseDefaultValue()

	case "hex":
		cur.Type = TypeHex
		kp.tryParsePrompt()

	case "def_hex":
		cur.Type = TypeHex
		kp.parseDefaultValue()

	case "string":
		cur.Type = TypeString
		kp.tryParsePrompt()

	case "def_string":
		cur.Type = TypeString
		kp.parseDefaultValue()
	default:
		kp.parseProperty(typ)
	}
}

func (kp *kconfigParser) parseProperty(prop string) {
	cur := kp.current()
	switch prop {

	case "prompt":
		kp.tryParsePrompt()

	case "depends":
		kp.MustConsume("on")
		cur.dependsOn = exprAnd(cur.dependsOn, kp.parseExpr())

	case "visible":
		kp.MustConsume("if")
		cur.visibleIf = exprAnd(cur.visibleIf, kp.parseExpr())

	case "select", "imply":
		_ = kp.Ident()
		if kp.TryConsume("if") {
			_ = kp.parseExpr()
		}

	case "option":
		// It can be 'option foo', or 'option bar="BAZ"'.
		kp.ConsumeLine()

	case "modules":

	case "optional":

	case "default":
		kp.parseDefaultValue()

	case "range":
		_, _ = kp.parseExpr(), kp.parseExpr() // from, to
		if kp.TryConsume("if") {
			_ = kp.parseExpr()
		}

	case "help", "---help---":
		kp.tryParseHelp()

	default:
		kp.failf("unknown line")
	}
}

func (kp *kconfigParser) includeSource(file string) {
	// ignore blank files
	if file == "" {
		return
	}
	kp.newCurrent(nil)
	if file[0] != filepath.Separator {
		file = filepath.Join(kp.baseDir, file)
	}
	data, err := os.ReadFile(file)
	if err != nil {
		kp.failf("%v", err)
		return
	}

	kp.includes = append(kp.includes, kp.parser)
	kp.parser = newParser(data, kp.baseDir, file, kp.env)
	kp.parseFile()
	err = kp.err
	kp.parser = kp.includes[len(kp.includes)-1]
	kp.includes = kp.includes[:len(kp.includes)-1]

	if kp.err == nil {
		kp.err = err
	}
}

func (kp *kconfigParser) pushCurrent(m *KConfigMenu) {
	kp.endCurrent()
	kp.cur = m
	kp.stack = append(kp.stack, m)
}

func (kp *kconfigParser) popCurrent() {
	kp.endCurrent()
	if len(kp.stack) < 2 {
		return
	}

	last := kp.stack[len(kp.stack)-1]
	kp.stack = kp.stack[:len(kp.stack)-1]
	top := kp.stack[len(kp.stack)-1]
	last.parent = top
	top.Children = append(top.Children, last)
}

func (kp *kconfigParser) newCurrent(m *KConfigMenu) {
	kp.endCurrent()
	kp.cur = m
}

func (kp *kconfigParser) current() *KConfigMenu {
	if kp.cur == nil {
		kp.failf("config property outside of config")
		return &KConfigMenu{}
	}

	return kp.cur
}

func (kp *kconfigParser) endCurrent() {
	if kp.cur == nil {
		return
	}

	if len(kp.stack) == 0 {
		kp.failf("unbalanced endmenu")
		kp.cur = nil
		return
	}

	top := kp.stack[len(kp.stack)-1]
	if top != kp.cur {
		kp.cur.parent = top
		top.Children = append(top.Children, kp.cur)
	}

	kp.cur = nil
}

func (kp *kconfigParser) tryParsePrompt() {
	if str, ok := kp.TryQuotedString(); ok {
		prompt := KConfigPrompt{
			Text: str,
		}

		if kp.TryConsume("if") {
			prompt.Condition = kp.parseExpr()
		}

		kp.current().Prompt = prompt
	}
}

func (kp *kconfigParser) parseDefaultValue() {
	def := DefaultValue{Value: kp.parseExpr()}
	if kp.TryConsume("if") {
		def.Condition = kp.parseExpr()
	}

	kp.current().Default = def
}

func (kp *kconfigParser) tryParseHelp() {
	var help []string
	baseHelpIdent := -1
	for kp.nextLine() {
		if kp.eol() {
			continue
		}
		if len(help) > 0 && kp.identLevel() < baseHelpIdent {
			break
		}
		if baseHelpIdent == -1 {
			baseHelpIdent = kp.identLevel()
		}
		help = append(help, kp.ConsumeLine())
		kp.helpIdent = kp.identLevel()
	}

	kp.current().Help = strings.Join(help, " ")
}
