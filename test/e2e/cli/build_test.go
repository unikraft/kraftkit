// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package cli_test

import (
	"debug/elf"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2" //nolint:stylecheck
	. "github.com/onsi/gomega"    //nolint:stylecheck

	fcmd "kraftkit.sh/test/e2e/framework/cmd"
	fcfg "kraftkit.sh/test/e2e/framework/config"
)

const buildProjectName = "helloworld"

func buildFixturesDir() (string, error) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("cannot determine path of fixtures directory")
	}
	return filepath.Join(filepath.Dir(filename), "fixtures", buildProjectName), nil
}

func kernelPath(projectDir string) string {
	return filepath.Join(projectDir, ".unikraft", "build",
		buildProjectName+"_qemu-x86_64")
}

var _ = Describe("kraft build", func() {
	var stdout *fcmd.IOStream
	var stderr *fcmd.IOStream
	var cfg *fcfg.Config

	BeforeEach(func() {
		stdout = fcmd.NewIOStream()
		stderr = fcmd.NewIOStream()
		cfg = fcfg.NewTempConfig()
	})

	Context("building a project", Ordered, func() {
		var projectDir string

		BeforeAll(func(_ SpecContext) {
			if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
				Skip("building qemu/x86_64 target requires a Linux x86_64 host without cross-compilation toolchains")
			}

			var err error
			projectDir, err = os.MkdirTemp("", "kraftkit-build-project-*")
			Expect(err).ToNot(HaveOccurred())
			DeferCleanup(func() error {
				return os.RemoveAll(projectDir)
			})

			fixturesDir, err := buildFixturesDir()
			Expect(err).ToNot(HaveOccurred())
			Expect(os.CopyFS(projectDir, os.DirFS(fixturesDir))).To(Succeed())

			build := fcmd.NewKraft(stdout, stderr, cfg.Path())
			tmpBase := filepath.Dir(filepath.Dir(cfg.Path()))

			for _, env := range os.Environ() {
				if !strings.HasPrefix(env, "HOME=") {
					build.Env = append(build.Env, env)
				}
			}
			build.Env = append(build.Env, "HOME="+tmpBase)

			build.Dir = projectDir
			build.Args = append(build.Args,
				"build",
				"--plat", "qemu",
				"--arch", "x86_64",
				"--no-prompt",
				"--log-level", "info",
				"--log-type", "json",
			)

			err = build.Run()
			Expect(err).ToNot(HaveOccurred(), func() string {
				return build.DumpError(stdout, stderr, err)
			})
		}, NodeTimeout(20*time.Minute))

		It("should produce a valid ELF kernel image for the qemu x86_64 target", func() {
			path := kernelPath(projectDir)
			Expect(path).To(BeAnExistingFile())
			f, err := elf.Open(path)
			Expect(err).ToNot(HaveOccurred())
			DeferCleanup(f.Close)

			Expect(f.Type).To(Equal(elf.ET_EXEC))
			Expect(f.Class).To(Equal(elf.ELFCLASS32))
			Expect(f.Machine).To(Equal(elf.EM_386))

			contents, err := os.ReadFile(path)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(contents)).To(ContainSubstring("Hello world!"))
		})

		It("should write the target configuration file", func() {
			configPath := filepath.Join(projectDir, ".config.helloworld_qemu-x86_64")
			Expect(configPath).To(BeAnExistingFile())
		})
	})

	Context("help", func() {
		It("should print build help", func() {
			cmd := fcmd.NewKraft(stdout, stderr, cfg.Path())
			cmd.Args = append(cmd.Args, "build", "--help")
			err := cmd.Run()
			Expect(err).ToNot(HaveOccurred(), func() string {
				return cmd.DumpError(stdout, stderr, err)
			})
			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp(`(?m)^Build a Unikraft unikernel\.`))
			Expect(stdout.String()).To(MatchRegexp(`(?m)^USAGE`))
			Expect(stdout.String()).To(MatchRegexp(`(?m)^  kraft build \[FLAGS\] \[SUBCOMMAND\|DIR\]`))
		})
	})

	Context("validation", func() {
		It("should fail when target directory does not exist", func() {
			cmd := fcmd.NewKraft(stdout, stderr, cfg.Path())
			cmd.Args = append(cmd.Args, "build", "/path/does/not/exist", "--no-prompt", "--log-level", "error")
			err := cmd.Run()
			Expect(err).To(HaveOccurred())
			Expect(stderr.String()).To(ContainSubstring("path is not a valid directory"))
		})

		It("should fail with invalid rootfs-type flag", func() {
			cmd := fcmd.NewKraft(stdout, stderr, cfg.Path())
			cmd.Args = append(cmd.Args, "build", "--rootfs-type", "invalid_type", "--no-prompt", "--log-level", "error")
			err := cmd.Run()
			Expect(err).To(HaveOccurred())
			Expect(stderr.String()).To(ContainSubstring("invalid_type is not included in: cpio, erofs, file, unknown"))
		})
	})
})
