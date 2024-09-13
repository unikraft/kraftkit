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

var _ = Describe("kraft cloud volume create", func() {
	var cmd *fcmd.Cmd

	var stdout *fcmd.IOStream
	var stderr *fcmd.IOStream

	var cfg *fcfg.Config

	const (
		volumeName = "volume-delete-test"
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
		cmd.Args = append(cmd.Args, "cloud", "volume", "delete", "--log-level", "info", "--log-type", "json")
	})

	When("invoked with an UUID", func() {
		var volumeUUID string

		BeforeEach(func() {
			createCmd := fcmd.NewKraft(stdout, stderr, cfg.Path())
			createCmd.Env = os.Environ()
			createCmd.Args = append(createCmd.Args, "cloud", "volume", "create", "--log-level", "info", "--log-type", "json", "--size", "1")
			err := createCmd.Run()
			if err != nil {
				fmt.Print(createCmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())

			volumeUUID = stdout.String()
			volumeUUID = volumeUUID[:len(volumeUUID)-1]
		})

		It("should delete the instance", func() {
			cmd.Args = append(cmd.Args, volumeUUID)
			err := cmd.Run()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())
		})
	})

	When("invoked with a name", func() {
		var volumeNameFull string

		BeforeEach(func() {
			id, err := rand.Int(rand.Reader, big.NewInt(100000000000))
			if err != nil {
				panic(err)
			}
			volumeNameFull = fmt.Sprintf("%s-%d", volumeName, id)

			createCmd := fcmd.NewKraft(stdout, stderr, cfg.Path())
			createCmd.Env = os.Environ()
			createCmd.Args = append(createCmd.Args, "cloud", "volume", "create", "--log-level", "info", "--log-type", "json", "--size", "1", "--name", volumeNameFull)
			err = createCmd.Run()
			if err != nil {
				fmt.Print(createCmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())
		})

		It("should delete the instance", func() {
			cmd.Args = append(cmd.Args, volumeNameFull)
			err := cmd.Run()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())
		})
	})

	When("invoked with two names", func() {
		var volumeNameFull1 string
		var volumeNameFull2 string

		BeforeEach(func() {
			id1, err := rand.Int(rand.Reader, big.NewInt(100000000000))
			if err != nil {
				panic(err)
			}
			volumeNameFull1 = fmt.Sprintf("%s-%d", volumeName, id1)

			id2, err := rand.Int(rand.Reader, big.NewInt(100000000000))
			if err != nil {
				panic(err)
			}
			volumeNameFull2 = fmt.Sprintf("%s-%d", volumeName, id2)

			createCmd1 := fcmd.NewKraft(stdout, stderr, cfg.Path())
			createCmd1.Env = os.Environ()
			createCmd1.Args = append(createCmd1.Args, "cloud", "volume", "create", "--log-level", "info", "--log-type", "json", "--size", "1", "--name", volumeNameFull1)
			err = createCmd1.Run()
			if err != nil {
				fmt.Print(createCmd1.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())

			createCmd2 := fcmd.NewKraft(stdout, stderr, cfg.Path())
			createCmd2.Env = os.Environ()
			createCmd2.Args = append(createCmd2.Args, "cloud", "volume", "create", "--log-level", "info", "--log-type", "json", "--size", "1", "--name", volumeNameFull2)
			err = createCmd2.Run()
			if err != nil {
				fmt.Print(createCmd2.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())
		})

		It("should delete the instance", func() {
			cmd.Args = append(cmd.Args, volumeNameFull1, volumeNameFull2)
			err := cmd.Run()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())
		})
	})

	When("invoked with the '--all' flag", Serial, func() {
		var volumeNameFull1 string
		var volumeNameFull2 string

		BeforeEach(func() {
			id1, err := rand.Int(rand.Reader, big.NewInt(100000000000))
			if err != nil {
				panic(err)
			}
			volumeNameFull1 = fmt.Sprintf("%s-%d", volumeName, id1)

			id2, err := rand.Int(rand.Reader, big.NewInt(100000000000))
			if err != nil {
				panic(err)
			}
			volumeNameFull2 = fmt.Sprintf("%s-%d", volumeName, id2)

			createCmd1 := fcmd.NewKraft(stdout, stderr, cfg.Path())
			createCmd1.Env = os.Environ()
			createCmd1.Args = append(createCmd1.Args, "cloud", "volume", "create", "--log-level", "info", "--log-type", "json", "--size", "1", "--name", volumeNameFull1)
			err = createCmd1.Run()
			if err != nil {
				fmt.Print(createCmd1.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())

			createCmd2 := fcmd.NewKraft(stdout, stderr, cfg.Path())
			createCmd2.Env = os.Environ()
			createCmd2.Args = append(createCmd2.Args, "cloud", "volume", "create", "--log-level", "info", "--log-type", "json", "--size", "1", "--name", volumeNameFull2)
			err = createCmd2.Run()
			if err != nil {
				fmt.Print(createCmd2.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())
		})

		It("should delete all volumes", func() {
			cmd.Args = append(cmd.Args, volumeNameFull1, volumeNameFull2)
			err := cmd.Run()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp(`removing 2 volume`))
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
			Expect(stdout.String()).To(MatchRegexp(`Permanently delete persistent volume`))
		})
	})
})
