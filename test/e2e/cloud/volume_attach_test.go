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

var _ = Describe("kraft cloud volume attach", func() {
	var cmd *fcmd.Cmd

	var stdout *fcmd.IOStream
	var stderr *fcmd.IOStream

	var cfg *fcfg.Config

	const (
		imageName      = "nginx:latest"
		instanceName   = "instance-attach-test"
		volumeName     = "volume-attach-test"
		instanceMemory = "64"
		// TODO: Run one test without a service group also
		instancePortMap = "443:8080"
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
		cmd.Args = append(cmd.Args, "cloud", "volume", "attach", "--log-level", "info", "--log-type", "json", "--at", "/mnt/test/volume")
	})

	When("invoked with json output and a name", func() {
		var volumeNameFull string
		var instanceNameFull string

		BeforeEach(func() {
			id1, err := rand.Int(rand.Reader, big.NewInt(100000000000))
			if err != nil {
				panic(err)
			}
			volumeNameFull = fmt.Sprintf("%s-%d", volumeName, id1)
			instanceNameFull = fmt.Sprintf("%s-%d", instanceName, id1)

			createCmd1 := fcmd.NewKraft(stdout, stderr, cfg.Path())
			createCmd1.Env = os.Environ()
			createCmd1.Args = append(createCmd1.Args, "cloud", "volume", "create", "--log-level", "info", "--log-type", "json", "--size", "1", "--name", volumeNameFull)
			err = createCmd1.Run()
			if err != nil {
				fmt.Print(createCmd1.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())

			createInstanceCmd1 := fcmd.NewKraft(stdout, stderr, cfg.Path())
			createInstanceCmd1.Env = os.Environ()
			createInstanceCmd1.Args = append(createInstanceCmd1.Args, "cloud", "instance", "create", "--log-level", "info", "--log-type", "json", "--port", instancePortMap, "--memory", "64", "--name", instanceNameFull, imageName)

			err = createInstanceCmd1.Run()
			if err != nil {
				fmt.Print(createInstanceCmd1.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())
		})

		AfterEach(func() {
			detachCmd1 := fcmd.NewKraft(stdout, stderr, cfg.Path())
			detachCmd1.Env = os.Environ()
			detachCmd1.Args = append(detachCmd1.Args, "cloud", "volume", "detach", "--log-level", "info", "--log-type", "json", volumeNameFull)
			err := detachCmd1.Run()
			if err != nil {
				fmt.Print(detachCmd1.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())

			instanceRmCmd1 := fcmd.NewKraft(stdout, stderr, cfg.Path())
			instanceRmCmd1.Env = os.Environ()
			instanceRmCmd1.Args = append(instanceRmCmd1.Args, "cloud", "instance", "delete", "--log-level", "info", "--log-type", "json", instanceNameFull)
			err = instanceRmCmd1.Run()
			if err != nil {
				fmt.Print(instanceRmCmd1.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())

			rmCmd1 := fcmd.NewKraft(stdout, stderr, cfg.Path())
			rmCmd1.Env = os.Environ()
			rmCmd1.Args = append(rmCmd1.Args, "cloud", "volume", "delete", "--log-level", "info", "--log-type", "json", volumeNameFull)
			err = rmCmd1.Run()
			if err != nil {
				fmt.Print(rmCmd1.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())
		})

		It("should attach the volume to the instance", func() {
			cmd.Args = append(cmd.Args, volumeNameFull, "--to", instanceNameFull)
			err := cmd.Run()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())

			getCmd1 := fcmd.NewKraft(stdout, stderr, cfg.Path())
			getCmd1.Env = os.Environ()
			getCmd1.Args = append(getCmd1.Args, "cloud", "volume", "get", "--log-level", "info", "--log-type", "json", "-o", "raw", volumeNameFull)
			err = getCmd1.Run()
			if err != nil {
				fmt.Print(getCmd1.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp(`"attached_to":"` + instanceNameFull + "\""))
		})
	})

	When("invoked with json output and a name and the read-only flag", func() {
		var volumeNameFull string
		var instanceNameFull string

		BeforeEach(func() {
			id1, err := rand.Int(rand.Reader, big.NewInt(100000000000))
			if err != nil {
				panic(err)
			}
			volumeNameFull = fmt.Sprintf("%s-%d", volumeName, id1)
			instanceNameFull = fmt.Sprintf("%s-%d", instanceName, id1)

			createCmd1 := fcmd.NewKraft(stdout, stderr, cfg.Path())
			createCmd1.Env = os.Environ()
			createCmd1.Args = append(createCmd1.Args, "cloud", "volume", "create", "--log-level", "info", "--log-type", "json", "--size", "1", "--name", volumeNameFull)
			err = createCmd1.Run()
			if err != nil {
				fmt.Print(createCmd1.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())

			createInstanceCmd1 := fcmd.NewKraft(stdout, stderr, cfg.Path())
			createInstanceCmd1.Env = os.Environ()
			createInstanceCmd1.Args = append(createInstanceCmd1.Args, "cloud", "instance", "create", "--log-level", "info", "--log-type", "json", "--port", instancePortMap, "--memory", "64", "--name", instanceNameFull, imageName)

			err = createInstanceCmd1.Run()
			if err != nil {
				fmt.Print(createInstanceCmd1.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())
		})

		AfterEach(func() {
			detachCmd1 := fcmd.NewKraft(stdout, stderr, cfg.Path())
			detachCmd1.Env = os.Environ()
			detachCmd1.Args = append(detachCmd1.Args, "cloud", "volume", "detach", "--log-level", "info", "--log-type", "json", volumeNameFull)
			err := detachCmd1.Run()
			if err != nil {
				fmt.Print(detachCmd1.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())

			instanceRmCmd1 := fcmd.NewKraft(stdout, stderr, cfg.Path())
			instanceRmCmd1.Env = os.Environ()
			instanceRmCmd1.Args = append(instanceRmCmd1.Args, "cloud", "instance", "delete", "--log-level", "info", "--log-type", "json", instanceNameFull)
			err = instanceRmCmd1.Run()
			if err != nil {
				fmt.Print(instanceRmCmd1.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())

			rmCmd1 := fcmd.NewKraft(stdout, stderr, cfg.Path())
			rmCmd1.Env = os.Environ()
			rmCmd1.Args = append(rmCmd1.Args, "cloud", "volume", "delete", "--log-level", "info", "--log-type", "json", volumeNameFull)
			err = rmCmd1.Run()
			if err != nil {
				fmt.Print(rmCmd1.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())
		})

		It("should attach the volume to the instance in read only mode", func() {
			cmd.Args = append(cmd.Args, volumeNameFull, "-r", "--to", instanceNameFull)
			err := cmd.Run()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())

			getCmd1 := fcmd.NewKraft(stdout, stderr, cfg.Path())
			getCmd1.Env = os.Environ()
			getCmd1.Args = append(getCmd1.Args, "cloud", "volume", "get", "--log-level", "info", "--log-type", "json", "-o", "json", volumeNameFull)
			err = getCmd1.Run()
			if err != nil {
				fmt.Print(getCmd1.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp(`"attached_to":"` + instanceNameFull + "\""))
			Expect(stdout.String()).To(MatchRegexp(`"read_only":true"`))
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
			Expect(stdout.String()).To(MatchRegexp(`Attach a persistent volume to an instance`))
		})
	})
})
