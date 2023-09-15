// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package cli_test

import (
	"crypto/sha1"
	"os"

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
			cmd.Args = append(cmd.Args, "update", "--log-level", "info", "--log-type", "json")

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
					Expect(stdout.String()).To(MatchRegexp(`^{"level":"info","msg":"Updating..."}\n$`))

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
					Expect(stdout.String()).To(MatchRegexp(`^Retrieve new lists of Unikraft components, libraries and packages.\n`))
				})
			})
		})
	})

	_ = Describe("source", func() {
		BeforeEach(func() {
			cmd.Args = append(cmd.Args, "source", "--no-check-updates", "--log-level", "info", "--log-type", "json")
		})

		Context("sourcing a new link in the config file", func() {
			BeforeEach(func() {
				// Save the config file by moving it to a temporary location
				os.Rename(cfg.Path(), cfg.Path()+".tmp")
			})

			AfterEach(func() {
				// Restore the config file
				os.Rename(cfg.Path()+".tmp", cfg.Path())
			})
			When("kraftkit was just installed", func() {
				It("should create the config file, add the default manifests, and the new link, and print nothing", func() {
					cmd.Args = append(cmd.Args, "https://example.com")
					err := cmd.Run()

					Expect(err).ToNot(HaveOccurred())
					Expect(stderr.String()).To(BeEmpty())

					// Check if the config file was created
					Expect(cfg.Path()).To(BeAnExistingFile())

					// Read the config file
					osFile, err := os.Open(cfg.Path())
					Expect(err).ToNot(HaveOccurred())
					defer osFile.Close()

					osFileInfo, err := osFile.Stat()
					Expect(err).ToNot(HaveOccurred())

					// Check if the config file is not empty
					Expect(osFileInfo.Size()).To(BeNumerically(">", 0))

					// Check if the config file has 600 permissions
					Expect(osFileInfo.Mode().Perm()).To(Equal(os.FileMode(0600)))

					// Read file content
					readBytes, err := os.ReadFile(cfg.Path())
					Expect(err).ToNot(HaveOccurred())

					// Marshal the yaml file into a map
					var cfgMap map[string]interface{}
					err = yaml.Unmarshal([]byte(readBytes), &cfgMap)
					Expect(err).ToNot(HaveOccurred())

					// Check if the config file contains the default manifests
					cfgMapUnikernel, ok := cfgMap["unikraft"].(map[string]interface{})
					Expect(ok).To(BeTrue())

					// Cast the manifests list to an array
					cfgMapUnikernelManifests, ok := cfgMapUnikernel["manifests"].([]interface{})
					Expect(ok).To(BeTrue())

					// Check if the default manifests are present and in the first position
					Expect(cfgMapUnikernelManifests).To(HaveLen(2))
					Expect(cfgMapUnikernelManifests[0]).To(Equal("https://manifests.kraftkit.sh/index.yaml"))

					// Check if the config file contains the new link
					Expect(cfgMapUnikernelManifests[1]).To(Equal("https://example.com"))

					// Check if stdout is empty
					Expect(stdout.String()).To(BeEmpty())
				})
			})

			When("kraftkit was already present with a config file", func() {
				BeforeEach(func() {
					cmd.Args = append(cmd.Args, "https://example1.com")
					err := cmd.Run()

					Expect(err).ToNot(HaveOccurred())
					Expect(stderr.String()).To(BeEmpty())

					// Replace the last argument
					cmd.Args = cmd.Args[:len(cmd.Args)-1]
					cmd.Args = append(cmd.Args, "https://example2.com")

					oldArgs := cmd.Args

					// Recreate the command to reuse
					stdout = fcmd.NewIOStream()
					stderr = fcmd.NewIOStream()

					cmd = fcmd.NewKraft(stdout, stderr, cfg.Path())
					cmd.Args = oldArgs
				})

				It("should leave the config file intact, add the new link, and print nothing", func() {
					err := cmd.Run()

					Expect(err).ToNot(HaveOccurred())
					Expect(stderr.String()).To(BeEmpty())

					// Check if the config file was created
					Expect(cfg.Path()).To(BeAnExistingFile())

					// Read the config file
					osFile, err := os.Open(cfg.Path())
					Expect(err).ToNot(HaveOccurred())
					defer osFile.Close()

					osFileInfo, err := osFile.Stat()
					Expect(err).ToNot(HaveOccurred())

					// Check if the config file is not empty
					Expect(osFileInfo.Size()).To(BeNumerically(">", 0))

					// Check if the config file has 600 permissions
					Expect(osFileInfo.Mode().Perm()).To(Equal(os.FileMode(0600)))

					// Read file content
					readBytes, err := os.ReadFile(cfg.Path())
					Expect(err).ToNot(HaveOccurred())

					// Marshal the yaml file into a map
					var cfgMap map[string]interface{}
					err = yaml.Unmarshal([]byte(readBytes), &cfgMap)
					Expect(err).ToNot(HaveOccurred())

					// Check if the config file contains the default manifests
					cfgMapUnikernel, ok := cfgMap["unikraft"].(map[string]interface{})
					Expect(ok).To(BeTrue())

					// Cast the manifests list to an array
					cfgMapUnikernelManifests, ok := cfgMapUnikernel["manifests"].([]interface{})
					Expect(ok).To(BeTrue())

					// Check if the default manifests are present and in the first position
					Expect(cfgMapUnikernelManifests).To(HaveLen(3))
					Expect(cfgMapUnikernelManifests[0]).To(Equal("https://manifests.kraftkit.sh/index.yaml"))

					// Check if the config file contains the new link
					Expect(cfgMapUnikernelManifests[1]).To(Equal("https://example1.com"))
					Expect(cfgMapUnikernelManifests[2]).To(Equal("https://example2.com"))

					// Check if stdout is empty
					Expect(stdout.String()).To(BeEmpty())
				})
			})

			When("sourcing an existing link in the config file", func() {
				It("warn the user and leave the file intact", func() {
					// Generate a clean config file
					cmd.Args = append(cmd.Args, "https://manifests.kraftkit.sh/index.yaml")
					err := cmd.Run()

					Expect(err).ToNot(HaveOccurred())
					Expect(stderr.String()).To(BeEmpty())

					// Calculate config file hash
					bytes, err := os.ReadFile(cfg.Path())
					Expect(err).ToNot(HaveOccurred())

					// Calculate sha1 hash of bytes
					sha1Hash := sha1.Sum(bytes)

					cmd.Args = append(cmd.Args, "https://manifests.kraftkit.sh/index.yaml")
					err = cmd.Run()

					Expect(err).ToNot(HaveOccurred())
					Expect(stderr.String()).To(BeEmpty())

					// Check warning message exists in stdout
					Expect(stdout.String()).To(MatchRegexp(`^{"level":"warning","msg":"manifest already saved: https://manifests\.kraftkit\.sh/index\.yaml"}\n$`))

					// Check if the config file was not modified
					newBytes, err := os.ReadFile(cfg.Path())
					Expect(err).ToNot(HaveOccurred())

					newSha1Hash := sha1.Sum(newBytes)
					Expect(newSha1Hash).To(Equal(sha1Hash))
				})
			})
		})

		Context("sourcing multiple links in the config file", func() {
			BeforeEach(func() {
				// Save the config file by moving it to a temporary location
				os.Rename(cfg.Path(), cfg.Path()+".tmp")
			})

			AfterEach(func() {
				// Restore the config file
				os.Rename(cfg.Path()+".tmp", cfg.Path())
			})
			When("kraftkit was already present, and all links are unique", func() {
				It("should add all links and print nothing", func() {
					cmd.Args = append(cmd.Args, "https://example1.com")
					cmd.Args = append(cmd.Args, "https://example2.com")
					cmd.Args = append(cmd.Args, "https://example3.com")
					err := cmd.Run()

					Expect(err).ToNot(HaveOccurred())
					Expect(stderr.String()).To(BeEmpty())

					// Check if the config file was created
					Expect(cfg.Path()).To(BeAnExistingFile())

					// Read the config file
					osFile, err := os.Open(cfg.Path())
					Expect(err).ToNot(HaveOccurred())
					defer osFile.Close()

					osFileInfo, err := osFile.Stat()
					Expect(err).ToNot(HaveOccurred())

					// Check if the config file is not empty
					Expect(osFileInfo.Size()).To(BeNumerically(">", 0))

					// Check if the config file has 600 permissions
					Expect(osFileInfo.Mode().Perm()).To(Equal(os.FileMode(0600)))

					// Read file content
					readBytes, err := os.ReadFile(cfg.Path())
					Expect(err).ToNot(HaveOccurred())

					// Marshal the yaml file into a map
					var cfgMap map[string]interface{}
					err = yaml.Unmarshal([]byte(readBytes), &cfgMap)
					Expect(err).ToNot(HaveOccurred())

					// Check if the config file contains the default manifests
					cfgMapUnikernel, ok := cfgMap["unikraft"].(map[string]interface{})
					Expect(ok).To(BeTrue())

					// Cast the manifests list to an array
					cfgMapUnikernelManifests, ok := cfgMapUnikernel["manifests"].([]interface{})
					Expect(ok).To(BeTrue())

					// Check if the default manifests are present and in the first position
					Expect(cfgMapUnikernelManifests).To(HaveLen(4))
					Expect(cfgMapUnikernelManifests[0]).To(Equal("https://manifests.kraftkit.sh/index.yaml"))

					// Check if the config file contains the new links
					Expect(cfgMapUnikernelManifests[1]).To(Equal("https://example1.com"))
					Expect(cfgMapUnikernelManifests[2]).To(Equal("https://example2.com"))
					Expect(cfgMapUnikernelManifests[3]).To(Equal("https://example3.com"))

					// Check if stdout is empty
					Expect(stdout.String()).To(BeEmpty())
				})
			})

			When("kraftkit was already present, and a link is duplicate", func() {
				It("should add links until the first error is met", func() {
					cmd.Args = append(cmd.Args, "https://example.com")
					cmd.Args = append(cmd.Args, "https://example.com")
					cmd.Args = append(cmd.Args, "https://example2.com")
					err := cmd.Run()

					Expect(err).ToNot(HaveOccurred())
					Expect(stderr.String()).To(BeEmpty())

					// Check if the config file was created
					Expect(cfg.Path()).To(BeAnExistingFile())

					// Read the config file
					osFile, err := os.Open(cfg.Path())
					Expect(err).ToNot(HaveOccurred())
					defer osFile.Close()

					osFileInfo, err := osFile.Stat()
					Expect(err).ToNot(HaveOccurred())

					// Check if the config file is not empty
					Expect(osFileInfo.Size()).To(BeNumerically(">", 0))

					// Check if the config file has 600 permissions
					Expect(osFileInfo.Mode().Perm()).To(Equal(os.FileMode(0600)))

					// Read file content
					readBytes, err := os.ReadFile(cfg.Path())
					Expect(err).ToNot(HaveOccurred())

					// Marshal the yaml file into a map
					var cfgMap map[string]interface{}
					err = yaml.Unmarshal([]byte(readBytes), &cfgMap)
					Expect(err).ToNot(HaveOccurred())

					// Check if the config file contains the default manifests
					cfgMapUnikernel, ok := cfgMap["unikraft"].(map[string]interface{})
					Expect(ok).To(BeTrue())

					// Cast the manifests list to an array
					cfgMapUnikernelManifests, ok := cfgMapUnikernel["manifests"].([]interface{})
					Expect(ok).To(BeTrue())

					// Check if the default manifests are present and in the first position
					Expect(cfgMapUnikernelManifests).To(HaveLen(2))
					Expect(cfgMapUnikernelManifests[0]).To(Equal("https://manifests.kraftkit.sh/index.yaml"))

					// Check if the config file contains the new link
					Expect(cfgMapUnikernelManifests[1]).To(Equal("https://example.com"))

					// Check if stdout contains the warning message
					Expect(stdout.String()).To(MatchRegexp(`^{"level":"warning","msg":"manifest already saved: https://example\.com"}\n$`))
				})
			})
		})
	})

	_ = Describe("unsource", func() {
		BeforeEach(func() {
			cmd.Args = append(cmd.Args, "unsource", "--no-check-updates", "--log-level", "info", "--log-type", "json")
		})

		Context("unsourcing a link in the config file", func() {
			BeforeEach(func() {
				// Save the config file by moving it to a temporary location
				os.Rename(cfg.Path(), cfg.Path()+".tmp")
			})

			AfterEach(func() {
				// Restore the config file
				os.Rename(cfg.Path()+".tmp", cfg.Path())
			})
			When("kraftkit was already present with a config file", func() {
				BeforeEach(func() {
					cmd.Args = append(cmd.Args, "https://example.com")
					err := cmd.Run()

					Expect(err).ToNot(HaveOccurred())
					Expect(stderr.String()).To(BeEmpty())

					// Replace the last argument
					cmd.Args = cmd.Args[:len(cmd.Args)-1]

					oldArgs := cmd.Args

					// Recreate the command to reuse
					stdout = fcmd.NewIOStream()
					stderr = fcmd.NewIOStream()

					cmd = fcmd.NewKraft(stdout, stderr, cfg.Path())
					cmd.Args = oldArgs
				})

				It("should remove the link and print nothing", func() {
					cmd.Args = append(cmd.Args, "https://manifests.kraftkit.sh/index.yaml")
					err := cmd.Run()

					Expect(err).ToNot(HaveOccurred())
					Expect(stderr.String()).To(BeEmpty())

					// Check if the config file was created
					Expect(cfg.Path()).To(BeAnExistingFile())

					// Read the config file
					osFile, err := os.Open(cfg.Path())
					Expect(err).ToNot(HaveOccurred())
					defer osFile.Close()

					osFileInfo, err := osFile.Stat()
					Expect(err).ToNot(HaveOccurred())

					// Check if the config file is not empty
					Expect(osFileInfo.Size()).To(BeNumerically(">", 0))

					// Check if the config file has 600 permissions
					Expect(osFileInfo.Mode().Perm()).To(Equal(os.FileMode(0600)))

					// Read file content
					readBytes, err := os.ReadFile(cfg.Path())
					Expect(err).ToNot(HaveOccurred())

					// Marshal the yaml file into a map
					var cfgMap map[string]interface{}
					err = yaml.Unmarshal([]byte(readBytes), &cfgMap)
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

			When("kraftkit was just installed and there's no config file", func() {
				It("should create the config file, add the default manifests, remove the link, and print nothing", func() {
					cmd.Args = append(cmd.Args, "https://manifests.kraftkit.sh/index.yaml")
					err := cmd.Run()

					Expect(err).ToNot(HaveOccurred())
					Expect(stderr.String()).To(BeEmpty())

					// Check if the config file was created
					Expect(cfg.Path()).To(BeAnExistingFile())

					// Read the config file
					osFile, err := os.Open(cfg.Path())
					Expect(err).ToNot(HaveOccurred())
					defer osFile.Close()

					osFileInfo, err := osFile.Stat()
					Expect(err).ToNot(HaveOccurred())

					// Check if the config file is not empty
					Expect(osFileInfo.Size()).To(BeNumerically(">", 0))

					// Check if the config file has 600 permissions
					Expect(osFileInfo.Mode().Perm()).To(Equal(os.FileMode(0600)))

					// Read file content
					readBytes, err := os.ReadFile(cfg.Path())
					Expect(err).ToNot(HaveOccurred())

					// Marshal the yaml file into a map
					var cfgMap map[string]interface{}
					err = yaml.Unmarshal([]byte(readBytes), &cfgMap)
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

			When("the config does not contain the link", func() {
				BeforeEach(func() {
					cmd.Args = append(cmd.Args, "https://example.com")
					err := cmd.Run()

					Expect(err).ToNot(HaveOccurred())
					Expect(stderr.String()).To(BeEmpty())

					// Replace the last argument
					cmd.Args = cmd.Args[:len(cmd.Args)-1]

					oldArgs := cmd.Args

					// Recreate the command to reuse
					stdout = fcmd.NewIOStream()
					stderr = fcmd.NewIOStream()

					cmd = fcmd.NewKraft(stdout, stderr, cfg.Path())
					cmd.Args = oldArgs
				})

				It("should do nothing and print a warning", func() {
					cmd.Args = append(cmd.Args, "https://example.com")
					err := cmd.Run()

					Expect(err).ToNot(HaveOccurred())
					Expect(stderr.String()).To(BeEmpty())

					// Check if the config file was created
					Expect(cfg.Path()).To(BeAnExistingFile())

					// Read the config file
					osFile, err := os.Open(cfg.Path())
					Expect(err).ToNot(HaveOccurred())
					defer osFile.Close()

					osFileInfo, err := osFile.Stat()
					Expect(err).ToNot(HaveOccurred())

					// Check if the config file is not empty
					Expect(osFileInfo.Size()).To(BeNumerically(">", 0))

					// Check if the config file has 600 permissions
					Expect(osFileInfo.Mode().Perm()).To(Equal(os.FileMode(0600)))

					// Read file content
					readBytes, err := os.ReadFile(cfg.Path())
					Expect(err).ToNot(HaveOccurred())

					// Marshal the yaml file into a map
					var cfgMap map[string]interface{}
					err = yaml.Unmarshal([]byte(readBytes), &cfgMap)
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
			BeforeEach(func() {
				// Save the config file by moving it to a temporary location
				os.Rename(cfg.Path(), cfg.Path()+".tmp")
			})

			AfterEach(func() {
				// Restore the config file
				os.Rename(cfg.Path()+".tmp", cfg.Path())
			})
			When("kraftkit was already present with a config file, and all links are unique", func() {
				BeforeEach(func() {
					// This is not a deep copy, it's a reference
					unsourceArgs := cmd.Args

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

					// Remove last 3 arguments and revert change
					cmd.Args = cmd.Args[:len(cmd.Args)-3]

					Expect(err).ToNot(HaveOccurred())
					Expect(stderr.String()).To(BeEmpty())

					// Recreate the command to reuse
					stdout = fcmd.NewIOStream()
					stderr = fcmd.NewIOStream()

					cmd = fcmd.NewKraft(stdout, stderr, cfg.Path())
					cmd.Args = unsourceArgs
					for idx, arg := range cmd.Args {
						if arg == "source" {
							cmd.Args[idx] = "unsource"
							break
						}
					}
				})

				It("should remove the links and print nothing", func() {
					cmd.Args = append(cmd.Args, "https://example1.com")
					cmd.Args = append(cmd.Args, "https://example2.com")
					cmd.Args = append(cmd.Args, "https://example3.com")
					err := cmd.Run()

					Expect(err).ToNot(HaveOccurred())
					Expect(stderr.String()).To(BeEmpty())

					// Check if the config file was created
					Expect(cfg.Path()).To(BeAnExistingFile())

					// Read the config file
					osFile, err := os.Open(cfg.Path())
					Expect(err).ToNot(HaveOccurred())
					defer osFile.Close()

					osFileInfo, err := osFile.Stat()
					Expect(err).ToNot(HaveOccurred())

					// Check if the config file is not empty
					Expect(osFileInfo.Size()).To(BeNumerically(">", 0))

					// Check if the config file has 600 permissions
					Expect(osFileInfo.Mode().Perm()).To(Equal(os.FileMode(0600)))

					// Read file content
					readBytes, err := os.ReadFile(cfg.Path())
					Expect(err).ToNot(HaveOccurred())

					// Marshal the yaml file into a map
					var cfgMap map[string]interface{}
					err = yaml.Unmarshal([]byte(readBytes), &cfgMap)
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

			When("kraftkit was already present with a config file, and the second link is duplicate", func() {
				BeforeEach(func() {
					// This is not a deep copy, it's a reference
					unsourceArgs := cmd.Args

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

					// Remove last 3 arguments
					cmd.Args = cmd.Args[:len(cmd.Args)-3]

					Expect(err).ToNot(HaveOccurred())
					Expect(stderr.String()).To(BeEmpty())

					// Recreate the command to reuse
					stdout = fcmd.NewIOStream()
					stderr = fcmd.NewIOStream()

					cmd = fcmd.NewKraft(stdout, stderr, cfg.Path())
					cmd.Args = unsourceArgs
					for idx, arg := range cmd.Args {
						if arg == "source" {
							cmd.Args[idx] = "unsource"
							break
						}
					}
				})

				It("should remove the first two links and print a warning", func() {
					cmd.Args = append(cmd.Args, "https://example1.com")
					cmd.Args = append(cmd.Args, "https://example2.com")
					cmd.Args = append(cmd.Args, "https://example2.com")
					err := cmd.Run()

					Expect(err).ToNot(HaveOccurred())
					Expect(stderr.String()).To(BeEmpty())

					// Check if the config file was created
					Expect(cfg.Path()).To(BeAnExistingFile())

					// Read the config file
					osFile, err := os.Open(cfg.Path())
					Expect(err).ToNot(HaveOccurred())
					defer osFile.Close()

					osFileInfo, err := osFile.Stat()
					Expect(err).ToNot(HaveOccurred())

					// Check if the config file is not empty
					Expect(osFileInfo.Size()).To(BeNumerically(">", 0))

					// Check if the config file has 600 permissions
					Expect(osFileInfo.Mode().Perm()).To(Equal(os.FileMode(0600)))

					// Read file content
					readBytes, err := os.ReadFile(cfg.Path())
					Expect(err).ToNot(HaveOccurred())

					// Marshal the yaml file into a map
					var cfgMap map[string]interface{}
					err = yaml.Unmarshal([]byte(readBytes), &cfgMap)
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
