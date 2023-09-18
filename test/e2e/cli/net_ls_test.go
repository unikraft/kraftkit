// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package cli_test

import (
	. "github.com/onsi/ginkgo/v2" //nolint:stylecheck
	. "github.com/onsi/gomega"    //nolint:stylecheck

	fcmd "kraftkit.sh/test/e2e/framework/cmd"
	fcfg "kraftkit.sh/test/e2e/framework/config"
)

var _ = Describe("kraft net ls", func() {
	var cmd *fcmd.Cmd

	var stdout *fcmd.IOStream
	var stderr *fcmd.IOStream

	var cfg *fcfg.Config

	BeforeEach(func() {
		stdout = fcmd.NewIOStream()
		stderr = fcmd.NewIOStream()

		cfg = fcfg.NewTempConfig()

		cmd = fcmd.NewKraftPrivileged(stdout, stderr, cfg.Path())
		cmd.Args = append(cmd.Args, "net", "ls", "--log-level", "info", "--log-type", "json")
	})

	When("invoked with a positional argument", func() {
		BeforeEach(func() {
			cmd.Args = append(cmd.Args, "test")
		})

		It("should print an error and exit with an error", func() {
			err := cmd.Run()
			Expect(err).To(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp(`^{"level":"error","msg":"unknown command \\"test\\" for \\"kraft net ls\\""}`))
		})
	})

	// Requires root privileges
	When("invoked without flags or positional arguments", func() {
		BeforeEach(func() {
			stdoutCreate := fcmd.NewIOStream()
			stderrCreate := fcmd.NewIOStream()
			cmdCreate := fcmd.NewKraftPrivileged(stdoutCreate, stderrCreate, cfg.Path())
			cmdCreate.Args = append(cmdCreate.Args, "net", "create", "--log-level", "info", "--log-type", "json")
			cmdCreate.Args = append(cmdCreate.Args, "--driver", "bridge")
			cmdCreate.Args = append(cmdCreate.Args, "--network", "172.47.0.1/24")
			cmdCreate.Args = append(cmdCreate.Args, "test-ls-0")

			err := cmdCreate.Run()

			Expect(err).ToNot(HaveOccurred())
			Expect(stderr.String()).To(BeEmpty())

			cmd = fcmd.NewKraftPrivileged(stdout, stderr, cfg.Path())
			cmd.Args = append(cmd.Args, "net", "ls", "--log-level", "info", "--log-type", "json")
		})

		AfterEach(func() {
			stdoutRm := fcmd.NewIOStream()
			stderrRm := fcmd.NewIOStream()
			cmdRm := fcmd.NewKraftPrivileged(stdoutRm, stderrRm, cfg.Path())
			cmdRm.Args = append(cmdRm.Args, "net", "rm", "--log-level", "info", "--log-type", "json")
			cmdRm.Args = append(cmdRm.Args, "test-ls-0")

			err := cmdRm.Run()

			Expect(err).ToNot(HaveOccurred())
			Expect(stderrRm.String()).To(BeEmpty())
		})

		It("should print a table of networks and exit", func() {
			err := cmd.Run()

			Expect(err).ToNot(HaveOccurred())
			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp(`^NAME[\t ]+NETWORK[\t ]+DRIVER[\t ]+STATUS\n`))
			Expect(stdout.String()).To(MatchRegexp(`test-ls-0[\t ]+172.47.0.1/24[\t ]+bridge[\t ]+up`))
		})
	})

	When("invoked with the --help flag", func() {
		BeforeEach(func() {
			cmd.Args = append(cmd.Args, "--help")
		})

		It("should print the command's help", func() {
			err := cmd.Run()
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp(`^List machine networks\n`))
		})
	})
})
