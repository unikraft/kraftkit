// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package cli_test

import (
	. "github.com/onsi/ginkgo/v2" //nolint:stylecheck
	. "github.com/onsi/gomega"    //nolint:stylecheck

	fcmd "kraftkit.sh/test/e2e/framework/cmd"
	fcfg "kraftkit.sh/test/e2e/framework/config"
)

var _ = Describe("kraft version", func() {
	var cmd *fcmd.Cmd

	var stdout *fcmd.IOStream
	var stderr *fcmd.IOStream

	var cfg *fcfg.Config

	BeforeEach(func() {
		stdout = fcmd.NewIOStream()
		stderr = fcmd.NewIOStream()

		cfg = fcfg.NewTempConfig()

		cmd = fcmd.NewKraft(stdout, stderr, cfg.Path())
		cmd.Args = append(cmd.Args, "version", "--no-check-updates", "--log-level", "info", "--log-type", "json")
	})

	When("invoked without flags or positional arguments", func() {
		It("should print the version and exit gracefully", func() {
			err := cmd.Run()
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp(`^kraft [\w\.-]+ \(\w+\) [\w\.-]+ .+\n$`))
		})
	})

	When("invoked with the --help flag", func() {
		BeforeEach(func() {
			cmd.Args = append(cmd.Args, "--help")
		})

		It("should print the command's help", func() {
			err := cmd.Run()
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp(`^Show kraft version information\n`))
		})
	})

	When("invoked with positional arguments", func() {
		BeforeEach(func() {
			cmd.Args = append(cmd.Args, "some-arg")
		})

		It("should print an error and exit", func() {
			err := cmd.Run()
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("exit status 1"))

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp(
				`^{"level":"error","msg":"unknown command \\"some-arg\\" for \\"kraft version\\""}\n$`,
			))
		})
	})
})
