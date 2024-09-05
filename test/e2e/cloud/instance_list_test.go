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

var _ = Describe("kraft cloud vm ls", func() {
	var cmd *fcmd.Cmd

	var stdout *fcmd.IOStream
	var stderr *fcmd.IOStream

	var cfg *fcfg.Config
	var instanceNameFull string

	const (
		imageName      = "nginx:latest"
		instanceName   = "instance-ls-test"
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
		cmd.Args = append(cmd.Args, "cloud", "vm", "ls", "--log-level", "info", "--log-type", "json")
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

	When("invoked with no flags", func() {
		It("should show all instances in table format", func() {
			err := cmd.Run()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}

			Expect(err).ToNot(HaveOccurred())
			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp(`NAME`))
			Expect(stdout.String()).To(MatchRegexp(instanceNameFull))
		})
	})

	When("invoked with the list output format", func() {
		BeforeEach(func() {
			cmd.Args = append(cmd.Args, "-o", "list")
		})

		It("should show instances in the list format", func() {
			err := cmd.Run()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}

			Expect(err).ToNot(HaveOccurred())
			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp(`name: ` + instanceNameFull))
		})
	})

	When("invoked with the json output format", func() {
		BeforeEach(func() {
			cmd.Args = append(cmd.Args, "-o", "json")
		})

		It("should instances in the json format", func() {
			err := cmd.Run()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}

			Expect(err).ToNot(HaveOccurred())
			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp(`"name":"` + instanceNameFull + `"`))
		})
	})

	When("invoked with the raw output format", func() {
		BeforeEach(func() {
			cmd.Args = append(cmd.Args, "-o", "raw")
		})

		It("should show instances in the raw format", func() {
			err := cmd.Run()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}

			Expect(err).ToNot(HaveOccurred())
			Expect(stderr.String()).To(BeEmpty())

			// Field exclusive to the raw output format
			Expect(stdout.String()).To(MatchRegexp(`"status":"success"`))
			Expect(stdout.String()).To(MatchRegexp(`"name":"` + instanceNameFull + `"`))
		})
	})

	When("invoked with the table output format", func() {
		BeforeEach(func() {
			cmd.Args = append(cmd.Args, "-o", "table")
		})

		It("should show instances in the table format", func() {
			err := cmd.Run()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}

			Expect(err).ToNot(HaveOccurred())
			Expect(stderr.String()).To(BeEmpty())

			// Output exclusive to the table output format
			Expect(stdout.String()).To(MatchRegexp(`NAME`))
			Expect(stdout.String()).To(MatchRegexp(instanceNameFull))
		})
	})

	When("invoked with an invalid output format", func() {
		format := "invalid"

		BeforeEach(func() {
			cmd.Args = append(cmd.Args, "-o", format)
		})

		It("should error out and print a message", func() {
			err := cmd.Run()

			Expect(err).To(HaveOccurred())
			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp(`invalid output format: ` + format))
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
			Expect(stdout.String()).To(MatchRegexp(`List all instances in your account`))
		})
	})
})
