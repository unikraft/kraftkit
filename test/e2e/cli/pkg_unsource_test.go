// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package cli_test

import (
	"fmt"
	"os"

	. "github.com/onsi/ginkgo/v2" //nolint:stylecheck
	. "github.com/onsi/gomega"    //nolint:stylecheck

	"sigs.k8s.io/kustomize/kyaml/yaml"

	fcmd "kraftkit.sh/test/e2e/framework/cmd"
	fcfg "kraftkit.sh/test/e2e/framework/config"
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

	_ = Describe("unsource", func() {
		BeforeEach(func() {
			cmd.Args = append(cmd.Args, "unsource", "--log-level", "info", "--log-type", "json")
		})

		Context("unsourcing a link in the config file", func() {
			When("the config file already exists", func() {
				It("should remove the link and print nothing", func() {
					cmd.Args = append(cmd.Args, "https://manifests.kraftkit.sh/index.yaml")
					err := cmd.Run()
					if err != nil {
						fmt.Printf("Error running command, dumping output:\n%s\n%s\n%s\n", err, stderr, stdout)
					}
					Expect(err).ToNot(HaveOccurred())
					Expect(stderr.String()).To(BeEmpty())

					// Read the config file
					osFile, err := os.Open(cfg.Path())
					if err != nil {
						fmt.Printf("Error running command, dumping output:\n%s\n%s\n%s\n", err, stderr, stdout)
					}
					Expect(err).ToNot(HaveOccurred())
					defer osFile.Close()

					osFileInfo, err := osFile.Stat()
					if err != nil {
						fmt.Printf("Error running command, dumping output:\n%s\n%s\n%s\n", err, stderr, stdout)
					}
					Expect(err).ToNot(HaveOccurred())

					// Check if the config file is not empty
					Expect(osFileInfo.Size()).To(BeNumerically(">", 0))

					// Check if the config file has 600 permissions
					Expect(osFileInfo.Mode().Perm()).To(Equal(os.FileMode(0o600)))

					// Read file content
					readBytes, err := os.ReadFile(cfg.Path())
					if err != nil {
						fmt.Printf("Error running command, dumping output:\n%s\n%s\n%s\n", err, stderr, stdout)
					}
					Expect(err).ToNot(HaveOccurred())

					// Marshal the yaml file into a map
					var cfgMap map[string]interface{}
					err = yaml.Unmarshal([]byte(readBytes), &cfgMap)
					if err != nil {
						fmt.Printf("Error running command, dumping output:\n%s\n%s\n%s\n", err, stderr, stdout)
					}
					Expect(err).ToNot(HaveOccurred())

					// Check if the config file contains the default manifests
					cfgMapUnikernel, ok := cfgMap["unikraft"].(map[string]interface{})
					Expect(ok).To(BeTrue())

					// Cast the manifests list to an array
					cfgMapUnikernelManifests, ok := cfgMapUnikernel["manifests"].([]interface{})
					Expect(ok).To(BeTrue())

					// Check if the default manifests are removed
					Expect(cfgMapUnikernelManifests).To(HaveLen(0))

					// Check if stdout is empty
					Expect(stdout.String()).To(BeEmpty())
				})
			})

			When("there is no config file", func() {
				BeforeEach(func() {
					// Save the config file by moving it to a temporary location
					err := os.Rename(cfg.Path(), cfg.Path()+".tmp")
					if err != nil {
						fmt.Printf("Error running command, dumping output:\n%s\n%s\n%s\n", err, stderr, stdout)
					}
					Expect(err).ToNot(HaveOccurred())
				})
				AfterEach(func() {
					// Restore the config file
					err := os.Rename(cfg.Path()+".tmp", cfg.Path())
					if err != nil {
						fmt.Printf("Error running command, dumping output:\n%s\n%s\n%s\n", err, stderr, stdout)
					}
					Expect(err).ToNot(HaveOccurred())
				})
				It("should create the config file, add the default manifests, remove the link, and print nothing", func() {
					cmd.Args = append(cmd.Args, "https://manifests.kraftkit.sh/index.yaml")
					err := cmd.Run()
					if err != nil {
						fmt.Printf("Error running command, dumping output:\n%s\n%s\n%s\n", err, stderr, stdout)
					}
					Expect(err).ToNot(HaveOccurred())
					Expect(stderr.String()).To(BeEmpty())

					// Read the config file
					osFile, err := os.Open(cfg.Path())
					if err != nil {
						fmt.Printf("Error running command, dumping output:\n%s\n%s\n%s\n", err, stderr, stdout)
					}
					Expect(err).ToNot(HaveOccurred())
					defer osFile.Close()

					osFileInfo, err := osFile.Stat()
					if err != nil {
						fmt.Printf("Error running command, dumping output:\n%s\n%s\n%s\n", err, stderr, stdout)
					}
					Expect(err).ToNot(HaveOccurred())

					// Check if the config file is not empty
					Expect(osFileInfo.Size()).To(BeNumerically(">", 0))

					// Check if the config file has 600 permissions
					Expect(osFileInfo.Mode().Perm()).To(Equal(os.FileMode(0o600)))

					// Read file content
					readBytes, err := os.ReadFile(cfg.Path())
					if err != nil {
						fmt.Printf("Error running command, dumping output:\n%s\n%s\n%s\n", err, stderr, stdout)
					}
					Expect(err).ToNot(HaveOccurred())

					// Marshal the yaml file into a map
					var cfgMap map[string]interface{}
					err = yaml.Unmarshal([]byte(readBytes), &cfgMap)
					if err != nil {
						fmt.Printf("Error running command, dumping output:\n%s\n%s\n%s\n", err, stderr, stdout)
					}
					Expect(err).ToNot(HaveOccurred())

					// Check if the config file contains the default manifests
					cfgMapUnikernel, ok := cfgMap["unikraft"].(map[string]interface{})
					Expect(ok).To(BeTrue())

					// Cast the manifests list to an array
					cfgMapUnikernelManifests, ok := cfgMapUnikernel["manifests"].([]interface{})
					Expect(ok).To(BeTrue())

					// Check if the default manifests are removed
					Expect(cfgMapUnikernelManifests).To(HaveLen(0))

					// Check if stdout is empty
					Expect(stdout.String()).To(BeEmpty())
				})
			})

			When("the config does not contain the link or it doesn't exist", func() {
				BeforeEach(func() {
					// Save the config file by moving it to a temporary location
					err := os.Rename(cfg.Path(), cfg.Path()+".tmp")
					if err != nil {
						fmt.Printf("Error running command, dumping output:\n%s\n%s\n%s\n", err, stderr, stdout)
					}
					Expect(err).ToNot(HaveOccurred())
				})
				AfterEach(func() {
					// Restore the config file
					err := os.Rename(cfg.Path()+".tmp", cfg.Path())
					if err != nil {
						fmt.Printf("Error running command, dumping output:\n%s\n%s\n%s\n", err, stderr, stdout)
					}
					Expect(err).ToNot(HaveOccurred())
				})
				It("should do nothing and print a warning", func() {
					cmd.Args = append(cmd.Args, "https://example.com")
					err := cmd.Run()
					if err != nil {
						fmt.Printf("Error running command, dumping output:\n%s\n%s\n%s\n", err, stderr, stdout)
					}
					Expect(err).ToNot(HaveOccurred())
					Expect(stderr.String()).To(BeEmpty())

					// Read the config file
					osFile, err := os.Open(cfg.Path())
					if err != nil {
						fmt.Printf("Error running command, dumping output:\n%s\n%s\n%s\n", err, stderr, stdout)
					}
					Expect(err).ToNot(HaveOccurred())
					defer osFile.Close()

					osFileInfo, err := osFile.Stat()
					if err != nil {
						fmt.Printf("Error running command, dumping output:\n%s\n%s\n%s\n", err, stderr, stdout)
					}
					Expect(err).ToNot(HaveOccurred())

					// Check if the config file is not empty
					Expect(osFileInfo.Size()).To(BeNumerically(">", 0))

					// Check if the config file has 600 permissions
					Expect(osFileInfo.Mode().Perm()).To(Equal(os.FileMode(0o600)))

					// Read file content
					readBytes, err := os.ReadFile(cfg.Path())
					if err != nil {
						fmt.Printf("Error running command, dumping output:\n%s\n%s\n%s\n", err, stderr, stdout)
					}
					Expect(err).ToNot(HaveOccurred())

					// Marshal the yaml file into a map
					var cfgMap map[string]interface{}
					err = yaml.Unmarshal([]byte(readBytes), &cfgMap)
					if err != nil {
						fmt.Printf("Error running command, dumping output:\n%s\n%s\n%s\n", err, stderr, stdout)
					}
					Expect(err).ToNot(HaveOccurred())

					// Check if the config file contains the default manifests
					cfgMapUnikernel, ok := cfgMap["unikraft"].(map[string]interface{})
					Expect(ok).To(BeTrue())

					// Cast the manifests list to an array
					cfgMapUnikernelManifests, ok := cfgMapUnikernel["manifests"].([]interface{})
					Expect(ok).To(BeTrue())

					// Check if the default manifests are still there
					Expect(cfgMapUnikernelManifests).To(HaveLen(1))
					Expect(cfgMapUnikernelManifests[0]).To(Equal("https://manifests.kraftkit.sh/index.yaml"))

					// Check if stdout contains the warning
					Expect(stdout.String()).To(MatchRegexp(`^{"level":"warning","msg":"manifest not found: https://example\.com"}\n$`))
				})
			})
		})

		Context("unsourcing multiple links in the config file", func() {
			When("a config file was already present, and all links are unique", func() {
				BeforeEach(func() {
					unsourceArgs := make([]string, len(cmd.Args))
					copy(unsourceArgs, cmd.Args)

					for idx, arg := range cmd.Args {
						if arg == "unsource" {
							cmd.Args[idx] = "source"
							break
						}
					}
					cmd.Args = append(cmd.Args, "https://example1.com")
					cmd.Args = append(cmd.Args, "https://example2.com")
					cmd.Args = append(cmd.Args, "https://example3.com")
					err := cmd.Run()
					if err != nil {
						fmt.Printf("Error running command, dumping output:\n%s\n%s\n%s\n", err, stderr, stdout)
					}
					Expect(err).ToNot(HaveOccurred())
					Expect(stderr.String()).To(BeEmpty())

					// Recreate the command to reuse
					stdout = fcmd.NewIOStream()
					stderr = fcmd.NewIOStream()

					cmd = fcmd.NewKraft(stdout, stderr, cfg.Path())
					cmd.Args = unsourceArgs
				})

				It("should remove the links and print nothing", func() {
					cmd.Args = append(cmd.Args, "https://example1.com")
					cmd.Args = append(cmd.Args, "https://example2.com")
					cmd.Args = append(cmd.Args, "https://example3.com")
					err := cmd.Run()
					if err != nil {
						fmt.Printf("Error running command, dumping output:\n%s\n%s\n%s\n", err, stderr, stdout)
					}
					Expect(err).ToNot(HaveOccurred())
					Expect(stderr.String()).To(BeEmpty())

					// Read the config file
					osFile, err := os.Open(cfg.Path())
					if err != nil {
						fmt.Printf("Error running command, dumping output:\n%s\n%s\n%s\n", err, stderr, stdout)
					}
					Expect(err).ToNot(HaveOccurred())
					defer osFile.Close()

					osFileInfo, err := osFile.Stat()
					if err != nil {
						fmt.Printf("Error running command, dumping output:\n%s\n%s\n%s\n", err, stderr, stdout)
					}
					Expect(err).ToNot(HaveOccurred())

					// Check if the config file is not empty
					Expect(osFileInfo.Size()).To(BeNumerically(">", 0))

					// Check if the config file has 600 permissions
					Expect(osFileInfo.Mode().Perm()).To(Equal(os.FileMode(0o600)))

					// Read file content
					readBytes, err := os.ReadFile(cfg.Path())
					if err != nil {
						fmt.Printf("Error running command, dumping output:\n%s\n%s\n%s\n", err, stderr, stdout)
					}
					Expect(err).ToNot(HaveOccurred())

					// Marshal the yaml file into a map
					var cfgMap map[string]interface{}
					err = yaml.Unmarshal([]byte(readBytes), &cfgMap)
					if err != nil {
						fmt.Printf("Error running command, dumping output:\n%s\n%s\n%s\n", err, stderr, stdout)
					}
					Expect(err).ToNot(HaveOccurred())

					// Check if the config file contains the default manifests
					cfgMapUnikernel, ok := cfgMap["unikraft"].(map[string]interface{})
					Expect(ok).To(BeTrue())

					// Cast the manifests list to an array
					cfgMapUnikernelManifests, ok := cfgMapUnikernel["manifests"].([]interface{})
					Expect(ok).To(BeTrue())

					// Check if the additional links are removed
					Expect(cfgMapUnikernelManifests).To(HaveLen(1))
					Expect(cfgMapUnikernelManifests[0]).To(Equal("https://manifests.kraftkit.sh/index.yaml"))

					// Check if stdout is empty
					Expect(stdout.String()).To(BeEmpty())
				})
			})

			When("a config file was already presend, and the second unsourced link is duplicate", func() {
				BeforeEach(func() {
					unsourceArgs := make([]string, len(cmd.Args))
					copy(unsourceArgs, cmd.Args)

					for idx, arg := range cmd.Args {
						if arg == "unsource" {
							cmd.Args[idx] = "source"
							break
						}
					}
					cmd.Args = append(cmd.Args, "https://example1.com")
					cmd.Args = append(cmd.Args, "https://example2.com")
					cmd.Args = append(cmd.Args, "https://example3.com")
					err := cmd.Run()
					if err != nil {
						fmt.Printf("Error running command, dumping output:\n%s\n%s\n%s\n", err, stderr, stdout)
					}
					Expect(err).ToNot(HaveOccurred())
					Expect(stderr.String()).To(BeEmpty())

					// Recreate the command to reuse
					stdout = fcmd.NewIOStream()
					stderr = fcmd.NewIOStream()

					cmd = fcmd.NewKraft(stdout, stderr, cfg.Path())
					cmd.Args = unsourceArgs
				})

				It("should remove the first two links and print a warning", func() {
					cmd.Args = append(cmd.Args, "https://example1.com")
					cmd.Args = append(cmd.Args, "https://example2.com")
					cmd.Args = append(cmd.Args, "https://example2.com")
					err := cmd.Run()
					if err != nil {
						fmt.Printf("Error running command, dumping output:\n%s\n%s\n%s\n", err, stderr, stdout)
					}
					Expect(err).ToNot(HaveOccurred())
					Expect(stderr.String()).To(BeEmpty())

					// Read the config file
					osFile, err := os.Open(cfg.Path())
					if err != nil {
						fmt.Printf("Error running command, dumping output:\n%s\n%s\n%s\n", err, stderr, stdout)
					}
					Expect(err).ToNot(HaveOccurred())
					defer osFile.Close()

					osFileInfo, err := osFile.Stat()
					if err != nil {
						fmt.Printf("Error running command, dumping output:\n%s\n%s\n%s\n", err, stderr, stdout)
					}
					Expect(err).ToNot(HaveOccurred())

					// Check if the config file is not empty
					Expect(osFileInfo.Size()).To(BeNumerically(">", 0))

					// Check if the config file has 600 permissions
					Expect(osFileInfo.Mode().Perm()).To(Equal(os.FileMode(0o600)))

					// Read file content
					readBytes, err := os.ReadFile(cfg.Path())
					if err != nil {
						fmt.Printf("Error running command, dumping output:\n%s\n%s\n%s\n", err, stderr, stdout)
					}
					Expect(err).ToNot(HaveOccurred())

					// Marshal the yaml file into a map
					var cfgMap map[string]interface{}
					err = yaml.Unmarshal([]byte(readBytes), &cfgMap)
					if err != nil {
						fmt.Printf("Error running command, dumping output:\n%s\n%s\n%s\n", err, stderr, stdout)
					}
					Expect(err).ToNot(HaveOccurred())

					// Check if the config file contains the default manifests
					cfgMapUnikernel, ok := cfgMap["unikraft"].(map[string]interface{})
					Expect(ok).To(BeTrue())

					// Cast the manifests list to an array
					cfgMapUnikernelManifests, ok := cfgMapUnikernel["manifests"].([]interface{})
					Expect(ok).To(BeTrue())

					// Check if the default manifest and the third link is still there
					Expect(cfgMapUnikernelManifests).To(HaveLen(2))
					Expect(cfgMapUnikernelManifests[0]).To(Equal("https://manifests.kraftkit.sh/index.yaml"))
					Expect(cfgMapUnikernelManifests[1]).To(Equal("https://example3.com"))

					// Check if stdout has a warning
					Expect(stdout.String()).To(MatchRegexp(`^{"level":"warning","msg":"manifest not found: https://example2\.com"}\n$`))
				})
			})
		})
	})
})
