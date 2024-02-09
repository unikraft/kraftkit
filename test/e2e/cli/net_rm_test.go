// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package cli_test

import (
	"fmt"
	"runtime"

	. "github.com/onsi/ginkgo/v2" //nolint:stylecheck
	. "github.com/onsi/gomega"    //nolint:stylecheck

	fcmd "kraftkit.sh/test/e2e/framework/cmd"
	fcfg "kraftkit.sh/test/e2e/framework/config"
)

var _ = Describe("kraft net rm", func() {
	var cmd *fcmd.Cmd

	var stdout *fcmd.IOStream
	var stderr *fcmd.IOStream

	var cfg *fcfg.Config

	BeforeEach(func() {
		if runtime.GOOS != "linux" {
			Skip("This test only supports Linux. See here for more information: https://github.com/unikraft/kraftkit/issues/840")
		}

		stdout = fcmd.NewIOStream()
		stderr = fcmd.NewIOStream()

		cfg = fcfg.NewTempConfig()

		cmd = fcmd.NewKraftPrivileged(stdout, stderr, cfg.Path())
		cmd.Args = append(cmd.Args, "net", "rm", "--log-level", "info", "--log-type", "json")
	})

	When("invoked without flags or positional arguments", func() {
		It("should print an error and exit with an error", func() {
			err := cmd.Run()
			Expect(err).To(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp(`^{"level":"error","msg":"accepts 1 arg\(s\), received 0"}\n`))
		})
	})

	When("invoked with two positional arguments", func() {
		BeforeEach(func() {
			cmd.Args = append(cmd.Args, "some-arg")
			cmd.Args = append(cmd.Args, "some-other-arg")
		})

		It("should print an error and exit with an error", func() {
			err := cmd.Run()
			Expect(err).To(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp(`^{"level":"error","msg":"accepts 1 arg\(s\), received 2"}\n$`))
		})
	})

	When("invoked with the --help flag", func() {
		BeforeEach(func() {
			cmd.Args = append(cmd.Args, "--help")
		})

		It("should print the command's help", func() {
			err := cmd.Run()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp(`^Remove a network.\n`))
		})
	})

	// Requires root privileges
	When("invoked with one positional argument, valid driver, and a valid network", func() {
		BeforeEach(func() {
			stdoutCreate := fcmd.NewIOStream()
			stderrCreate := fcmd.NewIOStream()
			cmdCreate := fcmd.NewKraftPrivileged(stdoutCreate, stderrCreate, cfg.Path())
			cmdCreate.Args = append(cmdCreate.Args, "net", "create", "--log-level", "info", "--log-type", "json")
			cmdCreate.Args = append(cmdCreate.Args, "--driver", "bridge")
			cmdCreate.Args = append(cmdCreate.Args, "--network", "172.46.0.1/24")
			cmdCreate.Args = append(cmdCreate.Args, "test-rm-0")

			err := cmdCreate.Run()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())
			Expect(stderr.String()).To(BeEmpty())

			cmd.Args = append(cmd.Args, "--driver", "bridge")
			cmd.Args = append(cmd.Args, "test-rm-0")
		})

		It("should delete the network, print the network name, and exit", func() {
			err := cmd.Run()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())
			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp(`^test-rm-0\n$`))

			// Check that the network no longer exists
			stdoutLs := fcmd.NewIOStream()
			stderrLs := fcmd.NewIOStream()
			cmdLs := fcmd.NewKraftPrivileged(stdoutLs, stderrLs, cfg.Path())
			cmdLs.Args = append(cmdLs.Args, "net", "ls", "--log-level", "info", "--log-type", "json")

			err = cmdLs.Run()

			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())
			Expect(stderrLs.String()).To(BeEmpty())
			Expect(stdoutLs.String()).To(MatchRegexp(`^NAME[\t ]+NETWORK[\t ]+DRIVER[\t ]+STATUS\n`))
			Expect(stdoutLs.String()).ToNot(MatchRegexp(`test-rm-0[\t ]+172.46.0.1/24[\t ]+bridge[\t ]+up`))
		})
	})

	// Requires root privileges
	When("invoked with one positional argument, valid driver, and an invalid network", func() {
		BeforeEach(func() {
			cmd.Args = append(cmd.Args, "--driver", "bridge")
			cmd.Args = append(cmd.Args, "test-rm-1")
		})

		It("should delete the network, print the network name, and exit", func() {
			err := cmd.Run()
			Expect(err).To(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp(`^{"level":"error","msg":"getting bridge test-rm-1 failed: Link not found"}\n$`))
		})
	})
})
