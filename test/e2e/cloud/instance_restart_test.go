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

var _ = Describe("kraft cloud vm restart", func() {
	var cmd *fcmd.Cmd

	var stdout *fcmd.IOStream
	var stderr *fcmd.IOStream

	var cfg *fcfg.Config
	var instanceNameFull string

	const (
		imageName      = "nginx:latest"
		instanceName   = "instance-restart-test"
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
		createCmd.Args = append(createCmd.Args, "cloud", "instance", "create", "-p", "11111:8080/tls", "--log-level", "info", "--log-type", "json", "-o", "json", "--start")

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

		stdout = fcmd.NewIOStream()
		stderr = fcmd.NewIOStream()
		cmd = fcmd.NewKraft(stdout, stderr, cfg.Path())
		cmd.Env = os.Environ()
		cmd.Args = append(cmd.Args, "cloud", "vm", "restart", "--log-level", "info", "--log-type", "json")
	})

	AfterEach(func() {
		stdout = fcmd.NewIOStream()
		stderr = fcmd.NewIOStream()
		rmCmd := fcmd.NewKraft(stdout, stderr, cfg.Path())
		rmCmd.Env = os.Environ()
		rmCmd.Args = append(rmCmd.Args, "cloud", "vm", "rm", "--log-level", "info", "--log-type", "json", instanceNameFull)

		err := rmCmd.Run()
		if err != nil {
			fmt.Print(rmCmd.DumpError(stdout, stderr, err))
		}

		Expect(err).ToNot(HaveOccurred())
	})

	When("invoked with the instance name and the wait flag", func() {
		BeforeEach(func() {
			cmd.Args = append(cmd.Args, "--wait", "10s", instanceNameFull)
		})

		It("should restart the instance", func() {
			err := cmd.Run()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}

			Expect(err).ToNot(HaveOccurred())
			Expect(stderr.String()).To(MatchRegexp(`stopping 1 instance`))
			Expect(stderr.String()).To(MatchRegexp(`starting 1 instance`))
			Expect(stdout.String()).To(BeEmpty())

			// Check if the instance is restarted
			stdout = fcmd.NewIOStream()
			stderr = fcmd.NewIOStream()
			getCmd := fcmd.NewKraft(stdout, stderr, cfg.Path())
			getCmd.Env = os.Environ()
			getCmd.Args = append(getCmd.Args, "cloud", "vm", "get", "--log-level", "info", "--log-type", "json", "-o", "json", instanceNameFull)

			err = getCmd.Run()
			if err != nil {
				fmt.Print(getCmd.DumpError(stdout, stderr, err))
			}

			Expect(err).ToNot(HaveOccurred())
			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp(`start_count":"2"`))
		})
	})

	When("invoked with the all flag", Serial, func() {
		BeforeEach(func() {
			cmd.Args = append(cmd.Args, "--all")
		})

		It("should start all instances", func() {
			err := cmd.Run()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}

			Expect(err).ToNot(HaveOccurred())
			Expect(stderr.String()).To(MatchRegexp(`stopping 1 instance`))
			Expect(stderr.String()).To(MatchRegexp(`starting 1 instance`))
			Expect(stdout.String()).To(BeEmpty())

			// Check if the instance is restarted
			stdout = fcmd.NewIOStream()
			stderr = fcmd.NewIOStream()
			getCmd := fcmd.NewKraft(stdout, stderr, cfg.Path())
			getCmd.Env = os.Environ()
			getCmd.Args = append(getCmd.Args, "cloud", "vm", "get", "--log-level", "info", "--log-type", "json", "-o", "json", instanceNameFull)

			err = getCmd.Run()
			if err != nil {
				fmt.Print(getCmd.DumpError(stdout, stderr, err))
			}

			Expect(err).ToNot(HaveOccurred())
			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp(`start_count":"2"`))
		})
	})

	When("invoked with the name of a stopped instance", func() {
		BeforeEach(func() {
			cmd.Args = append(cmd.Args, instanceNameFull)

			// Stop the instance before restarting
			stopStdout := fcmd.NewIOStream()
			stopStderr := fcmd.NewIOStream()
			stopCmd := fcmd.NewKraft(stopStdout, stopStderr, cfg.Path())
			stopCmd.Env = os.Environ()
			stopCmd.Args = append(stopCmd.Args, "cloud", "vm", "stop", "--log-level", "info", "--log-type", "json", instanceNameFull)
			err := stopCmd.Run()
			if err != nil {
				fmt.Print(stopCmd.DumpError(stdout, stopStderr, err))
			}
			Expect(err).ToNot(HaveOccurred())
			Expect(stopStderr.String()).To(MatchRegexp(`stopping 1 instance`))
		})

		It("should start the instance", func() {
			err := cmd.Run()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}

			Expect(err).ToNot(HaveOccurred())
			Expect(stderr.String()).To(MatchRegexp(`stopping 1 instance`))
			Expect(stderr.String()).To(MatchRegexp(`starting 1 instance`))
			Expect(stdout.String()).To(BeEmpty())

			// Check if the instance is restarted
			stdout = fcmd.NewIOStream()
			stderr = fcmd.NewIOStream()
			getCmd := fcmd.NewKraft(stdout, stderr, cfg.Path())
			getCmd.Env = os.Environ()
			getCmd.Args = append(getCmd.Args, "cloud", "vm", "get", "--log-level", "info", "--log-type", "json", "-o", "json", instanceNameFull)

			err = getCmd.Run()
			if err != nil {
				fmt.Print(getCmd.DumpError(stdout, stderr, err))
			}

			Expect(err).ToNot(HaveOccurred())
			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp(`start_count":"2"`))
		})
	})

	When("invoked with the instance name", func() {
		BeforeEach(func() {
			cmd.Args = append(cmd.Args, instanceNameFull)
		})

		It("should restart the instance", func() {
			err := cmd.Run()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}

			Expect(err).ToNot(HaveOccurred())
			Expect(stderr.String()).To(MatchRegexp(`stopping 1 instance`))
			Expect(stderr.String()).To(MatchRegexp(`starting 1 instance`))
			Expect(stdout.String()).To(BeEmpty())

			// Check if the instance is restarted
			stdout = fcmd.NewIOStream()
			stderr = fcmd.NewIOStream()
			getCmd := fcmd.NewKraft(stdout, stderr, cfg.Path())
			getCmd.Env = os.Environ()
			getCmd.Args = append(getCmd.Args, "cloud", "vm", "get", "--log-level", "info", "--log-type", "json", "-o", "json", instanceNameFull)

			err = getCmd.Run()
			if err != nil {
				fmt.Print(getCmd.DumpError(stdout, stderr, err))
			}

			Expect(err).ToNot(HaveOccurred())
			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp(`start_count":"2"`))
		})
	})

	When("invoked with two instance names", func() {
		var instanceNameFull2 string

		BeforeEach(func() {
			createStdout := fcmd.NewIOStream()
			createStderr := fcmd.NewIOStream()
			createCmd := fcmd.NewKraft(createStdout, createStderr, cfg.Path())
			createCmd.Env = os.Environ()
			createCmd.Args = append(createCmd.Args, "cloud", "instance", "create", "-p", "11111:8080/tls", "--log-level", "info", "--log-type", "json", "-o", "json")

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
				fmt.Print(createCmd.DumpError(createStdout, createStderr, err))
			}

			Expect(err).ToNot(HaveOccurred())
			Expect(createStderr.String()).To(BeEmpty())
			Expect(createStdout.String()).To(MatchRegexp(`stopped`))

			cmd.Args = append(cmd.Args, instanceNameFull, instanceNameFull2)
		})

		AfterEach(func() {
			stdout = fcmd.NewIOStream()
			stderr = fcmd.NewIOStream()
			rmCmd := fcmd.NewKraft(stdout, stderr, cfg.Path())
			rmCmd.Env = os.Environ()
			rmCmd.Args = append(rmCmd.Args, "cloud", "vm", "rm", "--log-level", "info", "--log-type", "json", instanceNameFull2)

			err := rmCmd.Run()
			if err != nil {
				fmt.Print(rmCmd.DumpError(stdout, stderr, err))
			}

			Expect(err).ToNot(HaveOccurred())
		})

		It("should restart the instances", func() {
			err := cmd.Run()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}

			Expect(err).ToNot(HaveOccurred())
			Expect(stderr.String()).To(MatchRegexp(`stopping 2 instance`))
			Expect(stderr.String()).To(MatchRegexp(`starting 2 instance`))
			Expect(stdout.String()).To(BeEmpty())

			// Check if the instance is restarted
			getStdout := fcmd.NewIOStream()
			getStderr := fcmd.NewIOStream()
			getCmd := fcmd.NewKraft(getStdout, getStderr, cfg.Path())
			getCmd.Env = os.Environ()
			getCmd.Args = append(getCmd.Args, "cloud", "vm", "get", "--log-level", "info", "--log-type", "json", "-o", "json", instanceNameFull)

			err = getCmd.Run()
			if err != nil {
				fmt.Print(getCmd.DumpError(getStdout, getStderr, err))
			}

			Expect(err).ToNot(HaveOccurred())
			Expect(getStderr.String()).To(BeEmpty())
			Expect(getStdout.String()).To(MatchRegexp(`start_count":"2"`))

			// Check if the instance is restarted
			getStdout2 := fcmd.NewIOStream()
			getStderr2 := fcmd.NewIOStream()
			getCmd2 := fcmd.NewKraft(getStdout2, getStderr2, cfg.Path())
			getCmd2.Env = os.Environ()
			getCmd2.Args = append(getCmd2.Args, "cloud", "vm", "get", "--log-level", "info", "--log-type", "json", "-o", "json", instanceNameFull2)

			err = getCmd2.Run()
			if err != nil {
				fmt.Print(getCmd.DumpError(getStdout2, getStderr2, err))
			}

			Expect(err).ToNot(HaveOccurred())
			Expect(getStderr2.String()).To(BeEmpty())
			Expect(getStdout2.String()).To(MatchRegexp(`start_count":"1"`))
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
			Expect(stdout.String()).To(MatchRegexp(`Restart instance\(s\) on Unikraft Cloud.`))
		})
	})
})
