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
		volumeName = "volume-create-test"
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
		cmd.Args = append(cmd.Args, "cloud", "volume", "create", "--log-level", "info", "--log-type", "json")
	})

	When("invoked with no flags", func() {
		It("should error out with a kraftkit error", func() {
			err := cmd.Run()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}
			Expect(err).To(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp(`must specify --size flag`))
		})
	})

	When("invoked with the size flag of unit 8", func() {
		var volumeUUID string

		BeforeEach(func() {
			cmd.Args = append(cmd.Args, "--size", "8")
		})

		AfterEach(func() {
			cmd = fcmd.NewKraft(stdout, stderr, cfg.Path())
			cmd.Env = os.Environ()
			cmd.Args = []string{"cloud", "volume", "delete", volumeUUID, "--log-level", "info", "--log-type", "json"}
			err := cmd.Run()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())
		})

		It("should create a volume of size 8Mi", func() {
			err := cmd.Run()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())

			volumeUUID = stdout.String()
		})
	})

	When("invoked with the size flag of unit 8Mi", func() {
		var volumeUUID string

		BeforeEach(func() {
			cmd.Args = append(cmd.Args, "--size", "8Mi")
		})

		AfterEach(func() {
			cmd = fcmd.NewKraft(stdout, stderr, cfg.Path())
			cmd.Env = os.Environ()
			cmd.Args = []string{"cloud", "volume", "delete", volumeUUID, "--log-level", "info", "--log-type", "json"}
			err := cmd.Run()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())
		})

		It("should create a volume of size 8Mi", func() {
			err := cmd.Run()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())

			volumeUUID = stdout.String()
		})
	})

	When("invoked with the size flag of unit 9M", func() {
		var volumeUUID string

		BeforeEach(func() {
			cmd.Args = append(cmd.Args, "--size", "9M")
		})

		AfterEach(func() {
			cmd = fcmd.NewKraft(stdout, stderr, cfg.Path())
			cmd.Env = os.Environ()
			cmd.Args = []string{"cloud", "volume", "delete", volumeUUID, "--log-level", "info", "--log-type", "json"}
			err := cmd.Run()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())
		})

		It("should create a volume of size 9M", func() {
			err := cmd.Run()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())

			volumeUUID = stdout.String()
		})
	})

	When("invoked with the size flag of unit 1Mi and a name", func() {
		var volumeUUID string

		BeforeEach(func() {
			id, err := rand.Int(rand.Reader, big.NewInt(100000000000))
			if err != nil {
				panic(err)
			}
			volumeNameFull := fmt.Sprintf("%s-%d", volumeName, id)

			cmd.Args = append(cmd.Args, "--size", "8Mi", "--name", volumeNameFull)
		})

		AfterEach(func() {
			cmd = fcmd.NewKraft(stdout, stderr, cfg.Path())
			cmd.Env = os.Environ()
			cmd.Args = []string{"cloud", "volume", "delete", volumeUUID, "--log-level", "info", "--log-type", "json"}
			err := cmd.Run()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())
		})

		It("should create a volume of size 1Mi", func() {
			err := cmd.Run()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())

			volumeUUID = stdout.String()
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
			Expect(stdout.String()).To(MatchRegexp(`Create a new persistent volume.`))
		})
	})
})
