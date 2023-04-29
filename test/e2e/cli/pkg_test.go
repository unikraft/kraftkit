// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package cli_test

import (
	. "github.com/onsi/ginkgo/v2" //nolint:stylecheck
	. "github.com/onsi/gomega"    //nolint:stylecheck

	"sigs.k8s.io/kustomize/kyaml/yaml"

	fcmd "kraftkit.sh/test/e2e/framework/cmd"
	fcfg "kraftkit.sh/test/e2e/framework/config"
	. "kraftkit.sh/test/e2e/framework/matchers" //nolint:stylecheck
)

var _ = Describe("kraft pkg", func() {
	var cmd *fcmd.Cmd

	var stdout *fcmd.IOStream
	var stderr *fcmd.IOStream

	var cfg *fcfg.Config

	BeforeEach(func() {
		stdout = fcmd.NewIOStream()
		stderr = fcmd.NewIOStream()

		cfg = fcfg.NewTempConfig()

		cmd = fcmd.NewKraft(stdout, stderr, cfg.Path())
		cmd.Args = append(cmd.Args, "pkg")
	})

	_ = Describe("update", func() {
		var manifestsPath string

		BeforeEach(func() {
			cmd.Args = append(cmd.Args, "update")

			manifestsPath = yaml.GetValue(cfg.Read("paths", "manifests"))
			Expect(manifestsPath).To(SatisfyAny(
				Not(BeAnExistingFile()),
				BeAnEmptyDirectory(),
			), "manifests directory should either be empty or not yet created")
		})

		Context("implicitly using the default manager type (manifest)", func() {
			When("invoked without flags or positional arguments", func() {
				It("should retrieve the list of components, libraries and packages", func() {
					err := cmd.Run()
					Expect(err).ToNot(HaveOccurred())

					Expect(stderr.String()).To(BeEmpty())
					// The command sends ANSI escape sequences while updating, such as `\e[2K` (erase entire line).
					// References:
					//   https://www.regular-expressions.info/nonprint.html
					//   https://gist.github.com/fnky/458719343aabd01cfb17a3a4f7296797
					Expect(stdout.String()).To(MatchRegexp(`\x1b\[2K\[\+\] Updating\.\.\. \[\d+\.\d+s\]\r\n`), "Quoted output: %q", stdout)

					Expect(manifestsPath).To(ContainFiles("index.yaml", "unikraft.yaml"))
					Expect(manifestsPath).To(ContainDirectories("libs"))
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
					Expect(stdout).To(MatchRegexp(`^Retrieve new lists of Unikraft components, libraries and packages.\n`))
				})
			})
		})
	})
})
