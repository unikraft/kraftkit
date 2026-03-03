// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package cli_test

import (
	"fmt"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	fcmd "kraftkit.sh/test/e2e/framework/cmd"
	fcfg "kraftkit.sh/test/e2e/framework/config"
	ukarch "kraftkit.sh/unikraft/arch"
)

var _ = Describe("kraft pkg list", Ordered, func() {
	var cmd *fcmd.Cmd
	var stdout *fcmd.IOStream
	var stderr *fcmd.IOStream
	var cfg *fcfg.Config
	var pkgs []string

	refreshCatalog := func(tmpBase string) {
		updateCmd := fcmd.NewKraft(stdout, stderr, cfg.Path())

		updateCmd.Env = append(updateCmd.Env, "HOME="+tmpBase)
		updateCmd.Dir = tmpBase

		updateCmd.Args = append(
			updateCmd.Args,
			"pkg",
			"update",
			"--log-level",
			"error",
		)

		err := updateCmd.Run()
		if err != nil {
			fmt.Print(updateCmd.DumpError(stdout, stderr, err))
		}
		Expect(err).ToNot(HaveOccurred())

		stderr.Reset()
		stdout.Reset()
	}

	BeforeAll(func() {
		cfg = fcfg.NewTempConfig()
		stdout = fcmd.NewIOStream()
		stderr = fcmd.NewIOStream()

		tmpBase := filepath.Dir(filepath.Dir(cfg.Path()))

		pkgs = []string{
			// stable fixed apps
			"unikraft.org/helloworld",
			"unikraft.org/base",
		}

		// refresh the local view of the remote catalog
		refreshCatalog(tmpBase)
	})

	BeforeEach(func() {
		stdout = fcmd.NewIOStream()
		stderr = fcmd.NewIOStream()

		cmd = fcmd.NewKraft(stdout, stderr, cfg.Path())

		// Force HOME to temp dir and prevent leaks from the host machine.
		tmpBase := filepath.Dir(filepath.Dir(cfg.Path()))
		cmd.Env = append(cmd.Env, "HOME="+tmpBase)
		cmd.Dir = tmpBase

		cmd.Args = append(
			cmd.Args,
			"pkg",
			"list",
			"--log-type",
			"json",
			"--output",
			"json",
		)
	})

	Context("listing from oci catalog", func() {
		runListCmd := func() {
			err := cmd.Run()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			// Assert logs
			Expect(
				stderr.String(),
			).To(ContainSubstring(`"msg":"updating"`))
		}
		When("no flags", func() {
			It("list packages filtered by host architecture (default)", func() {
				runListCmd()
				output := stdout.String()
				for _, pkg := range pkgs {
					Expect(output).To(ContainSubstring(pkg))
				}
			})
		})
		When("using --apps flags", func() {
			BeforeEach(func() {
				cmd.Args = append(cmd.Args, "--apps")
			})
			It("list apps filtered by host architecture", func() {
				runListCmd()
				output := stdout.String()
				for _, pkg := range pkgs {
					Expect(output).To(ContainSubstring(pkg))
				}
			})
		})
		When("using --all flag", func() {
			BeforeEach(func() {
				cmd.Args = append(cmd.Args, "--all")
			})
			It("list all packages", func() {
				runListCmd()

				output := stdout.String()

				for _, pkg := range pkgs {
					count := strings.Count(output, pkg)
					Expect(count).To(BeNumerically(">", 1))
				}
			})
		})
		When("using --arch flag", func() {
			BeforeEach(func() {
				cmd.Args = append(cmd.Args, "--arch")
			})

			It("filters for x86_64", func() {
				cmd.Args = append(cmd.Args, "x86_64")
				runListCmd()
				Expect(stdout.String()).To(ContainSubstring(`/x86_64`))
				Expect(stdout.String()).ToNot(ContainSubstring(`/arm64`))
			})
			It("filters for arm64", func() {
				cmd.Args = append(cmd.Args, "arm64")
				runListCmd()
				Expect(stdout.String()).To(ContainSubstring(`/arm64`))
				Expect(stdout.String()).ToNot(ContainSubstring(`/x86_64`))
			})

			It("defaults to host architecture for empty string", func() {
				cmd.Args = append(cmd.Args, "")

				hostArch, err := ukarch.HostArchitecture()
				Expect(err).ToNot(HaveOccurred())
				runListCmd()

				output := stdout.String()
				Expect(output).To(ContainSubstring("/" + hostArch))

				unexpectedArch := "x86_64"
				if hostArch == unexpectedArch {
					unexpectedArch = "arm64"
				}
				Expect(output).ToNot(ContainSubstring("/" + unexpectedArch))
			})
			It("filter for invalid architecture(no matches)", func() {
				cmd.Args = append(cmd.Args, "invalid")
				runListCmd()

				output := stdout.String()
				Expect(output).ToNot(ContainSubstring(`/x86_64`))
				Expect(output).ToNot(ContainSubstring(`/arm64`))
			})
		})
		When("using --plat flag", func() {
			BeforeEach(func() {
				cmd.Args = append(cmd.Args, "--plat")
			})

			It("filters for qemu", func() {
				cmd.Args = append(cmd.Args, "qemu")
				runListCmd()

				output := stdout.String()
				Expect(output).To(ContainSubstring(`qemu/`))
				Expect(output).ToNot(ContainSubstring(`fc/`))
			})
			It("filters for fc", func() {
				cmd.Args = append(cmd.Args, "fc")
				runListCmd()

				output := stdout.String()
				Expect(output).To(ContainSubstring(`fc/`))
				Expect(output).ToNot(ContainSubstring(`qemu/`))
			})

			It("defaults to host platform for empty string", func() {
				cmd.Args = append(cmd.Args, "")
				runListCmd()

				Expect(stdout.String()).To(MatchRegexp(`"(qemu|fc|xen|kraftcloud)/`))
			})
			It("filter for invalid platform (no matches in oci catalog)", func() {
				cmd.Args = append(cmd.Args, "invalid")
				runListCmd()

				output := stdout.String()
				Expect(output).ToNot(ContainSubstring(`qemu/`))
				Expect(output).ToNot(ContainSubstring(`fc/`))
			})
		})
		When("with a clean config", func() {
			BeforeEach(func() {
				// create empty config
				cfg := fcfg.NewTempConfig()
				stdout.Reset()
				stderr.Reset()

				cmd = fcmd.NewKraft(stdout, stderr, cfg.Path())
				tmpBase := filepath.Dir(filepath.Dir(cfg.Path()))
				cmd.Env = append(cmd.Env, "HOME="+tmpBase)
				cmd.Dir = tmpBase

				cmd.Args = append(
					cmd.Args,
					"pkg",
					"list",
					"--log-level",
					"debug",
					"--log-type",
					"json",
					"--output",
					"json",
				)
			})
			It("list no packages for --local", func() {
				cmd.Args = append(cmd.Args, "--local")
				runListCmd()

				debugLog := stderr.String()

				Expect(debugLog).To(ContainSubstring(`"local":true`))
				Expect(
					debugLog,
				).To(ContainSubstring(`{"level":"debug","msg":"found 0/0 matching packages in oci catalog"}`))
			})
			It("list all remote packages for --remote", func() {
				cmd.Args = append(cmd.Args, "--remote")
				runListCmd()

				Expect(stderr.String()).To(ContainSubstring(`"remote":true`))

				// confirs we got a json list
				Expect(stdout.String()).To(HavePrefix("["))
			})
		})
		When("invoked with the --help flag", func() {
			BeforeEach(func() {
				cmd.Args = append(cmd.Args, "--help")
			})

			It("should print the command's help header", func() {
				err := cmd.Run()
				Expect(err).ToNot(HaveOccurred())
				Expect(stderr.String()).To(BeEmpty())
				Expect(
					stdout.String(),
				).To(ContainSubstring(`List installed Unikraft component packages.`))
			})
		})
	})
})
