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
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2" //nolint:stylecheck
	. "github.com/onsi/gomega"    //nolint:stylecheck

	fcmd "kraftkit.sh/test/e2e/framework/cmd"
	fcfg "kraftkit.sh/test/e2e/framework/config"
)

func serviceUUIDParser(stdout *fcmd.IOStream) string {
	if strings.Contains(stdout.String(), "\"uuid\"") {
		uuid := strings.SplitN(stdout.String(), "uuid\":\"", 2)[1]
		uuid = strings.SplitN(uuid, "\"", 2)[0]
		if uuid == "" {
			return ""
		}
		return uuid
	}

	return ""
}

var _ = Describe("kraft cloud scale add", func() {
	var cmd *fcmd.Cmd

	var stdout *fcmd.IOStream
	var stderr *fcmd.IOStream

	var cfg *fcfg.Config

	const (
		imageName      = "nginx:latest"
		instanceName   = "instance-add-test"
		serviceName    = "service-add-test"
		policyName     = "policy-add-test"
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
		cmd.Args = append(cmd.Args, "cloud", "scale", "add", "--log-level", "info", "--log-type", "json")
	})

	When("invoked with a policy inside a configuration", func() {
		var instanceNameFull string
		var serviceNameFull string
		var policyNameFull1 string
		var serviceUUID string

		BeforeEach(func() {
			id, err := rand.Int(rand.Reader, big.NewInt(100000000000))
			if err != nil {
				panic(err)
			}
			instanceNameFull = fmt.Sprintf("%s-%d", instanceName, id)
			serviceNameFull = fmt.Sprintf("%s-%d", serviceName, id)
			policyNameFull1 = fmt.Sprintf("%s-%d-1", policyName, id)

			serviceStdout := fcmd.NewIOStream()
			serviceStderr := fcmd.NewIOStream()
			serviceCreateCmd := fcmd.NewKraft(serviceStdout, serviceStderr, cfg.Path())
			serviceCreateCmd.Args = append(serviceCreateCmd.Args, "cloud", "service", "create",
				"--log-level", "info", "--log-type", "json", "-o", "json",
				"--name", serviceNameFull, "443:8080/tls+http")
			err = serviceCreateCmd.Run()
			if err != nil {
				fmt.Print(serviceCreateCmd.DumpError(serviceStdout, serviceStderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(serviceStderr.String()).To(BeEmpty())
			Expect(serviceStdout.String()).To(MatchRegexp(serviceNameFull))

			// Extract the service UUID
			serviceUUID = serviceUUIDParser(serviceStdout)
			Expect(serviceUUID).ToNot(BeEmpty())

			createStdout := fcmd.NewIOStream()
			createStderr := fcmd.NewIOStream()
			instanceCreateCmd := fcmd.NewKraft(createStdout, createStderr, cfg.Path())
			instanceCreateCmd.Env = os.Environ()
			instanceCreateCmd.Args = append(instanceCreateCmd.Args, "cloud", "instance", "create",
				"-o", "json", "--log-level", "info", "--log-type", "json",
				"--name", instanceNameFull,
				"--memory", instanceMemory,
				"--service", serviceNameFull,
				imageName)
			err = instanceCreateCmd.Run()
			if err != nil {
				fmt.Print(instanceCreateCmd.DumpError(createStdout, createStderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(createStderr.String()).To(BeEmpty())
			Expect(createStdout.String()).ToNot(BeEmpty())

			instanceTemplateCreateStdout := fcmd.NewIOStream()
			instanceTemplateCreateStderr := fcmd.NewIOStream()
			instanceTemplateCreateCmd := fcmd.NewKraft(instanceTemplateCreateStdout, instanceTemplateCreateStderr, cfg.Path())
			instanceTemplateCreateCmd.Env = os.Environ()
			instanceTemplateCreateCmd.Args = append(instanceTemplateCreateCmd.Args, "cloud", "instance", "template", "create",
				"--log-level", "info", "--log-type", "json",
				instanceNameFull)
			err = instanceTemplateCreateCmd.Run()
			if err != nil {
				fmt.Print(instanceTemplateCreateCmd.DumpError(instanceTemplateCreateStdout, instanceTemplateCreateStderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(instanceTemplateCreateStderr.String()).To(BeEmpty())
			Expect(instanceTemplateCreateStdout.String()).ToNot(BeEmpty())

			initStdout := fcmd.NewIOStream()
			initStderr := fcmd.NewIOStream()
			scaleInitCmd := fcmd.NewKraft(initStdout, initStderr, cfg.Path())
			scaleInitCmd.Args = append(scaleInitCmd.Args, "cloud", "scale", "init",
				"--template", instanceNameFull,
				"--min-size", "2",
				"--max-size", "10",
				"--cooldown-time", "13s",
				"--warmup-time", "26s",
				serviceNameFull)
			err = scaleInitCmd.Run()
			if err != nil {
				fmt.Print(scaleInitCmd.DumpError(initStdout, initStderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(initStderr.String()).To(BeEmpty())
			Expect(initStdout.String()).To(BeEmpty())
		})

		AfterEach(func() {
			scaleRemoveStdout := fcmd.NewIOStream()
			scaleRemoveStderr := fcmd.NewIOStream()
			scaleRemoveCmd1 := fcmd.NewKraft(scaleRemoveStdout, scaleRemoveStderr, cfg.Path())
			scaleRemoveCmd1.Env = os.Environ()
			scaleRemoveCmd1.Args = append(scaleRemoveCmd1.Args, "cloud", "scale", "remove", serviceUUID, policyNameFull1)
			err := scaleRemoveCmd1.Run()
			if err != nil {
				fmt.Print(scaleRemoveCmd1.DumpError(scaleRemoveStdout, scaleRemoveStderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			scaleResetStdout := fcmd.NewIOStream()
			scaleResetStderr := fcmd.NewIOStream()
			scaleResetCmd := fcmd.NewKraft(scaleResetStdout, scaleResetStderr, cfg.Path())
			scaleResetCmd.Env = os.Environ()
			scaleResetCmd.Args = append(scaleResetCmd.Args, "cloud", "scale", "reset", serviceNameFull)
			err = scaleResetCmd.Run()
			if err != nil {
				fmt.Print(scaleResetCmd.DumpError(scaleResetStdout, scaleResetStderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			instanceTemplateDeleteStdout := fcmd.NewIOStream()
			instanceTemplateDeleteStderr := fcmd.NewIOStream()
			instanceTemplateDeleteCmd := fcmd.NewKraft(instanceTemplateDeleteStdout, instanceTemplateDeleteStderr, cfg.Path())
			instanceTemplateDeleteCmd.Env = os.Environ()
			instanceTemplateDeleteCmd.Args = append(instanceTemplateDeleteCmd.Args, "cloud", "instance", "template", "delete", instanceNameFull)
			err = instanceTemplateDeleteCmd.Run()
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

		It("should add the policy to the configuration", func() {
			cmd.Args = append(cmd.Args, serviceNameFull, "--name", policyNameFull1, "--step", "0:10/1", "--step", "10:20/2")
			err := cmd.Run()
			time.Sleep(2 * time.Second)
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
			scaleGetCmd.Args = append(scaleGetCmd.Args, "cloud", "scale", "get", serviceNameFull, "-p", policyNameFull1, "-o", "json")
			err = scaleGetCmd.Run()
			time.Sleep(2 * time.Second)
			if err != nil {
				fmt.Print(scaleGetCmd.DumpError(scaleGetStdout, scaleGetStderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(scaleGetStderr.String()).To(BeEmpty())
			Expect(scaleGetStdout.String()).To(MatchRegexp(`"name":"` + policyNameFull1))
			Expect(scaleGetStdout.String()).To(MatchRegexp(`"metric":"cpu"`))
			Expect(scaleGetStdout.String()).To(MatchRegexp(`"adjustment_type":"change"`))
			Expect(scaleGetStdout.String()).To(MatchRegexp(`"adjustment":1`))
			Expect(scaleGetStdout.String()).To(MatchRegexp(`"adjustment":2`))
			Expect(scaleGetStdout.String()).To(MatchRegexp(`"lower_bound":0`))
			Expect(scaleGetStdout.String()).To(MatchRegexp(`"lower_bound":10`))
			Expect(scaleGetStdout.String()).To(MatchRegexp(`"upper_bound":10`))
			Expect(scaleGetStdout.String()).To(MatchRegexp(`"upper_bound":20`))
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
			Expect(stdout.String()).To(MatchRegexp(`Add an autoscale configuration policy for a service`))
		})
	})
})
