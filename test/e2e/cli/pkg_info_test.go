// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package cli_test

import (
	"fmt"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	fcmd "kraftkit.sh/test/e2e/framework/cmd"
	fcfg "kraftkit.sh/test/e2e/framework/config"
)

var _ = Describe("kraft pkg info", func() {
	var cmd *fcmd.Cmd
	var stdout *fcmd.IOStream
	var stderr *fcmd.IOStream
	var cfg *fcfg.Config
	var pkg string

	BeforeEach(func() {
		pkg = "unikraft.org/helloworld:latest"
		stdout = fcmd.NewIOStream()
		stderr = fcmd.NewIOStream()
		cfg = fcfg.NewTempConfig()

		cmd = fcmd.NewKraft(stdout, stderr, cfg.Path())

		// Force HOME to temp dir and prevent leaks from the host machine.
		tmpBase := filepath.Dir(filepath.Dir(cfg.Path()))
		cmd.Env = append(cmd.Env, "HOME="+tmpBase)
		cmd.Dir = tmpBase

		cmd.Args = append(cmd.Args, "pkg", "info", "--log-type", "json", "--output", "json")
	})

	Context("lookup package", func() {
		When("it exists", func() {
			BeforeEach(func() {
				pullCmd := fcmd.NewKraft(stdout, stderr, cfg.Path())
				pullCmd.Env = cmd.Env
				pullCmd.Dir = cmd.Dir
				pullCmd.Args = append(
					pullCmd.Args,
					"pkg",
					"pull",
					"--log-level",
					"error",
					"-u",
					pkg,
				)

				err := pullCmd.Run()
				if err != nil {
					fmt.Print(pullCmd.DumpError(stdout, stderr, err))
				}
				Expect(err).ToNot(HaveOccurred())

				stderr.Reset()
				stdout.Reset()

				cmd.Args = append(cmd.Args, pkg)
			})

			It("list package info and verify it exists in the jailed runtime", func() {
				err := cmd.Run()
				if err != nil {
					fmt.Print(cmd.DumpError(stdout, stderr, err))
				}
				Expect(err).ToNot(HaveOccurred())

				// Assert logs
				Expect(
					stderr.String(),
				).To(MatchRegexp(`{"level":"info","msg":"finding unikraft.org/helloworld:latest"}`))

				// Assert table content in stdout
				Expect(stdout.String()).To(ContainSubstring(`"name":"unikraft.org/helloworld"`))
				Expect(stdout.String()).To(ContainSubstring(`"version":"latest"`))
			})
		})

		When("it doesn't exist", func() {
			BeforeEach(func() {
				cmd.Args = append(cmd.Args, "unikraft.org/fake-pkg:latest")
			})

			It("should return a 404-style error", func() {
				err := cmd.Run()
				Expect(err).To(HaveOccurred())
				Expect(
					stderr.String(),
				).To(MatchRegexp(`{"level":"error","msg":"could not find: unikraft.org/fake-pkg:latest"}`))
			})
		})

		When("no package is provided", func() {
			It("should return a validation error", func() {
				err := cmd.Run()
				Expect(err).To(HaveOccurred())

				// Assert error log
				Expect(
					stderr.String(),
				).To(MatchRegexp(`{"level":"error","msg":"package name\(s\) not specified"}\n?`))
			})
		})

		When("invoked with the --help flag", func() {
			BeforeEach(func() {
				cmd.Args = append(cmd.Args, "--help")
			})

			It("should print the command's help header", func() {
				err := cmd.Run()
				Expect(err).ToNot(HaveOccurred())
				Expect(stderr.String()).To(BeEmpty())
				Expect(
					stdout.String(),
				).To(MatchRegexp(`^Shows a Unikraft package like library, core, etc.\n`))
			})
		})
	})
})
