// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package cli_test

import (
	"encoding/json"
	"fmt"
	"runtime"

	. "github.com/onsi/ginkgo/v2" //nolint:stylecheck
	. "github.com/onsi/gomega"    //nolint:stylecheck

	fcmd "kraftkit.sh/test/e2e/framework/cmd"
	fcfg "kraftkit.sh/test/e2e/framework/config"
)

var _ = Describe("kraft net inspect", func() {
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
		cmd.Args = append(cmd.Args, "net", "inspect", "--log-level", "info", "--log-type", "json")
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
			cmd.Args = append(cmd.Args, "test-inspect-0")
		})

		It("should error out and exit with an error", func() {
			err := cmd.Run()
			Expect(err).To(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp(`^{"level":"error","msg":"could not get link test-inspect-0: Link not found"}\n$`))
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
			cmdCreate.Args = append(cmdCreate.Args, "--network", "172.50.0.1/24")
			cmdCreate.Args = append(cmdCreate.Args, "test-inspect-1")

			err := cmdCreate.Run()
			if err != nil {
				fmt.Printf("Error running command, dumping output:\n%s\n%s\n%s\n", err, stderr, stdout)
			}
			Expect(err).ToNot(HaveOccurred())
			Expect(stderrCreate.String()).To(BeEmpty())
			Expect(stdoutCreate.String()).To(MatchRegexp(`^test-inspect-1\n$`))

			cmd.Args = append(cmd.Args, "--driver", "bridge")
			cmd.Args = append(cmd.Args, "test-inspect-1")
		})

		AfterEach(func() {
			stdoutRm := fcmd.NewIOStream()
			stderrRm := fcmd.NewIOStream()
			cmdRm := fcmd.NewKraftPrivileged(stdoutRm, stderrRm, cfg.Path())
			cmdRm.Args = append(cmdRm.Args, "net", "rm", "test-inspect-1")
			err := cmdRm.Run()
			if err != nil {
				fmt.Printf("Error running command, dumping output:\n%s\n%s\n%s\n", err, stderr, stdout)
			}
			Expect(err).ToNot(HaveOccurred())
		})

		It("should print a detailed, valid, json for the interface", func() {
			err := cmd.Run()
			if err != nil {
				fmt.Printf("Error running command, dumping output:\n%s\n%s\n%s\n", err, stderr, stdout)
			}
			Expect(err).ToNot(HaveOccurred())
			Expect(stderr.String()).To(BeEmpty())

			// Unmarshal stdout to json
			inspectData := make(map[string]interface{})
			err = json.Unmarshal([]byte(stdout.String()), &inspectData)
			if err != nil {
				fmt.Printf("Error running command, dumping output:\n%s\n%s\n%s\n", err, stderr, stdout)
			}
			Expect(err).ToNot(HaveOccurred())

			// Check if network is correct
			Expect(inspectData["spec"].(map[string]interface{})["ifName"]).To(Equal("test-inspect-1"))
			// Check if the network is up
			Expect(inspectData["status"].(map[string]interface{})["state"]).To(Equal("up"))
		})
	})

	When("invoked with the --help flag", func() {
		BeforeEach(func() {
			cmd.Args = append(cmd.Args, "--help")
		})

		It("should print the command's help", func() {
			err := cmd.Run()
			if err != nil {
				fmt.Printf("Error running command, dumping output:\n%s\n%s\n%s\n", err, stderr, stdout)
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp(`^Inspect a machine network\n`))
		})
	})
})
