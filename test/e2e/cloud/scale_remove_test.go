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
	"time"

	. "github.com/onsi/ginkgo/v2" //nolint:stylecheck
	. "github.com/onsi/gomega"    //nolint:stylecheck

	fcmd "kraftkit.sh/test/e2e/framework/cmd"
	fcfg "kraftkit.sh/test/e2e/framework/config"
)

var _ = Describe("kraft cloud scale remove", func() {
	var cmd *fcmd.Cmd

	var stdout *fcmd.IOStream
	var stderr *fcmd.IOStream

	var cfg *fcfg.Config

	const (
		imageName      = "nginx:latest"
		instanceName   = "instance-remove-test"
		serviceName    = "service-remove-test"
		policyName     = "policy-remove-test"
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

		cmd = fcmd.NewKraft(stdout, stderr, cfg.Path())
		cmd.Env = os.Environ()
		cmd.Args = append(cmd.Args, "cloud", "scale", "remove", "--log-level", "info", "--log-type", "json")
	})

	When("invoked with a policy to remove", func() {
		var instanceNameFull string
		var serviceNameFull string
		var policyNameFull1 string
		var serviceUUID string

		BeforeEach(func() {
			id, err := rand.Int(rand.Reader, big.NewInt(100000000))
			if err != nil {
				panic(err)
			}
			instanceNameFull = fmt.Sprintf("%s-%d", instanceName, id)
			serviceNameFull = fmt.Sprintf("%s-%d", serviceName, id)
			policyNameFull1 = fmt.Sprintf("%s-%d", policyName, id)

			serviceCreateCmd := fcmd.NewKraft(stdout, stderr, cfg.Path())
			serviceCreateCmd.Args = append(serviceCreateCmd.Args, "cloud", "service", "create",
				"--log-level", "info", "--log-type", "json", "-o", "json",
				"--name", serviceNameFull, "443:8080/tls+http")
			err = serviceCreateCmd.Run()
			if err != nil {
				fmt.Print(serviceCreateCmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp(serviceNameFull))

			// Extract the service UUID
			serviceUUID = serviceUUIDParser(stdout)
			Expect(serviceUUID).ToNot(BeEmpty())

			instanceCreateCmd := fcmd.NewKraft(stdout, stderr, cfg.Path())
			instanceCreateCmd.Env = os.Environ()
			instanceCreateCmd.Args = append(instanceCreateCmd.Args, "cloud", "instance", "create",
				"-o", "json", "--log-level", "info", "--log-type", "json",
				"--name", instanceNameFull,
				"--memory", instanceMemory,
				"--start",
				"--service", serviceNameFull,
				imageName)
			err = instanceCreateCmd.Run()
			if err != nil {
				fmt.Print(instanceCreateCmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())

			scaleInitCmd := fcmd.NewKraft(stdout, stderr, cfg.Path())
			scaleInitCmd.Args = append(scaleInitCmd.Args, "cloud", "scale", "init",
				"--master", instanceNameFull,
				"--min-size", "2",
				"--max-size", "10",
				"--cooldown-time", "13s",
				"--warmup-time", "26s",
				serviceNameFull)
			err = scaleInitCmd.Run()
			if err != nil {
				fmt.Print(scaleInitCmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())

			scaleAddCmd := fcmd.NewKraft(stdout, stderr, cfg.Path())
			scaleAddCmd.Env = os.Environ()
			scaleAddCmd.Args = append(scaleAddCmd.Args, "cloud", "scale", "add", serviceNameFull, "--name", policyNameFull1, "--step", "0:10/1", "--step", "10:20/2")
			err = scaleAddCmd.Run()
			time.Sleep(2 * time.Second)
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			scaleResetCmd := fcmd.NewKraft(stdout, stderr, cfg.Path())
			scaleResetCmd.Env = os.Environ()
			scaleResetCmd.Args = append(scaleResetCmd.Args, "cloud", "scale", "reset", serviceNameFull)
			err := scaleResetCmd.Run()
			if err != nil {
				fmt.Print(scaleResetCmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			instanceDeleteCmd := fcmd.NewKraft(stdout, stderr, cfg.Path())
			instanceDeleteCmd.Env = os.Environ()
			instanceDeleteCmd.Args = append(instanceDeleteCmd.Args, "cloud", "instance", "delete", instanceNameFull)
			err = instanceDeleteCmd.Run()
			time.Sleep(2 * time.Second)
			if err != nil {
				fmt.Print(instanceDeleteCmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			serviceDeleteCmd := fcmd.NewKraft(stdout, stderr, cfg.Path())
			serviceDeleteCmd.Env = os.Environ()
			serviceDeleteCmd.Args = append(serviceDeleteCmd.Args, "cloud", "service", "delete", serviceNameFull)
			err = serviceDeleteCmd.Run()
			time.Sleep(2 * time.Second)
			if err != nil {
				fmt.Print(serviceDeleteCmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())
		})

		It("should remove the policy from the configuration", func() {
			cmd.Args = append(cmd.Args, serviceUUID, policyNameFull1)
			err := cmd.Run()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())

			scaleGetCmd := fcmd.NewKraft(stdout, stderr, cfg.Path())
			scaleGetCmd.Env = os.Environ()
			scaleGetCmd.Args = append(scaleGetCmd.Args, "cloud", "scale", "get", serviceNameFull, "-o", "list")
			err = scaleGetCmd.Run()
			time.Sleep(2 * time.Second)
			if err != nil {
				fmt.Print(scaleGetCmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp(`name: ` + serviceNameFull))
			Expect(stdout.String()).To(MatchRegexp(`enabled: true`))
			Expect(stdout.String()).To(MatchRegexp(`min size: 2`))
			Expect(stdout.String()).To(MatchRegexp(`max size: 10`))
			Expect(stdout.String()).To(MatchRegexp(`26000`))
			Expect(stdout.String()).To(MatchRegexp(`26000`))
			Expect(stdout.String()).ToNot(MatchRegexp(policyNameFull1))
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
			Expect(stdout.String()).To(MatchRegexp(`Delete an autoscale configuration policy`))
		})
	})
})
