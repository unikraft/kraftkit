package cli_test

import (
	"fmt"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/kustomize/kyaml/yaml"

	fcmd "kraftkit.sh/test/e2e/framework/cmd"
	fcfg "kraftkit.sh/test/e2e/framework/config"
)

var _ = Describe("kraft pkg pull", func() {
	var cmd *fcmd.Cmd
	var stdout *fcmd.IOStream
	var stderr *fcmd.IOStream
	var cfg *fcfg.Config

	BeforeEach(func(ctx SpecContext) {
		stdout = fcmd.NewIOStream()
		stderr = fcmd.NewIOStream()
		cfg = fcfg.NewTempConfig()

		tmpBase := filepath.Dir(filepath.Dir(cfg.Path()))
		cmd = fcmd.NewKraft(stdout, stderr, cfg.Path())
		cmd.Env = append(cmd.Env, "HOME="+tmpBase)
		cmd.Dir = tmpBase

		cmd.Args = append(
			cmd.Args,
			"pkg", "pull",
			"--log-level", "info",
			"--log-type", "json",
		)
	})

	Context("error cases", func() {
		When("an invalid flag is passed", func() {
			It("returns unknown flag error", func() {
				cmd.Args = append(cmd.Args, "--invalid")

				err := cmd.Run()
				Expect(err).To(HaveOccurred())
				Expect(stderr.String()).To(ContainSubstring("unknown flag"))
			})
		})

		When("a flag that needs an argument gets none", func() {
			It("returns flag needs an argument error", func() {
				cmd.Args = append(cmd.Args, "--arch")

				err := cmd.Run()
				Expect(err).To(HaveOccurred())
				Expect(stderr.String()).To(ContainSubstring("flag needs an argument"))
			})
		})

		When("no Kraftfile exists in workdir", func() {
			It("returns an error with non-zero exit code", func() {
				tmpBase := filepath.Dir(filepath.Dir(cfg.Path()))
				cmd.Args = append(cmd.Args, "--workdir", tmpBase)

				err := cmd.Run()
				if err != nil {
					fmt.Print(cmd.DumpError(stdout, stderr, err))
				}
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Context("help", func() {
		When("invoked with the --help flag", func() {
			It("should print the command help", func() {
				cmd.Args = append(cmd.Args, "--help")

				err := cmd.Run()
				Expect(err).ToNot(HaveOccurred())
				Expect(stderr.String()).To(BeEmpty())
				Expect(stdout.String()).To(ContainSubstring(
					"Pull a Unikraft unikernel, component microlibrary from a remote location",
				))
			})
		})
	})
})

var _ = Describe("kraft pkg pull (remote catalog)", Ordered, func() {
	var cmd *fcmd.Cmd
	var stdout *fcmd.IOStream
	var stderr *fcmd.IOStream
	var cfg *fcfg.Config
	var tmpBase string

	BeforeAll(func(ctx SpecContext) {
		stdout = fcmd.NewIOStream()
		stderr = fcmd.NewIOStream()
		cfg = fcfg.NewTempConfig()

		tmpBase = filepath.Dir(filepath.Dir(cfg.Path()))

		cfg.Write(
			yaml.SetField(
				"unikraft",
				yaml.MustParse(`
manifests:
  - https://manifests.kraftkit.sh/index.yaml
`),
			),
		)

		updateCmd := fcmd.NewKraft(stdout, stderr, cfg.Path())
		updateCmd.Env = append(updateCmd.Env, "HOME="+tmpBase)
		updateCmd.Dir = tmpBase
		updateCmd.Args = append(
			updateCmd.Args,
			"pkg", "update",
			"--log-level", "error",
		)

		err := updateCmd.Run()
		if err != nil {
			fmt.Print(updateCmd.DumpError(stdout, stderr, err))
		}
		Expect(err).ToNot(HaveOccurred())

		stdout.Reset()
		stderr.Reset()
	}, NodeTimeout(120*time.Second))

	BeforeEach(func() {
		stdout = fcmd.NewIOStream()
		stderr = fcmd.NewIOStream()

		cmd = fcmd.NewKraft(stdout, stderr, cfg.Path())
		cmd.Env = append(cmd.Env, "HOME="+tmpBase)
		cmd.Dir = tmpBase

		cmd.Args = append(
			cmd.Args,
			"pkg", "pull",
			"--log-level", "info",
			"--log-type", "json",
		)
	})

	Context("pulling a known package by name", func() {
		When("package exists in the remote catalog", func() {
			XIt("should pull successfully and print pulling message", func(ctx SpecContext) {
				cmd.Args = append(
					cmd.Args,
					"--update",
					"--no-checksum",
					"unikraft.org/helloworld:latest",
				)

				err := cmd.Run()
				if err != nil {
					fmt.Print(cmd.DumpError(stdout, stderr, err))
				}

				Expect(err).ToNot(HaveOccurred())
				Expect(stderr.String()).To(ContainSubstring(
					`"msg":"pulling unikraft.org/helloworld:latest`,
				))
			}, SpecTimeout(120*time.Second))
		})

		When("a non-existent package is requested", func() {
			It("should return an error and print could not find message", func(ctx SpecContext) {
				cmd.Args = append(
					cmd.Args,
					"unikraft.org/doesnotexist-xyz:latest",
				)

				err := cmd.Run()
				if err != nil {
					fmt.Print(cmd.DumpError(stdout, stderr, err))
				}

				Expect(err).To(HaveOccurred())
				Expect(stderr.String()).To(ContainSubstring("could not find"))
			}, SpecTimeout(60*time.Second))
		})
	})
})
