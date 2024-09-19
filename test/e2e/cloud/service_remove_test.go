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

var _ = Describe("kraft cloud service remove", func() {
	var cmd *fcmd.Cmd

	var stdout *fcmd.IOStream
	var stderr *fcmd.IOStream

	var cfg *fcfg.Config

	const (
		imageName      = "nginx:latest"
		instanceName   = "instance-remove-test"
		serviceName    = "service-remove-test"
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
		cmd.Args = append(cmd.Args, "cloud", "service", "delete", "--log-level", "info", "--log-type", "json")
	})

	When("invoked with a single empty service", func() {
		var serviceNameFull string

		BeforeEach(func() {
			id, err := rand.Int(rand.Reader, big.NewInt(100000000000))
			if err != nil {
				panic(err)
			}
			serviceNameFull = fmt.Sprintf("%s-%d", serviceName, id)

			serviceCreateCmd := fcmd.NewKraft(stdout, stderr, cfg.Path())
			serviceCreateCmd.Env = os.Environ()
			serviceCreateCmd.Args = append(serviceCreateCmd.Args, "cloud", "service", "create",
				"--log-level", "info", "--log-type", "json", "-o", "json",
				"--name", serviceNameFull,
				"443:8080/tls+http")

			err = serviceCreateCmd.Run()
			if err != nil {
				fmt.Print(serviceCreateCmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())
		})

		It("should remove the service", func() {
			cmd.Args = append(cmd.Args, serviceNameFull)
			err := cmd.Run()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())

			getCmd := fcmd.NewKraft(stdout, stderr, cfg.Path())
			getCmd.Env = os.Environ()
			getCmd.Args = append(getCmd.Args, "cloud", "service", "get", "-o", "json", serviceNameFull)

			err = getCmd.Run()
			Expect(err).To(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())
		})
	})

	When("invoked with two empty services", func() {
		var serviceNameFull1 string
		var serviceNameFull2 string

		BeforeEach(func() {
			id, err := rand.Int(rand.Reader, big.NewInt(100000000000))
			if err != nil {
				panic(err)
			}
			serviceNameFull1 = fmt.Sprintf("%s-%d-1", serviceName, id)
			serviceNameFull2 = fmt.Sprintf("%s-%d-2", serviceName, id)

			serviceCreateCmd1 := fcmd.NewKraft(stdout, stderr, cfg.Path())
			serviceCreateCmd1.Env = os.Environ()
			serviceCreateCmd1.Args = append(serviceCreateCmd1.Args, "cloud", "service", "create",
				"--log-level", "info", "--log-type", "json", "-o", "json",
				"--name", serviceNameFull1,
				"443:8080/tls+http")

			err = serviceCreateCmd1.Run()
			if err != nil {
				fmt.Print(serviceCreateCmd1.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())

			serviceCreateCmd2 := fcmd.NewKraft(stdout, stderr, cfg.Path())
			serviceCreateCmd2.Env = os.Environ()
			serviceCreateCmd2.Args = append(serviceCreateCmd2.Args, "cloud", "service", "create",
				"--log-level", "info", "--log-type", "json", "-o", "json",
				"--name", serviceNameFull2,
				"443:8080/tls+http")

			err = serviceCreateCmd2.Run()
			if err != nil {
				fmt.Print(serviceCreateCmd2.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())
		})

		It("should remove the services", func() {
			cmd.Args = append(cmd.Args, serviceNameFull1, serviceNameFull2)
			err := cmd.Run()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())

			getCmd := fcmd.NewKraft(stdout, stderr, cfg.Path())
			getCmd.Env = os.Environ()
			getCmd.Args = append(getCmd.Args, "cloud", "service", "get", "-o", "json", serviceNameFull1, serviceNameFull2)

			err = getCmd.Run()
			Expect(err).To(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())
		})
	})

	When("invoked with a single service with instances attached", func() {
		var instanceNameFull string
		var serviceNameFull string

		BeforeEach(func() {
			id, err := rand.Int(rand.Reader, big.NewInt(100000000000))
			if err != nil {
				panic(err)
			}
			instanceNameFull = fmt.Sprintf("%s-%d", instanceName, id)
			serviceNameFull = fmt.Sprintf("%s-%d", serviceName, id)

			serviceCreateCmd := fcmd.NewKraft(stdout, stderr, cfg.Path())
			serviceCreateCmd.Env = os.Environ()
			serviceCreateCmd.Args = append(serviceCreateCmd.Args, "cloud", "service", "create",
				"--log-level", "info", "--log-type", "json", "-o", "json",
				"--name", serviceNameFull,
				"443:8080/tls+http")

			err = serviceCreateCmd.Run()
			if err != nil {
				fmt.Print(serviceCreateCmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())

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
		})

		AfterEach(func() {
			instanceDeleteCmd := fcmd.NewKraft(stdout, stderr, cfg.Path())
			instanceDeleteCmd.Env = os.Environ()
			instanceDeleteCmd.Args = append(instanceDeleteCmd.Args, "cloud", "instance", "delete", instanceNameFull)
			err := instanceDeleteCmd.Run()
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

		It("should error out when removing", func() {
			cmd.Args = append(cmd.Args, serviceNameFull)
			err := cmd.Run()
			Expect(err).To(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())

			getCmd := fcmd.NewKraft(stdout, stderr, cfg.Path())
			getCmd.Env = os.Environ()
			getCmd.Args = append(getCmd.Args, "cloud", "service", "get", "-o", "json", serviceNameFull)

			err = getCmd.Run()
			if err != nil {
				fmt.Print(getCmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())
		})
	})

	When("invoked with no arguments and the '--all' flag", Serial, func() {
		var instanceNameFull string
		var serviceNameFull1 string
		var serviceNameFull2 string

		BeforeEach(func() {
			id, err := rand.Int(rand.Reader, big.NewInt(100000000000))
			if err != nil {
				panic(err)
			}
			instanceNameFull = fmt.Sprintf("%s-%d", instanceName, id)
			serviceNameFull1 = fmt.Sprintf("%s-%d-1", serviceName, id)
			serviceNameFull2 = fmt.Sprintf("%s-%d-2", serviceName, id)

			serviceCreateCmd1 := fcmd.NewKraft(stdout, stderr, cfg.Path())
			serviceCreateCmd1.Env = os.Environ()
			serviceCreateCmd1.Args = append(serviceCreateCmd1.Args, "cloud", "service", "create",
				"--log-level", "info", "--log-type", "json", "-o", "json",
				"--name", serviceNameFull1,
				"443:8080/tls+http")

			err = serviceCreateCmd1.Run()
			if err != nil {
				fmt.Print(serviceCreateCmd1.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())

			serviceCreateCmd2 := fcmd.NewKraft(stdout, stderr, cfg.Path())
			serviceCreateCmd2.Env = os.Environ()
			serviceCreateCmd2.Args = append(serviceCreateCmd2.Args, "cloud", "service", "create",
				"--log-level", "info", "--log-type", "json", "-o", "json",
				"--name", serviceNameFull2,
				"443:8080/tls+http")

			err = serviceCreateCmd2.Run()
			if err != nil {
				fmt.Print(serviceCreateCmd2.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())

			instanceCreateCmd := fcmd.NewKraft(stdout, stderr, cfg.Path())
			instanceCreateCmd.Env = os.Environ()
			instanceCreateCmd.Args = append(instanceCreateCmd.Args, "cloud", "instance", "create",
				"-o", "json", "--log-level", "info", "--log-type", "json",
				"--name", instanceNameFull,
				"--memory", instanceMemory,
				"--start",
				"--service", serviceNameFull1,
				imageName)
			err = instanceCreateCmd.Run()
			if err != nil {
				fmt.Print(instanceCreateCmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())
		})

		AfterEach(func() {
			instanceDeleteCmd := fcmd.NewKraft(stdout, stderr, cfg.Path())
			instanceDeleteCmd.Env = os.Environ()
			instanceDeleteCmd.Args = append(instanceDeleteCmd.Args, "cloud", "instance", "delete", instanceNameFull)
			err := instanceDeleteCmd.Run()
			time.Sleep(2 * time.Second)
			if err != nil {
				fmt.Print(instanceDeleteCmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			serviceDeleteCmd := fcmd.NewKraft(stdout, stderr, cfg.Path())
			serviceDeleteCmd.Env = os.Environ()
			serviceDeleteCmd.Args = append(serviceDeleteCmd.Args, "cloud", "service", "delete", serviceNameFull1)
			err = serviceDeleteCmd.Run()
			time.Sleep(2 * time.Second)
			if err != nil {
				fmt.Print(serviceDeleteCmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())
		})

		It("should remove all created services and ignore attached ones", func() {
			cmd.Args = append(cmd.Args, "--all")
			err := cmd.Run()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())

			lsCmd := fcmd.NewKraft(stdout, stderr, cfg.Path())
			lsCmd.Env = os.Environ()
			lsCmd.Args = append(lsCmd.Args, "cloud", "service", "list", "-o", "json")

			err = lsCmd.Run()
			if err != nil {
				fmt.Print(lsCmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp(`"name":"` + serviceNameFull1 + `"`))
			Expect(stdout.String()).To(MatchRegexp(`ignoring 1 service`))
		})
	})

	When("invoked with a single service and the '--wait-empty' flag", func() {
		var instanceNameFull string
		var serviceNameFull string

		BeforeEach(func() {
			id, err := rand.Int(rand.Reader, big.NewInt(100000000000))
			if err != nil {
				panic(err)
			}
			instanceNameFull = fmt.Sprintf("%s-%d", instanceName, id)
			serviceNameFull = fmt.Sprintf("%s-%d", serviceName, id)

			serviceCreateCmd := fcmd.NewKraft(stdout, stderr, cfg.Path())
			serviceCreateCmd.Env = os.Environ()
			serviceCreateCmd.Args = append(serviceCreateCmd.Args, "cloud", "service", "create",
				"--log-level", "info", "--log-type", "json", "-o", "json",
				"--name", serviceNameFull,
				"443:8080/tls+http")

			err = serviceCreateCmd.Run()
			if err != nil {
				fmt.Print(serviceCreateCmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())

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

			instanceDeleteCmd := fcmd.NewKraft(stdout, stderr, cfg.Path())
			instanceDeleteCmd.Env = os.Environ()
			instanceDeleteCmd.Args = append(instanceDeleteCmd.Args, "cloud", "instance", "delete", instanceNameFull)
			err = instanceDeleteCmd.Run()
			if err != nil {
				fmt.Print(instanceDeleteCmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())
		})

		It("remove the service after no instances are attached anymore", func() {
			cmd.Args = append(cmd.Args, "--wait-empty", serviceNameFull)
			err := cmd.Run()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())

			getCmd := fcmd.NewKraft(stdout, stderr, cfg.Path())
			getCmd.Env = os.Environ()
			getCmd.Args = append(getCmd.Args, "cloud", "service", "get", "-o", "json", serviceNameFull)

			err = getCmd.Run()
			Expect(err).To(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())
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
			Expect(stdout.String()).To(MatchRegexp(`Delete services`))
		})
	})
})
