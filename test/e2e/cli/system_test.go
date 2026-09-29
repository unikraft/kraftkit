// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package cli_test

import (
	"fmt"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2" //nolint:stylecheck
	. "github.com/onsi/gomega"    //nolint:stylecheck

	fcmd "kraftkit.sh/test/e2e/framework/cmd"
	fcfg "kraftkit.sh/test/e2e/framework/config"
)

var _ = Describe("kraft system", func() {
	var stdout *fcmd.IOStream
	var stderr *fcmd.IOStream

	var cfg *fcfg.Config

	kraft := func(args ...string) *fcmd.Cmd {
		stdout = fcmd.NewIOStream()
		stderr = fcmd.NewIOStream()

		cmd := fcmd.NewKraft(stdout, stderr, cfg.Path())
		tmpBase := filepath.Dir(filepath.Dir(cfg.Path()))
		cmd.Env = append(cmd.Env, "HOME="+tmpBase)
		cmd.Dir = tmpBase

		cmd.Args = append(cmd.Args, "system")
		cmd.Args = append(cmd.Args, args...)
		cmd.Args = append(cmd.Args, "--log-level", "error", "--log-type", "json")

		return cmd
	}

	// run executes a kraft system command that is expected to succeed.
	run := func(args ...string) {
		GinkgoHelper()

		cmd := kraft(args...)
		err := cmd.Run()
		if err != nil {
			fmt.Print(cmd.DumpError(stdout, stderr, err))
		}
		Expect(err).ToNot(HaveOccurred())
	}

	// list runs `kraft system list` and returns its output as lines. The order
	// of map entries (toolchain.*, aliases.*, auth.*) is random, so callers
	// must only check for the presence or absence of lines.
	list := func() []string {
		GinkgoHelper()

		run("list")
		Expect(stderr.String()).To(BeEmpty())

		return strings.Split(strings.TrimRight(stdout.String(), "\n"), "\n")
	}

	// value reads a scalar value from the config file on disk.
	value := func(path ...string) string {
		GinkgoHelper()

		node := cfg.Read(path...)
		Expect(node).ToNot(BeNil(), "expected %v in config file", path)

		return node.YNode().Value
	}

	BeforeEach(func() {
		cfg = fcfg.NewTempConfig()
	})

	Context("set", func() {
		It("should write a toolchain variable and keep existing keys", func() {
			run("set", "toolchain.CC=clang")
			Expect(stdout.String()).To(BeEmpty())
			Expect(stderr.String()).To(BeEmpty())

			Expect(value("toolchain", "CC")).To(Equal("clang"))
			Expect(cfg.Read("paths", "manifests")).ToNot(BeNil())
			Expect(cfg.Read("paths", "sources")).ToNot(BeNil())
		})

		It("should overwrite an existing value", func() {
			run("set", "toolchain.CC=clang")
			run("set", "toolchain.CC=gcc")

			Expect(value("toolchain", "CC")).To(Equal("gcc"))
		})

		It("should set several keys in one call", func() {
			run("set", "toolchain.CC=clang", "collect_anonymous_telemetry=true")

			Expect(value("toolchain", "CC")).To(Equal("clang"))
			Expect(value("collect_anonymous_telemetry")).To(Equal("true"))
		})
	})

	Context("list", func() {
		It("should print set values as key=value lines", func() {
			run("set", "toolchain.CC=clang", "toolchain.CXX=clang++", "collect_anonymous_telemetry=true")

			lines := list()
			Expect(lines).To(ContainElements(
				"toolchain.CC=clang",
				"toolchain.CXX=clang++",
				"collect_anonymous_telemetry=true",
			))
		})

		It("should not print the config directory", func() {
			Expect(list()).ToNot(ContainElement(HavePrefix("paths.config=")))
		})
	})

	Context("unset", func() {
		It("should remove a toolchain variable from the file and from list", func() {
			run("set", "toolchain.CC=clang")
			run("unset", "toolchain.CC")

			Expect(cfg.Read("toolchain", "CC")).To(BeNil())
			Expect(list()).ToNot(ContainElement(HavePrefix("toolchain.CC=")))
		})

		It("should only remove the given key", func() {
			run("set", "toolchain.CC=clang", "toolchain.UK_CFLAGS=-O2")
			run("unset", "toolchain.CC")

			Expect(cfg.Read("toolchain", "CC")).To(BeNil())
			Expect(value("toolchain", "UK_CFLAGS")).To(Equal("-O2"))
			Expect(value("paths", "manifests")).ToNot(BeEmpty())

			lines := list()
			Expect(lines).To(ContainElement("toolchain.UK_CFLAGS=-O2"))
			Expect(lines).ToNot(ContainElement(HavePrefix("toolchain.CC=")))
		})

		It("should remove several keys in one call", func() {
			run("set", "toolchain.CC=clang", "toolchain.UK_CFLAGS=-O2")
			run("unset", "toolchain.CC", "toolchain.UK_CFLAGS")

			Expect(cfg.Read("toolchain", "CC")).To(BeNil())
			Expect(cfg.Read("toolchain", "UK_CFLAGS")).To(BeNil())
			Expect(list()).ToNot(ContainElement(HavePrefix("toolchain.")))
		})

		It("should keep the key removed after a later set of another key", func() {
			run("set", "toolchain.CC=clang")
			run("unset", "toolchain.CC")
			run("set", "toolchain.UK_CFLAGS=-O2")

			Expect(cfg.Read("toolchain", "CC")).To(BeNil())
			Expect(value("toolchain", "UK_CFLAGS")).To(Equal("-O2"))
		})

		It("should succeed when the toolchain key does not exist", func() {
			run("unset", "toolchain.DOES_NOT_EXIST")

			Expect(cfg.Read("toolchain", "DOES_NOT_EXIST")).To(BeNil())
		})
	})

	Context("error cases", func() {
		expectFailure := func(substr string, args ...string) {
			GinkgoHelper()

			cmd := kraft(args...)
			err := cmd.Run()
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("exit status 1"))

			Expect(stdout.String()).To(BeEmpty())
			Expect(stderr.String()).To(ContainSubstring(substr))
		}

		It("should reject set without a value", func() {
			expectFailure("invalid argument: expected KEY=VALUE", "set", "toolchain.CC")
		})

		It("should reject set of an unknown key", func() {
			expectFailure("invalid key: does_not_exist", "set", "does_not_exist=1")
		})

		It("should reject set of a bool with a non-bool value", func() {
			expectFailure("unsupported type conversion", "set", "no_prompt=maybe")
		})

		It("should reject unset of an unknown key", func() {
			expectFailure("invalid key: does_not_exist", "unset", "does_not_exist")
		})

		It("should reject set without arguments", func() {
			expectFailure("requires at least 1 arg", "set")
		})

		It("should reject unset without arguments", func() {
			expectFailure("requires at least 1 arg", "unset")
		})

		It("should reject list with an argument", func() {
			expectFailure("unknown command", "list", "some-arg")
		})
	})

	Context("help", func() {
		It("should print system help", func() {
			cmd := kraft("--help")
			err := cmd.Run()
			Expect(err).ToNot(HaveOccurred())
			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp(`(?m)^Manage KraftKit and host system$`))
			Expect(stdout.String()).To(MatchRegexp(`(?m)^USAGE$`))
			Expect(stdout.String()).To(MatchRegexp(`(?m)^  kraft system SUBCOMMAND$`))
		})

		It("should print set help", func() {
			cmd := kraft("set", "--help")
			err := cmd.Run()
			Expect(err).ToNot(HaveOccurred())
			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp(`(?m)^Set a KraftKit configuration option$`))
			Expect(stdout.String()).To(MatchRegexp(`(?m)^USAGE$`))
			Expect(stdout.String()).To(MatchRegexp(`(?m)^  kraft system set KEY=VALUE$`))
		})

		It("should print list help", func() {
			cmd := kraft("list", "--help")
			err := cmd.Run()
			Expect(err).ToNot(HaveOccurred())
			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp(`(?m)^List all KraftKit configuration options and their current values\.$`))
			Expect(stdout.String()).To(MatchRegexp(`(?m)^USAGE$`))
			Expect(stdout.String()).To(MatchRegexp(`(?m)^  kraft system list \[FLAGS\]$`))
		})

		It("should print unset help", func() {
			cmd := kraft("unset", "--help")
			err := cmd.Run()
			Expect(err).ToNot(HaveOccurred())
			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp(`(?m)^Unset a KraftKit configuration option$`))
			Expect(stdout.String()).To(MatchRegexp(`(?m)^USAGE$`))
			Expect(stdout.String()).To(MatchRegexp(`(?m)^  kraft system unset KEY \[KEY \.\.\.\]$`))
		})
	})
})
