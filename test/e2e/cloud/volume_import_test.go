// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package cloud_test

import (
	"crypto/rand"
	"fmt"
	"io"
	"math/big"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2" //nolint:stylecheck
	. "github.com/onsi/gomega"    //nolint:stylecheck

	fcmd "kraftkit.sh/test/e2e/framework/cmd"
	fcfg "kraftkit.sh/test/e2e/framework/config"
)

var _ = Describe("kraft cloud volume import", func() {
	var cmd *fcmd.Cmd

	var stdout *fcmd.IOStream
	var stderr *fcmd.IOStream

	var cfg *fcfg.Config

	const (
		imageName      = "nginx:latest"
		instanceName   = "instance-import-test"
		volumeName     = "volume-import-test"
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
		cmd.Args = append(cmd.Args, "cloud", "volume", "import", "--log-level", "info", "--log-type", "json")
	})

	When("invoked with a volume name and a Dockerfile source", func() {
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
			createCmd1.Args = append(createCmd1.Args, "cloud", "volume", "create",
				"--log-level", "info", "--log-type", "json",
				"--size", "8", "--name", volumeNameFull)
			err = createCmd1.Run()
			if err != nil {
				fmt.Print(createCmd1.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())

			createInstanceCmd1 := fcmd.NewKraft(stdout, stderr, cfg.Path())
			createInstanceCmd1.Env = os.Environ()
			createInstanceCmd1.Args = append(createInstanceCmd1.Args, "cloud", "instance", "create",
				"--log-level", "info", "--log-type", "json", "-o", "json",
				"--port", instancePortMap, "--memory", "64",
				"--name", instanceNameFull, imageName)

			err = createInstanceCmd1.Run()
			if err != nil {
				fmt.Print(createInstanceCmd1.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())
		})

		AfterEach(func() {
			stopCmd1 := fcmd.NewKraft(stdout, stderr, cfg.Path())
			stopCmd1.Env = os.Environ()
			stopCmd1.Args = append(stopCmd1.Args, "cloud", "instance", "stop",
				"--log-level", "info", "--log-type", "json",
				instanceNameFull)
			err := stopCmd1.Run()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())

			detachCmd1 := fcmd.NewKraft(stdout, stderr, cfg.Path())
			detachCmd1.Env = os.Environ()
			detachCmd1.Args = append(detachCmd1.Args, "cloud", "volume", "detach",
				"--log-level", "info", "--log-type", "json",
				volumeNameFull)
			err = detachCmd1.Run()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())

			getCmd1 := fcmd.NewKraft(stdout, stderr, cfg.Path())
			getCmd1.Env = os.Environ()
			getCmd1.Args = append(getCmd1.Args, "cloud", "volume", "get",
				"--log-level", "info", "--log-type", "json",
				"-o", "raw", volumeNameFull)
			err = getCmd1.Run()
			if err != nil {
				fmt.Print(getCmd1.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(MatchRegexp(`"attached_to":"` + instanceNameFull + "\""))

			instanceRmCmd1 := fcmd.NewKraft(stdout, stderr, cfg.Path())
			instanceRmCmd1.Env = os.Environ()
			instanceRmCmd1.Args = append(instanceRmCmd1.Args, "cloud", "instance", "delete",
				"--log-level", "info", "--log-type", "json",
				instanceNameFull)
			err = instanceRmCmd1.Run()
			if err != nil {
				fmt.Print(instanceRmCmd1.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())

			rmCmd1 := fcmd.NewKraft(stdout, stderr, cfg.Path())
			rmCmd1.Env = os.Environ()
			rmCmd1.Args = append(rmCmd1.Args, "cloud", "volume", "delete",
				"--log-level", "info", "--log-type", "json",
				volumeNameFull)
			err = rmCmd1.Run()
			if err != nil {
				fmt.Print(rmCmd1.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())
		})

		It("should import files to the volume from the Dockerfile", func() {
			cmd.Args = append(cmd.Args, "-v", volumeNameFull, "-s", "fixtures/import/Dockerfile")
			time.Sleep(time.Second)
			err := cmd.Run()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())

			attachInstanceCmd1 := fcmd.NewKraft(stdout, stderr, cfg.Path())
			attachInstanceCmd1.Env = os.Environ()
			attachInstanceCmd1.Args = append(attachInstanceCmd1.Args, "cloud", "volume", "attach",
				"--log-level", "info", "--log-type", "json",
				volumeNameFull, "--at", "/wwwroot", "--to", instanceNameFull)
			err = attachInstanceCmd1.Run()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())

			startCmd1 := fcmd.NewKraft(stdout, stderr, cfg.Path())
			startCmd1.Env = os.Environ()
			startCmd1.Args = append(startCmd1.Args, "cloud", "instance", "start",
				"--log-level", "info", "--log-type", "json",
				instanceNameFull)
			err = startCmd1.Run()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())

			url := urlParser(stdout)
			Expect(url).ToNot(BeEmpty())

			curlCmd := fcmd.NewCurl(stdout, stderr)
			curlCmd.Args = append(curlCmd.Args, url)

			err = curlCmd.Run()
			time.Sleep(time.Second)
			if err != nil {
				fmt.Print(curlCmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp(`Hello World!`))
		})
	})

	When("invoked with a volume name and a directory source", func() {
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
			createCmd1.Args = append(createCmd1.Args, "cloud", "volume", "create",
				"--log-level", "info", "--log-type", "json",
				"--size", "8", "--name", volumeNameFull)
			err = createCmd1.Run()
			if err != nil {
				fmt.Print(createCmd1.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())

			createInstanceCmd1 := fcmd.NewKraft(stdout, stderr, cfg.Path())
			createInstanceCmd1.Env = os.Environ()
			createInstanceCmd1.Args = append(createInstanceCmd1.Args, "cloud", "instance", "create",
				"--log-level", "info", "--log-type", "json", "-o", "json",
				"--port", instancePortMap, "--memory", "64",
				"--name", instanceNameFull, imageName)

			err = createInstanceCmd1.Run()
			if err != nil {
				fmt.Print(createInstanceCmd1.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())
		})

		AfterEach(func() {
			stopCmd1 := fcmd.NewKraft(stdout, stderr, cfg.Path())
			stopCmd1.Env = os.Environ()
			stopCmd1.Args = append(stopCmd1.Args, "cloud", "instance", "stop",
				"--log-level", "info", "--log-type", "json",
				instanceNameFull)
			err := stopCmd1.Run()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())

			detachCmd1 := fcmd.NewKraft(stdout, stderr, cfg.Path())
			detachCmd1.Env = os.Environ()
			detachCmd1.Args = append(detachCmd1.Args, "cloud", "volume", "detach",
				"--log-level", "info", "--log-type", "json",
				volumeNameFull)
			err = detachCmd1.Run()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())

			getCmd1 := fcmd.NewKraft(stdout, stderr, cfg.Path())
			getCmd1.Env = os.Environ()
			getCmd1.Args = append(getCmd1.Args, "cloud", "volume", "get",
				"--log-level", "info", "--log-type", "json",
				"-o", "raw", volumeNameFull)
			err = getCmd1.Run()
			if err != nil {
				fmt.Print(getCmd1.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(MatchRegexp(`"attached_to":"` + instanceNameFull + "\""))

			instanceRmCmd1 := fcmd.NewKraft(stdout, stderr, cfg.Path())
			instanceRmCmd1.Env = os.Environ()
			instanceRmCmd1.Args = append(instanceRmCmd1.Args, "cloud", "instance", "delete",
				"--log-level", "info", "--log-type", "json",
				instanceNameFull)
			err = instanceRmCmd1.Run()
			if err != nil {
				fmt.Print(instanceRmCmd1.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())

			rmCmd1 := fcmd.NewKraft(stdout, stderr, cfg.Path())
			rmCmd1.Env = os.Environ()
			rmCmd1.Args = append(rmCmd1.Args, "cloud", "volume", "delete",
				"--log-level", "info", "--log-type", "json",
				volumeNameFull)
			err = rmCmd1.Run()
			if err != nil {
				fmt.Print(rmCmd1.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())
		})

		It("should import files to the volume from the cpio file", func() {
			cmd.Args = append(cmd.Args, "-v", volumeNameFull, "-s", "fixtures/import/cpio")
			err := cmd.Run()
			time.Sleep(time.Second)
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())

			attachInstanceCmd1 := fcmd.NewKraft(stdout, stderr, cfg.Path())
			attachInstanceCmd1.Env = os.Environ()
			attachInstanceCmd1.Args = append(attachInstanceCmd1.Args, "cloud", "volume", "attach",
				"--log-level", "info", "--log-type", "json",
				volumeNameFull, "--at", "/wwwroot", "--to", instanceNameFull)
			err = attachInstanceCmd1.Run()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())

			startCmd1 := fcmd.NewKraft(stdout, stderr, cfg.Path())
			startCmd1.Env = os.Environ()
			startCmd1.Args = append(startCmd1.Args, "cloud", "instance", "start",
				"--log-level", "info", "--log-type", "json",
				instanceNameFull)
			err = startCmd1.Run()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())

			url := urlParser(stdout)
			Expect(url).ToNot(BeEmpty())

			curlCmd := fcmd.NewCurl(stdout, stderr)
			curlCmd.Args = append(curlCmd.Args, url)

			err = curlCmd.Run()
			time.Sleep(time.Second)
			if err != nil {
				fmt.Print(curlCmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp(`Hello World!`))
		})
	})

	When("invoked with a volume name and a directory source and the '--force' flag", func() {
		var volumeNameFull string
		var content string

		BeforeEach(func() {
			id1, err := rand.Int(rand.Reader, big.NewInt(100000000000))
			if err != nil {
				panic(err)
			}
			volumeNameFull = fmt.Sprintf("%s-%d", volumeName, id1)

			createCmd1 := fcmd.NewKraft(stdout, stderr, cfg.Path())
			createCmd1.Env = os.Environ()
			createCmd1.Args = append(createCmd1.Args, "cloud", "volume", "create",
				"--log-level", "info", "--log-type", "json",
				"--size", "9", "--name", volumeNameFull)
			err = createCmd1.Run()
			if err != nil {
				fmt.Print(createCmd1.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())

			file, err := os.OpenFile("fixtures/import/cpio-large/index.html", os.O_RDWR, 0o644)
			if err != nil {
				panic(err)
			}

			buf := make([]byte, 20)
			_, err = file.Read(buf)
			if err != nil {
				panic(err)
			}

			content = string(buf)
			for i := 0; i < 50000; i++ {
				_, err = io.WriteString(file, content)
				if err != nil {
					panic(err)
				}
			}

			err = file.Close()
			if err != nil {
				panic(err)
			}
		})

		AfterEach(func() {
			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())

			rmCmd1 := fcmd.NewKraft(stdout, stderr, cfg.Path())
			rmCmd1.Env = os.Environ()
			rmCmd1.Args = append(rmCmd1.Args, "cloud", "volume", "delete",
				"--log-level", "info", "--log-type", "json",
				volumeNameFull)
			err := rmCmd1.Run()
			if err != nil {
				fmt.Print(rmCmd1.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())

			file, err := os.OpenFile("fixtures/import/cpio-large/index.html", os.O_WRONLY, 0o644)
			if err != nil {
				panic(err)
			}

			err = file.Truncate(0)
			if err != nil {
				panic(err)
			}

			_, err = file.WriteString(content)
			if err != nil {
				panic(err)
			}

			err = file.Close()
			if err != nil {
				panic(err)
			}
		})

		It("should import files to the volume from the cpio file regardless of size", func() {
			cmd.Args = append(cmd.Args, "-v", volumeNameFull, "-s", "fixtures/import/cpio-large", "--force")
			err := cmd.Run()
			time.Sleep(time.Second)
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())
		})
	})

	When("invoked with a volume name and a url source", func() {
		var volumeNameFull string

		BeforeEach(func() {
			id1, err := rand.Int(rand.Reader, big.NewInt(100000000000))
			if err != nil {
				panic(err)
			}
			volumeNameFull = fmt.Sprintf("%s-%d", volumeName, id1)

			createCmd1 := fcmd.NewKraft(stdout, stderr, cfg.Path())
			createCmd1.Env = os.Environ()
			createCmd1.Args = append(createCmd1.Args, "cloud", "volume", "create",
				"--log-level", "info", "--log-type", "json",
				"--size", "128", "--name", volumeNameFull)
			err = createCmd1.Run()
			if err != nil {
				fmt.Print(createCmd1.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())
		})

		AfterEach(func() {
			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())

			rmCmd1 := fcmd.NewKraft(stdout, stderr, cfg.Path())
			rmCmd1.Env = os.Environ()
			rmCmd1.Args = append(rmCmd1.Args, "cloud", "volume", "delete",
				"--log-level", "info", "--log-type", "json",
				volumeNameFull)
			err := rmCmd1.Run()
			if err != nil {
				fmt.Print(rmCmd1.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())
		})

		It("should import files to the volume from the docker link", func() {
			cmd.Args = append(cmd.Args, "-v", volumeNameFull, "-s", "ubuntu:noble-20240827.1")
			err := cmd.Run()
			time.Sleep(time.Second)
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

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
			Expect(stdout.String()).To(MatchRegexp(`Import local data to a persistent volume`))
		})
	})
})
