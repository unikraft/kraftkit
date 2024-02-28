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

var _ = Describe("kraft net up", func() {
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
		cmd.Args = append(cmd.Args, "net", "up", "--log-level", "info", "--log-type", "json")
	})

	When("invoked without flags or positional arguments", func() {
		It("should print an error and exit with an error", func() {
			err := cmd.Run()
			Expect(err).To(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp(`^{"level":"error","msg":"accepts 1 arg\(s\), received 0"}\n`))
		})
	})

	// Requires root privileges
	When("invoked with a non-existing network", func() {
		BeforeEach(func() {
			cmd.Args = append(cmd.Args, "--driver", "bridge")
			cmd.Args = append(cmd.Args, "test-up-0")
		})

		It("should error out and exit with an error", func() {
			err := cmd.Run()
			Expect(err).To(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp(`^{"level":"error","msg":"getting bridge test-up-0 failed: Link not found"}\n$`))
		})
	})

	// Requires root privileges
	When("invoked with an existing network", func() {
		BeforeEach(func() {
			// Create the network
			stdoutCreate := fcmd.NewIOStream()
			stderrCreate := fcmd.NewIOStream()
			cmdCreate := fcmd.NewKraftPrivileged(stdoutCreate, stderrCreate, cfg.Path())
			cmdCreate.Args = append(cmdCreate.Args, "net", "create", "--log-level", "info", "--log-type", "json")
			cmdCreate.Args = append(cmdCreate.Args, "--driver", "bridge")
			cmdCreate.Args = append(cmdCreate.Args, "--network", "172.49.0.1/24")
			cmdCreate.Args = append(cmdCreate.Args, "test-up-1")

			err := cmdCreate.Run()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())
			Expect(stderrCreate.String()).To(BeEmpty())

			cmd.Args = append(cmd.Args, "--driver", "bridge")
			cmd.Args = append(cmd.Args, "test-up-1")
		})

		AfterEach(func() {
			stdoutRm := fcmd.NewIOStream()
			stderrRm := fcmd.NewIOStream()
			cmdRm := fcmd.NewKraftPrivileged(stdoutRm, stderrRm, cfg.Path())
			cmdRm.Args = append(cmdRm.Args, "net", "rm", "test-up-1")
			err := cmdRm.Run()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())
		})

		It("should bring the network up and print its name", func() {
			// Bring the network down
			stdoutDown := fcmd.NewIOStream()
			stderrDown := fcmd.NewIOStream()
			cmdDown := fcmd.NewKraftPrivileged(stdoutDown, stderrDown, cfg.Path())
			cmdDown.Args = append(cmdDown.Args, "net", "down", "--log-level", "info", "--log-type", "json")
			cmdDown.Args = append(cmdDown.Args, "--driver", "bridge")
			cmdDown.Args = append(cmdDown.Args, "test-up-1")

			err := cmdDown.Run()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())
			Expect(stderrDown.String()).To(BeEmpty())
			Expect(stdoutDown.String()).To(MatchRegexp(`^test-up-1\n$`))

			// Check if the network is down
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
			Expect(stdoutLs.String()).To(MatchRegexp(`test-up-1[\t ]+172.49.0.1/24[\t ]+bridge[\t ]+down`))

			// Bring the network back up
			err = cmd.Run()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())
			Expect(stderr.String()).To(BeEmpty())

			// Check if the network is up
			stdoutLs = fcmd.NewIOStream()
			stderrLs = fcmd.NewIOStream()
			cmdLs = fcmd.NewKraftPrivileged(stdoutLs, stderrLs, cfg.Path())
			cmdLs.Args = append(cmdLs.Args, "net", "ls", "--log-level", "info", "--log-type", "json")

			err = cmdLs.Run()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())
			Expect(stderrLs.String()).To(BeEmpty())
			Expect(stdoutLs.String()).To(MatchRegexp(`^NAME[\t ]+NETWORK[\t ]+DRIVER[\t ]+STATUS\n`))
			Expect(stdoutLs.String()).To(MatchRegexp(`test-up-1[\t ]+172.49.0.1/24[\t ]+bridge[\t ]+up`))
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
			Expect(stdout.String()).To(MatchRegexp(`^Bring a network online.\n`))
		})
	})
})
