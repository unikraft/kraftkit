package cli_test

import (
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	fcmd "kraftkit.sh/test/e2e/framework/cmd"
	fcfg "kraftkit.sh/test/e2e/framework/config"
	"kraftkit.sh/test/e2e/framework/matchers"
	"kraftkit.sh/unikraft/app"
)

const kraftYAML = `specification: v0.6
name: helloworld
unikraft:
  version: stable
targets:
  - architecture: x86_64
    platform: qemu
`

const mainC = `#include <stdio.h>
int main() {
    printf("Hello world from Unikraft!\n");
    return 0;
}
`

const makefileUK = `$(eval $(call addlib,apphelloworld))

APPHELLOWORLD_SRCS-y += $(APPHELLOWORLD_BASE)/main.c
`

var _ = Describe("kraft clean", func() {
	var cmd *fcmd.Cmd

	var stdout *fcmd.IOStream
	var stderr *fcmd.IOStream

	var cfg *fcfg.Config

	BeforeEach(func() {
		stderr = fcmd.NewIOStream()
		stdout = fcmd.NewIOStream()

		cfg = fcfg.NewTempConfig()

		cmd = fcmd.NewKraft(stdout, stderr, cfg.Path())
		cmd.Args = append(cmd.Args, "clean", "--log-level", "info", "--log-type", "json")
		cmd.Dir = GinkgoT().TempDir()
	})

	Describe("error cases", func() {
		When("no valid Kraftfile exists", func() {
			BeforeEach(func() {
				// ensure that no Kraftfile exists in the Path
				fileMatcher := matchers.ContainFiles(app.DefaultFileNames...)
				success, err := fileMatcher.Match(cmd.Dir)
				Expect(err).ToNot(HaveOccurred())
				Expect(success).To(BeFalse())
			})
			It("returns an errors with non-zero exit code", func() {
				err := cmd.Run()
				if err != nil {
					fmt.Print(cmd.DumpError(stdout, stderr, err))
				}
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("exit status 1"))
				Expect(stderr.String()).To(ContainSubstring("no Kraftfile specified"))
				Expect(stdout.String()).To(BeEmpty())
			})
		})

		When("invalid flag is passed", func() {
			BeforeEach(func() {
				cmd.Args = append(cmd.Args, "--invalid")
			})
			It("returns unknown flag error", func() {
				err := cmd.Run()
				if err != nil {
					fmt.Print(cmd.DumpError(stdout, stderr, err))
				}
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("exit status 1"))
				Expect(stderr.String()).To(ContainSubstring("unknown flag"))
				Expect(stdout.String()).To(BeEmpty())
			})
		})

		When("invalid shorthand is passed", func() {
			BeforeEach(func() {
				cmd.Args = append(cmd.Args, "-Z")
			})
			It("returns unknown shorthand flag error", func() {
				err := cmd.Run()
				if err != nil {
					fmt.Print(cmd.DumpError(stdout, stderr, err))
				}
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("exit status 1"))
				Expect(stderr.String()).To(ContainSubstring("unknown shorthand flag"))
				Expect(stdout.String()).To(BeEmpty())
			})
		})

		When("flag expects an argument but nothing is passed", func() {
			BeforeEach(func() {
				cmd.Args = append(cmd.Args, "-K")
			})
			It("exits with flag needs an argument error", func() {
				err := cmd.Run()
				if err != nil {
					fmt.Print(cmd.DumpError(stdout, stderr, err))
				}
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("exit status 1"))
				Expect(stderr.String()).To(ContainSubstring("flag needs an argument"))
				Expect(stdout.String()).To(BeEmpty())
			})
		})
	})

	Describe("executes successfully", func() {
		When("build artifact exists", func() {
			BeforeEach(func() {
				workdir := cmd.Dir

				// write kraft.yaml
				writeFile(workdir, "kraft.yaml", kraftYAML)

				// write main.c
				writeFile(workdir, "main.c", mainC)

				// write Makefile.uk
				writeFile(workdir, "Makefile.uk", makefileUK)

				buildStderr, buildStdout := fcmd.NewIOStream(), fcmd.NewIOStream()
				buildCmd := fcmd.NewKraft(buildStdout, buildStderr, cfg.Path())
				buildCmd.Args = append(buildCmd.Args, "build", "--arch", "x86_64", "--plat", "qemu", workdir)

				err := buildCmd.Run()
				if err != nil {
					fmt.Print(cmd.DumpError(buildStdout, buildStderr, err))
				}
				Expect(err).ToNot(HaveOccurred())

				Expect(workdir).To(matchers.ContainDirectories(".unikraft/build"))
				Expect(filepath.Join(workdir, ".unikraft", "build")).To(matchers.ContainFiles("helloworld_qemu-x86_64"))
			})
			It("removes kernel binary and debug files from .unikraft/build", func() {
				err := cmd.Run()
				if err != nil {
					fmt.Print(cmd.DumpError(stdout, stderr, err))
				}
				Expect(err).ToNot(HaveOccurred())

				buildDir := filepath.Join(cmd.Dir, ".unikraft", "build")

				// assert removed
				Expect(filepath.Join(buildDir, "helloworld_qemu-x86_64")).ToNot(BeAnExistingFile())
				Expect(buildDir).ToNot(matchers.ContainFiles(
					"helloworld_qemu-x86_64.dbg",
					"helloworld_qemu-x86_64.dbg.cmd",
					"helloworld_qemu-x86_64.dbg.gdb.py",
				))

				// assert kept
				Expect(buildDir).To(matchers.ContainFiles(
					"helloworld_qemu-x86_64.bootinfo",
					"helloworld_qemu-x86_64.bootinfo.cmd",
					"helloworld_qemu-x86_64.multiboot.cmd",
				))
			})

			It("removes the entire .unikraft/build directory when --proper flag is passed", func() {
				cmd.Args = append(cmd.Args, "--proper")
				err := cmd.Run()
				if err != nil {
					fmt.Print(cmd.DumpError(stdout, stderr, err))
				}
				Expect(err).ToNot(HaveOccurred())

				// assert --proper removes .unikraft/build and preserves .unikraft/
				Expect(filepath.Join(cmd.Dir, ".unikraft", "build")).ToNot(BeADirectory())
				Expect(filepath.Join(cmd.Dir, ".unikraft")).To(BeADirectory())
			})
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
			Expect(stdout.String()).To(MatchRegexp(`^Remove the build object files of a Unikraft project\n`))
		})
	})
})

func writeFile(workdir string, fileName string, fileContents string) {
	Expect(os.WriteFile(
		filepath.Join(workdir, fileName),
		[]byte(fileContents),
		0o644,
	)).To(Succeed())
}
