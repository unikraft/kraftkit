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

var _ = Describe("kraft cloud volume list", func() {
	var cmd *fcmd.Cmd

	var stdout *fcmd.IOStream
	var stderr *fcmd.IOStream

	var cfg *fcfg.Config

	const (
		volumeName = "volume-list-test"
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
		cmd.Args = append(cmd.Args, "cloud", "volume", "list", "--log-level", "info", "--log-type", "json")
	})

	When("invoked with json output", Serial, func() {
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
			createCmd1.Args = append(createCmd1.Args, "cloud", "volume", "create", "--log-level", "info", "--log-type", "json", "--size", "8", "--name", volumeNameFull1)
			err = createCmd1.Run()
			if err != nil {
				fmt.Print(createCmd1.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())

			createCmd2 := fcmd.NewKraft(stdout, stderr, cfg.Path())
			createCmd2.Env = os.Environ()
			createCmd2.Args = append(createCmd2.Args, "cloud", "volume", "create", "--log-level", "info", "--log-type", "json", "--size", "8", "--name", volumeNameFull2)
			err = createCmd2.Run()
			if err != nil {
				fmt.Print(createCmd2.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())
		})

		AfterEach(func() {
			rmCmd1 := fcmd.NewKraft(stdout, stderr, cfg.Path())
			rmCmd1.Env = os.Environ()
			rmCmd1.Args = append(rmCmd1.Args, "cloud", "volume", "delete", "--log-level", "info", "--log-type", "json", volumeNameFull1)
			err := rmCmd1.Run()
			if err != nil {
				fmt.Print(rmCmd1.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())

			rmCmd2 := fcmd.NewKraft(stdout, stderr, cfg.Path())
			rmCmd2.Env = os.Environ()
			rmCmd2.Args = append(rmCmd2.Args, "cloud", "volume", "delete", "--log-level", "info", "--log-type", "json", volumeNameFull2)
			err = rmCmd2.Run()
			if err != nil {
				fmt.Print(rmCmd2.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())
		})

		It("should list two instances", func() {
			cmd.Args = append(cmd.Args, "-o", "json")
			err := cmd.Run()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp(`"name":"` + volumeNameFull1 + `"`))
			Expect(stdout.String()).To(MatchRegexp(`"name":"` + volumeNameFull2 + `"`))
		})
	})

	When("invoked with list output", Serial, func() {
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
			createCmd1.Args = append(createCmd1.Args, "cloud", "volume", "create", "--log-level", "info", "--log-type", "json", "--size", "8", "--name", volumeNameFull1)
			err = createCmd1.Run()
			if err != nil {
				fmt.Print(createCmd1.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())

			createCmd2 := fcmd.NewKraft(stdout, stderr, cfg.Path())
			createCmd2.Env = os.Environ()
			createCmd2.Args = append(createCmd2.Args, "cloud", "volume", "create", "--log-level", "info", "--log-type", "json", "--size", "8", "--name", volumeNameFull2)
			err = createCmd2.Run()
			if err != nil {
				fmt.Print(createCmd2.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())
		})

		AfterEach(func() {
			rmCmd1 := fcmd.NewKraft(stdout, stderr, cfg.Path())
			rmCmd1.Env = os.Environ()
			rmCmd1.Args = append(rmCmd1.Args, "cloud", "volume", "delete", "--log-level", "info", "--log-type", "json", volumeNameFull1)
			err := rmCmd1.Run()
			if err != nil {
				fmt.Print(rmCmd1.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())

			rmCmd2 := fcmd.NewKraft(stdout, stderr, cfg.Path())
			rmCmd2.Env = os.Environ()
			rmCmd2.Args = append(rmCmd2.Args, "cloud", "volume", "delete", "--log-level", "info", "--log-type", "json", volumeNameFull2)
			err = rmCmd2.Run()
			if err != nil {
				fmt.Print(rmCmd2.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())
		})

		It("should list two instances", func() {
			cmd.Args = append(cmd.Args, "-o", "list")
			err := cmd.Run()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp(`name: ` + volumeNameFull1))
			Expect(stdout.String()).To(MatchRegexp(`name: ` + volumeNameFull2))
		})
	})

	When("invoked with table output", Serial, func() {
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
			createCmd1.Args = append(createCmd1.Args, "cloud", "volume", "create", "--log-level", "info", "--log-type", "json", "--size", "8", "--name", volumeNameFull1)
			err = createCmd1.Run()
			if err != nil {
				fmt.Print(createCmd1.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())

			createCmd2 := fcmd.NewKraft(stdout, stderr, cfg.Path())
			createCmd2.Env = os.Environ()
			createCmd2.Args = append(createCmd2.Args, "cloud", "volume", "create", "--log-level", "info", "--log-type", "json", "--size", "8", "--name", volumeNameFull2)
			err = createCmd2.Run()
			if err != nil {
				fmt.Print(createCmd2.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())
		})

		AfterEach(func() {
			rmCmd1 := fcmd.NewKraft(stdout, stderr, cfg.Path())
			rmCmd1.Env = os.Environ()
			rmCmd1.Args = append(rmCmd1.Args, "cloud", "volume", "delete", "--log-level", "info", "--log-type", "json", volumeNameFull1)
			err := rmCmd1.Run()
			if err != nil {
				fmt.Print(rmCmd1.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())

			rmCmd2 := fcmd.NewKraft(stdout, stderr, cfg.Path())
			rmCmd2.Env = os.Environ()
			rmCmd2.Args = append(rmCmd2.Args, "cloud", "volume", "delete", "--log-level", "info", "--log-type", "json", volumeNameFull2)
			err = rmCmd2.Run()
			if err != nil {
				fmt.Print(rmCmd2.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())
		})

		It("should list two instances", func() {
			cmd.Args = append(cmd.Args, "-o", "table")
			err := cmd.Run()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp(volumeNameFull1))
			Expect(stdout.String()).To(MatchRegexp(volumeNameFull2))
		})
	})

	When("invoked with raw output", Serial, func() {
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
			createCmd1.Args = append(createCmd1.Args, "cloud", "volume", "create", "--log-level", "info", "--log-type", "json", "--size", "8", "--name", volumeNameFull1)
			err = createCmd1.Run()
			if err != nil {
				fmt.Print(createCmd1.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())

			createCmd2 := fcmd.NewKraft(stdout, stderr, cfg.Path())
			createCmd2.Env = os.Environ()
			createCmd2.Args = append(createCmd2.Args, "cloud", "volume", "create", "--log-level", "info", "--log-type", "json", "--size", "8", "--name", volumeNameFull2)
			err = createCmd2.Run()
			if err != nil {
				fmt.Print(createCmd2.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())
		})

		AfterEach(func() {
			rmCmd1 := fcmd.NewKraft(stdout, stderr, cfg.Path())
			rmCmd1.Env = os.Environ()
			rmCmd1.Args = append(rmCmd1.Args, "cloud", "volume", "delete", "--log-level", "info", "--log-type", "json", volumeNameFull1)
			err := rmCmd1.Run()
			if err != nil {
				fmt.Print(rmCmd1.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())

			rmCmd2 := fcmd.NewKraft(stdout, stderr, cfg.Path())
			rmCmd2.Env = os.Environ()
			rmCmd2.Args = append(rmCmd2.Args, "cloud", "volume", "delete", "--log-level", "info", "--log-type", "json", volumeNameFull2)
			err = rmCmd2.Run()
			if err != nil {
				fmt.Print(rmCmd2.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())
		})

		It("should list two instances", func() {
			cmd.Args = append(cmd.Args, "-o", "raw")
			err := cmd.Run()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp(`"name":"` + volumeNameFull1 + `"`))
			Expect(stdout.String()).To(MatchRegexp(`"name":"` + volumeNameFull2 + `"`))
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
			Expect(stdout.String()).To(MatchRegexp(`List all volumes in your account`))
		})
	})
})
