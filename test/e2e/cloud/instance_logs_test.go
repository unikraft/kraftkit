// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package cloud_test

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"os"

	. "github.com/onsi/ginkgo/v2" //nolint:stylecheck
	. "github.com/onsi/gomega"    //nolint:stylecheck

	fcmd "kraftkit.sh/test/e2e/framework/cmd"
	fcfg "kraftkit.sh/test/e2e/framework/config"
)

var _ = Describe("kraft cloud vm logs", func() {
	var cmd *fcmd.Cmd

	var stdout *fcmd.IOStream
	var stderr *fcmd.IOStream

	var cfg *fcfg.Config
	var instanceNameFull string

	const (
		imageName      = "nginx:latest"
		instanceName   = "instance-logs-test"
		instanceMemory = "64"
	)

	BeforeEach(func() {
		token := os.Getenv("UNIKRAFTCLOUD_TOKEN")

		if token == "" {
			token = os.Getenv("KRAFTCLOUD_TOKEN")
		}

		if token == "" {
			token = os.Getenv("KC_TOKEN")
		}

		if token == "" {
			token = os.Getenv("UKC_TOKEN")
		}

		if token == "" {
			Skip("UNIKRAFTCLOUD_TOKEN is not set")
		}

		metro := os.Getenv("UNIKRAFTCLOUD_METRO")

		if metro == "" {
			metro = os.Getenv("KRAFTCLOUD_METRO")
		}

		if metro == "" {
			metro = os.Getenv("KC_METRO")
		}

		if metro == "" {
			metro = os.Getenv("UKC_METRO")
		}

		if metro == "" {
			Skip("UNIKRAFTCLOUD_METRO is not set")
		}

		stdout = fcmd.NewIOStream()
		stderr = fcmd.NewIOStream()

		cfg = fcfg.NewTempConfig()

		createCmd := fcmd.NewKraft(stdout, stderr, cfg.Path())
		createCmd.Env = os.Environ()
		createCmd.Args = append(createCmd.Args, "cloud", "instance", "create", "--log-level", "info", "--log-type", "json", "-o", "json")

		id, err := rand.Int(rand.Reader, big.NewInt(100000000000))
		if err != nil {
			panic(err)
		}
		instanceNameFull = fmt.Sprintf("%s-%d", instanceName, id)

		createCmd.Args = append(createCmd.Args,
			"--memory", instanceMemory,
			"--name", instanceNameFull,
			"--start",
			imageName,
		)

		err = createCmd.Run()
		if err != nil {
			fmt.Print(createCmd.DumpError(stdout, stderr, err))
		}

		Expect(err).ToNot(HaveOccurred())
		Expect(stderr.String()).To(BeEmpty())
		Expect(stdout.String()).To(MatchRegexp(`stopped`))

		cmd = fcmd.NewKraft(stdout, stderr, cfg.Path())
		cmd.Env = os.Environ()
		cmd.Args = append(cmd.Args, "cloud", "vm", "logs", "--log-level", "info", "--log-type", "json")
	})

	AfterEach(func() {
		rmCmd := fcmd.NewKraft(stdout, stderr, cfg.Path())
		rmCmd.Env = os.Environ()
		rmCmd.Args = append(rmCmd.Args, "cloud", "vm", "rm", "--log-level", "info", "--log-type", "json", instanceNameFull)

		err := rmCmd.Run()
		if err != nil {
			fmt.Print(rmCmd.DumpError(stdout, stderr, err))
		}

		Expect(err).ToNot(HaveOccurred())
	})

	When("invoked with an instance name", func() {
		BeforeEach(func() {
			cmd.Args = append(cmd.Args, instanceNameFull)
		})

		It("should show all log lines", func() {
			err := cmd.Run()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}

			Expect(err).ToNot(HaveOccurred())
			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp(`Powered by Unikraft`))
			Expect(stdout.String()).To(MatchRegexp(`1: Set IPv4 address`))
		})
	})

	When("invoked with an instance name and the tail flag", func() {
		BeforeEach(func() {
			cmd.Args = append(cmd.Args, "--tail", "1", instanceNameFull)
		})

		It("should show only the last given lines from the log", func() {
			err := cmd.Run()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}

			Expect(err).ToNot(HaveOccurred())
			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp(`Powered by Unikraft`))
			Expect(stdout.String()).ToNot(MatchRegexp(`1: Set IPv4 address`))
		})
	})
	When("invoked with an instance name and the follow flag", func() {
		BeforeEach(func() {
			cmd.Args = append(cmd.Args, "--follow", instanceNameFull)
		})

		It("should show all lines in the log and wait for input", func() {
			err := cmd.Start()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}

			// Check if the process is running
			pid := cmd.Process.Pid
			_, err = os.FindProcess(pid)
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			err = cmd.Process.Kill()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			err = cmd.Process.Release()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())
		})
	})

	When("invoked with two instance names", func() {
		var instanceNameFull2 string

		BeforeEach(func() {
			createCmd := fcmd.NewKraft(stdout, stderr, cfg.Path())
			createCmd.Env = os.Environ()
			createCmd.Args = append(createCmd.Args, "cloud", "instance", "create", "--log-level", "info", "--log-type", "json", "-o", "json")

			id, err := rand.Int(rand.Reader, big.NewInt(100000000000))
			if err != nil {
				panic(err)
			}
			instanceNameFull2 = fmt.Sprintf("%s-%d", instanceName, id)

			createCmd.Args = append(createCmd.Args,
				"--memory", instanceMemory,
				"--name", instanceNameFull2,
				"--start",
				imageName,
			)

			err = createCmd.Run()
			if err != nil {
				fmt.Print(createCmd.DumpError(stdout, stderr, err))
			}

			Expect(err).ToNot(HaveOccurred())
			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp(`running`))

			cmd.Args = append(cmd.Args, instanceNameFull, instanceNameFull2)
		})

		AfterEach(func() {
			rmCmd := fcmd.NewKraft(stdout, stderr, cfg.Path())
			rmCmd.Env = os.Environ()
			rmCmd.Args = append(rmCmd.Args, "cloud", "vm", "rm", "--log-level", "info", "--log-type", "json", instanceNameFull2)

			err := rmCmd.Run()
			if err != nil {
				fmt.Print(rmCmd.DumpError(stdout, stderr, err))
			}

			Expect(err).ToNot(HaveOccurred())
			Expect(stderr.String()).To(BeEmpty())
		})

		It("should show all log lines and a prefix for each instance", func() {
			err := cmd.Run()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}

			Expect(err).ToNot(HaveOccurred())
			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp(`Powered by Unikraft`))
			Expect(stdout.String()).To(MatchRegexp(`1: Set IPv4 address`))
			Expect(stdout.String()).To(MatchRegexp(instanceNameFull))
			Expect(stdout.String()).To(MatchRegexp(instanceNameFull2))
		})
	})

	When("invoked with two instance names and the 'no-prefix' flag", func() {
		var instanceNameFull2 string

		BeforeEach(func() {
			createCmd := fcmd.NewKraft(stdout, stderr, cfg.Path())
			createCmd.Env = os.Environ()
			createCmd.Args = append(createCmd.Args, "cloud", "instance", "create", "--log-level", "info", "--log-type", "json", "-o", "json")

			id, err := rand.Int(rand.Reader, big.NewInt(100000000000))
			if err != nil {
				panic(err)
			}
			instanceNameFull2 = fmt.Sprintf("%s-%d", instanceName, id)

			createCmd.Args = append(createCmd.Args,
				"--memory", instanceMemory,
				"--name", instanceNameFull2,
				"--start",
				imageName,
			)

			err = createCmd.Run()
			if err != nil {
				fmt.Print(createCmd.DumpError(stdout, stderr, err))
			}

			Expect(err).ToNot(HaveOccurred())
			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp(`running`))

			cmd.Args = append(cmd.Args, "--no-prefix", instanceNameFull, instanceNameFull2)
		})

		AfterEach(func() {
			rmCmd := fcmd.NewKraft(stdout, stderr, cfg.Path())
			rmCmd.Env = os.Environ()
			rmCmd.Args = append(rmCmd.Args, "cloud", "vm", "rm", "--log-level", "info", "--log-type", "json", instanceNameFull2)

			err := rmCmd.Run()
			if err != nil {
				fmt.Print(rmCmd.DumpError(stdout, stderr, err))
			}

			Expect(err).ToNot(HaveOccurred())
			Expect(stderr.String()).To(BeEmpty())
		})

		It("should show all log lines and and no prefix", func() {
			err := cmd.Run()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}

			Expect(err).ToNot(HaveOccurred())
			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp(`Powered by Unikraft`))
			Expect(stdout.String()).To(MatchRegexp(`1: Set IPv4 address`))
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
			Expect(stdout.String()).To(MatchRegexp(`Get console output of an instance`))
		})
	})
})
