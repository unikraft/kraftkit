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

var _ = Describe("kraft cloud vm rm", func() {
	var cmd *fcmd.Cmd

	var stdout *fcmd.IOStream
	var stderr *fcmd.IOStream

	var cfg *fcfg.Config
	var instanceNameFull string

	const (
		imageName      = "nginx:latest"
		instanceName   = "instance-remove-test"
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
		Expect(stdout.String()).To(MatchRegexp(`running`))

		cmd = fcmd.NewKraft(stdout, stderr, cfg.Path())
		cmd.Env = os.Environ()
		cmd.Args = append(cmd.Args, "cloud", "vm", "rm", "--log-level", "info", "--log-type", "json")
	})

	When("invoked with the all flag", Serial, func() {
		BeforeEach(func() {
			cmd.Args = append(cmd.Args, "--all")
		})

		It("should remove all instances", func() {
			err := cmd.Run()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}

			Expect(err).ToNot(HaveOccurred())
			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp(`removing 1 instance`))

			// Check if the instance still exists
			getCmd := fcmd.NewKraft(stdout, stderr, cfg.Path())
			getCmd.Env = os.Environ()
			getCmd.Args = append(getCmd.Args, "cloud", "vm", "get", "--log-level", "info", "--log-type", "json", "-o", "json", instanceNameFull)

			err = getCmd.Run()

			Expect(err).To(HaveOccurred())
			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp("No instance with name '" + instanceNameFull + "'"))
		})
	})

	When("invoked with the instance name", func() {
		BeforeEach(func() {
			cmd.Args = append(cmd.Args, instanceNameFull)
		})

		It("should remove the instance", func() {
			err := cmd.Run()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}

			Expect(err).ToNot(HaveOccurred())
			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp(`removing 1 instance`))

			// Check if the instance still exists
			getCmd := fcmd.NewKraft(stdout, stderr, cfg.Path())
			getCmd.Env = os.Environ()
			getCmd.Args = append(getCmd.Args, "cloud", "vm", "get", "--log-level", "info", "--log-type", "json", "-o", "json", instanceNameFull)

			err = getCmd.Run()

			Expect(err).To(HaveOccurred())
			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp("No instance with name '" + instanceNameFull + "'"))
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
				imageName,
			)

			err = createCmd.Run()
			if err != nil {
				fmt.Print(createCmd.DumpError(stdout, stderr, err))
			}

			Expect(err).ToNot(HaveOccurred())
			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp(`stopped`))

			cmd.Args = append(cmd.Args, instanceNameFull, instanceNameFull2)
		})

		It("should remove the instances", func() {
			err := cmd.Run()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}

			Expect(err).ToNot(HaveOccurred())
			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp(`removing 2 instance`))

			// Check if the instance still exists
			getCmd := fcmd.NewKraft(stdout, stderr, cfg.Path())
			getCmd.Env = os.Environ()
			getCmd.Args = append(getCmd.Args, "cloud", "vm", "get", "--log-level", "info", "--log-type", "json", "-o", "json", instanceNameFull)

			err = getCmd.Run()

			Expect(err).To(HaveOccurred())
			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp("No instance with name '" + instanceNameFull + "'"))

			// Check if the instance still exists
			getCmd2 := fcmd.NewKraft(stdout, stderr, cfg.Path())
			getCmd2.Env = os.Environ()
			getCmd2.Args = append(getCmd2.Args, "cloud", "vm", "get", "--log-level", "info", "--log-type", "json", "-o", "json", instanceNameFull2)

			err = getCmd2.Run()

			Expect(err).To(HaveOccurred())
			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp("No instance with name '" + instanceNameFull2 + "'"))
		})
	})

	When("invoked with the 'stopped' flag", Serial, func() {
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
				imageName,
			)

			err = createCmd.Run()
			if err != nil {
				fmt.Print(createCmd.DumpError(stdout, stderr, err))
			}

			Expect(err).ToNot(HaveOccurred())
			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp(`stopped`))

			cmd.Args = append(cmd.Args, "--stopped")
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
			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp(`removing 1 instance`))
		})

		It("should remove only the stopped instances", func() {
			err := cmd.Run()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}

			Expect(err).ToNot(HaveOccurred())
			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp(`removing 1 instance`))

			// Check if the instance still exists
			getCmd := fcmd.NewKraft(stdout, stderr, cfg.Path())
			getCmd.Env = os.Environ()
			getCmd.Args = append(getCmd.Args, "cloud", "vm", "get", "--log-level", "info", "--log-type", "json", "-o", "json", instanceNameFull)

			err = getCmd.Run()
			if err != nil {
				fmt.Print(getCmd.DumpError(stdout, stderr, err))
			}

			Expect(err).ToNot(HaveOccurred())
			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())

			// Check if the instance still exists
			getCmd2 := fcmd.NewKraft(stdout, stderr, cfg.Path())
			getCmd2.Env = os.Environ()
			getCmd2.Args = append(getCmd2.Args, "cloud", "vm", "get", "--log-level", "info", "--log-type", "json", "-o", "json", instanceNameFull2)

			err = getCmd2.Run()

			Expect(err).To(HaveOccurred())
			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp("No instance with name '" + instanceNameFull2 + "'"))
		})
	})

	When("invoked with the --help flag", func() {
		BeforeEach(func() {
			cmd.Args = append(cmd.Args, "--help")
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
			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp(`removing 1 instance`))
		})

		It("should print the command's help", func() {
			err := cmd.Run()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp(`Remove a`))
		})
	})
})
