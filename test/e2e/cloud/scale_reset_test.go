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

var _ = Describe("kraft cloud scale reset", func() {
	var cmd *fcmd.Cmd

	var stdout *fcmd.IOStream
	var stderr *fcmd.IOStream

	var cfg *fcfg.Config

	const (
		imageName      = "nginx:latest"
		instanceName   = "instance-reset-test"
		serviceName    = "service-reset-test"
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
		cmd.Args = append(cmd.Args, "cloud", "scale", "reset", "--log-level", "info", "--log-type", "json")
	})

	When("invoked with a configured autoscale configuration", func() {
		var instanceNameFull string
		var serviceNameFull string

		BeforeEach(func() {
			id, err := rand.Int(rand.Reader, big.NewInt(100000000000))
			if err != nil {
				panic(err)
			}
			instanceNameFull = fmt.Sprintf("%s-%d", instanceName, id)
			serviceNameFull = fmt.Sprintf("%s-%d", serviceName, id)

			serviceCreateStdout := fcmd.NewIOStream()
			serviceCreateStderr := fcmd.NewIOStream()
			serviceCreateCmd := fcmd.NewKraft(serviceCreateStdout, serviceCreateStderr, cfg.Path())
			serviceCreateCmd.Args = append(serviceCreateCmd.Args, "cloud", "service", "create",
				"--log-level", "info", "--log-type", "json",
				"--name", serviceNameFull, "443:8080/tls+http")
			err = serviceCreateCmd.Run()
			if err != nil {
				fmt.Print(serviceCreateCmd.DumpError(serviceCreateStdout, serviceCreateStderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(serviceCreateStderr.String()).To(BeEmpty())
			Expect(serviceCreateStdout.String()).To(MatchRegexp(serviceNameFull))

			instanceCreateStdout := fcmd.NewIOStream()
			instanceCreateStderr := fcmd.NewIOStream()
			instanceCreateCmd := fcmd.NewKraft(instanceCreateStdout, instanceCreateStderr, cfg.Path())
			instanceCreateCmd.Env = os.Environ()
			instanceCreateCmd.Args = append(instanceCreateCmd.Args, "cloud", "instance", "create",
				"-o", "json", "--log-level", "info", "--log-type", "json",
				"--name", instanceNameFull,
				"--memory", instanceMemory,
				"--service", serviceNameFull,
				imageName)
			err = instanceCreateCmd.Run()
			if err != nil {
				fmt.Print(instanceCreateCmd.DumpError(instanceCreateStdout, instanceCreateStderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(instanceCreateStderr.String()).To(BeEmpty())
			Expect(instanceCreateStdout.String()).ToNot(BeEmpty())

			instanceTemplateCreateStdout := fcmd.NewIOStream()
			instanceTemplateCreateStderr := fcmd.NewIOStream()
			instanceTemplateCreateCmd := fcmd.NewKraft(instanceTemplateCreateStdout, instanceTemplateCreateStderr, cfg.Path())
			instanceTemplateCreateCmd.Env = os.Environ()
			instanceTemplateCreateCmd.Args = append(instanceTemplateCreateCmd.Args, "cloud", "instance", "template", "create",
				"--log-level", "info", "--log-type", "json",
				instanceNameFull)
			err = instanceTemplateCreateCmd.Run()
			time.Sleep(2 * time.Second)
			if err != nil {
				fmt.Print(instanceTemplateCreateCmd.DumpError(instanceTemplateCreateStdout, instanceTemplateCreateStderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(instanceTemplateCreateStderr.String()).To(BeEmpty())
			Expect(instanceTemplateCreateStdout.String()).ToNot(BeEmpty())

			scaleInitStdout := fcmd.NewIOStream()
			scaleInitStderr := fcmd.NewIOStream()
			scaleInitCmd := fcmd.NewKraft(scaleInitStdout, scaleInitStderr, cfg.Path())
			scaleInitCmd.Env = os.Environ()
			scaleInitCmd.Args = append(scaleInitCmd.Args, "cloud", "scale", "init",
				"--template", instanceNameFull,
				"--min-size", "2",
				"--max-size", "10",
				"--cooldown-time", "13s",
				"--warmup-time", "26s",
				serviceNameFull)
			err = scaleInitCmd.Run()
			time.Sleep(2 * time.Second)
			if err != nil {
				fmt.Print(scaleInitCmd.DumpError(scaleInitStdout, scaleInitStderr, err))
			}
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			instanceTemplateDeleteStdout := fcmd.NewIOStream()
			instanceTemplateDeleteStderr := fcmd.NewIOStream()
			instanceTemplateDeleteCmd := fcmd.NewKraft(instanceTemplateDeleteStdout, instanceTemplateDeleteStderr, cfg.Path())
			instanceTemplateDeleteCmd.Env = os.Environ()
			instanceTemplateDeleteCmd.Args = append(instanceTemplateDeleteCmd.Args, "cloud", "instance", "template", "delete", instanceNameFull)
			err := instanceTemplateDeleteCmd.Run()
			time.Sleep(2 * time.Second)
			if err != nil {
				fmt.Print(instanceTemplateDeleteCmd.DumpError(instanceTemplateDeleteStdout, instanceTemplateDeleteStderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			serviceDeleteStdout := fcmd.NewIOStream()
			serviceDeleteStderr := fcmd.NewIOStream()
			serviceDeleteCmd := fcmd.NewKraft(serviceDeleteStdout, serviceDeleteStderr, cfg.Path())
			serviceDeleteCmd.Env = os.Environ()
			serviceDeleteCmd.Args = append(serviceDeleteCmd.Args, "cloud", "service", "delete", serviceNameFull)
			err = serviceDeleteCmd.Run()
			time.Sleep(2 * time.Second)
			if err != nil {
				fmt.Print(serviceDeleteCmd.DumpError(serviceDeleteStdout, serviceDeleteStderr, err))
			}
			Expect(err).ToNot(HaveOccurred())
		})

		It("should reset an autoscale configuration", func() {
			cmd.Args = append(cmd.Args, serviceNameFull)
			err := cmd.Run()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(BeEmpty())

			scaleGetStdout := fcmd.NewIOStream()
			scaleGetStderr := fcmd.NewIOStream()
			scaleGetCmd := fcmd.NewKraft(scaleGetStdout, scaleGetStderr, cfg.Path())
			scaleGetCmd.Env = os.Environ()
			scaleGetCmd.Args = append(scaleGetCmd.Args, "cloud", "scale", "get", serviceNameFull, "-o", "list")
			err = scaleGetCmd.Run()
			time.Sleep(2 * time.Second)
			if err != nil {
				fmt.Print(scaleGetCmd.DumpError(scaleGetStdout, scaleGetStderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(scaleGetStderr.String()).To(BeEmpty())
			Expect(scaleGetStdout.String()).To(MatchRegexp(`name: ` + serviceNameFull))
			Expect(scaleGetStdout.String()).To(MatchRegexp(`enabled: false`))
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
			Expect(stdout.String()).To(MatchRegexp(`Reset autoscale configuration of a service`))
		})
	})
})
