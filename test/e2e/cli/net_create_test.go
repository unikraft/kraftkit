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

var _ = Describe("kraft net create", func() {
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
		cmd.Args = append(cmd.Args, "net", "create", "--log-level", "info", "--log-type", "json")
	})

	When("invoked without flags or positional arguments", func() {
		It("should print an error and exit with an error", func() {
			err := cmd.Run()
			Expect(err).To(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp(`^{"level":"error","msg":"accepts 1 arg\(s\), received 0"}\n`))
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
			Expect(stdout.String()).To(MatchRegexp(`^Create a new machine network.\n`))
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

	When("invoked with one positional argument and a network without a subnet", func() {
		BeforeEach(func() {
			cmd.Args = append(cmd.Args, "--network", "172.45.1.1")
			cmd.Args = append(cmd.Args, "t-cr-0")
		})

		It("should print an error and exit with an error", func() {
			err := cmd.Run()
			Expect(err).To(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp(`^{"level":"error","msg":"invalid CIDR address: 172\.45\.1\.1"}\n$`))
		})
	})

	When("invoked with one positional argument and a network without a complete ip", func() {
		BeforeEach(func() {
			cmd.Args = append(cmd.Args, "--network", "172.45.2/24")
			cmd.Args = append(cmd.Args, "test-cr-1")
		})

		It("should print an error and exit with an error", func() {
			err := cmd.Run()
			Expect(err).To(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp(`^{"level":"error","msg":"invalid CIDR address: 172\.45\.2/24"}\n$`))
		})
	})

	When("invoked with one positional argument and an invalid network", func() {
		BeforeEach(func() {
			cmd.Args = append(cmd.Args, "--network", "1234")
			cmd.Args = append(cmd.Args, "t-cr-2")
		})

		It("should print an error and exit with an error", func() {
			err := cmd.Run()
			Expect(err).To(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp(`^{"level":"error","msg":"invalid CIDR address: 1234"}\n$`))
		})
	})

	When("invoked with one positional argument, invalid driver, and a valid network", func() {
		BeforeEach(func() {
			cmd.Args = append(cmd.Args, "--driver", "unknown")
			cmd.Args = append(cmd.Args, "--network", "172.45.3.1/24")
			cmd.Args = append(cmd.Args, "t-cr-3")
		})

		It("should print an error and exit", func() {
			err := cmd.Run()
			Expect(err).To(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp(`^{"level":"error","msg":"unsupported network driver strategy: unknown \(contributions welcome\!\)"}\n$`))
		})
	})

	// Requires root privileges
	When("invoked with one positional argument, valid driver, and a valid network", func() {
		BeforeEach(func() {
			cmd.Args = append(cmd.Args, "--driver", "bridge")
			cmd.Args = append(cmd.Args, "--network", "172.45.4.1/24")
			cmd.Args = append(cmd.Args, "t-cr-4")
		})

		AfterEach(func() {
			// Remove the network
			stdoutRm := fcmd.NewIOStream()
			stderrRm := fcmd.NewIOStream()
			cmdRm := fcmd.NewKraftPrivileged(stdoutRm, stderrRm, cfg.Path())
			cmdRm.Args = append(cmdRm.Args, "net", "rm", "t-cr-4")

			err := cmdRm.Run()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())
		})

		It("should create the network, print the network name, and exit", func() {
			err := cmd.Run()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())
			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp(`^t-cr-4\n$`))

			// Check that the network exists
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
			Expect(stdoutLs.String()).To(MatchRegexp(`t-cr-4[\t ]+172.45.4.1/24[\t ]+bridge[\t ]+up`))
		})
	})

	When("invoked with one positional argument without a network", func() {
		BeforeEach(func() {
			cmd.Args = append(cmd.Args, "t-cr-5")
		})

		AfterEach(func() {
			// Remove the network
			stdoutRm := fcmd.NewIOStream()
			stderrRm := fcmd.NewIOStream()
			cmdRm := fcmd.NewKraftPrivileged(stdoutRm, stderrRm, cfg.Path())
			cmdRm.Args = append(cmdRm.Args, "net", "rm", "t-cr-5")

			err := cmdRm.Run()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())
		})

		It("should create the network, print the network name, and exit", func() {
			err := cmd.Run()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())
			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp(`^t-cr-5\n$`))

			// Check that the network exists
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
			Expect(stdoutLs.String()).To(MatchRegexp(`t-cr-5[\t ]+172.18.0.1/16[\t ]+bridge[\t ]+up`))
		})
	})
})
