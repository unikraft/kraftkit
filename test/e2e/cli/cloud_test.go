// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package cli_test

import (
	. "github.com/onsi/ginkgo/v2" //nolint:stylecheck
	. "github.com/onsi/gomega"    //nolint:stylecheck

	fcmd "kraftkit.sh/test/e2e/framework/cmd"
	fcfg "kraftkit.sh/test/e2e/framework/config"
)

var _ = Describe("kraft cloud", func() {
	var cmd *fcmd.Cmd

	var stdout *fcmd.IOStream
	var stderr *fcmd.IOStream

	var cfg *fcfg.Config

	BeforeEach(func() {
		stdout = fcmd.NewIOStream()
		stderr = fcmd.NewIOStream()

		cfg = fcfg.NewTempConfig()

		cmd = fcmd.NewKraft(stdout, stderr, cfg.Path())
		cmd.Args = append(cmd.Args, "cloud")
	})

	When("invoked without flags or positional arguments", func() {
		It("should point to the unikraft CLI and exit", func() {
			err := cmd.Run()
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("exit status 1"))

			Expect(stdout.String()).To(BeEmpty())
			Expect(stderr.String()).To(ContainSubstring("`kraft cloud` has been removed"))
			Expect(stderr.String()).To(ContainSubstring("https://unikraft.com/docs/cli"))
		})
	})

	When("invoked with the --help flag", func() {
		BeforeEach(func() {
			cmd.Args = append(cmd.Args, "--help")
		})

		It("should point to the unikraft CLI and exit", func() {
			err := cmd.Run()
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("exit status 1"))

			Expect(stdout.String()).To(BeEmpty())
			Expect(stderr.String()).To(ContainSubstring("https://unikraft.com/docs/cli"))
		})
	})

	When("invoked with a subcommand and its flags", func() {
		BeforeEach(func() {
			cmd.Args = append(cmd.Args, "instance", "create", "--metro", "fra", "-p", "443:8080", "nginx")
		})

		It("should point to the unikraft CLI and exit", func() {
			err := cmd.Run()
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("exit status 1"))

			Expect(stdout.String()).To(BeEmpty())
			Expect(stderr.String()).To(ContainSubstring("https://unikraft.com/docs/cli"))
		})
	})
})

var _ = Describe("kraft cl", func() {
	var cmd *fcmd.Cmd

	var stdout *fcmd.IOStream
	var stderr *fcmd.IOStream

	var cfg *fcfg.Config

	BeforeEach(func() {
		stdout = fcmd.NewIOStream()
		stderr = fcmd.NewIOStream()

		cfg = fcfg.NewTempConfig()

		cmd = fcmd.NewKraft(stdout, stderr, cfg.Path())
		cmd.Args = append(cmd.Args, "cl", "instance", "list")
	})

	When("invoked with a subcommand", func() {
		It("should point to the unikraft CLI and exit", func() {
			err := cmd.Run()
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("exit status 1"))

			Expect(stdout.String()).To(BeEmpty())
			Expect(stderr.String()).To(ContainSubstring("https://unikraft.com/docs/cli"))
		})
	})
})

var _ = Describe("kraft --help", func() {
	var cmd *fcmd.Cmd

	var stdout *fcmd.IOStream
	var stderr *fcmd.IOStream

	var cfg *fcfg.Config

	BeforeEach(func() {
		stdout = fcmd.NewIOStream()
		stderr = fcmd.NewIOStream()

		cfg = fcfg.NewTempConfig()

		cmd = fcmd.NewKraft(stdout, stderr, cfg.Path())
		cmd.Args = append(cmd.Args, "--help")
	})

	It("should not list the cloud command", func() {
		err := cmd.Run()
		Expect(err).ToNot(HaveOccurred())

		Expect(stdout.String()).ToNot(BeEmpty())
		Expect(stdout.String()).ToNot(ContainSubstring("cloud"))
	})
})
