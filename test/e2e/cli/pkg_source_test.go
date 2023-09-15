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

	_ = Describe("source", func() {
		BeforeEach(func() {
			cmd.Args = append(cmd.Args, "source", "--log-level", "info", "--log-type", "json")
		})

		Context("sourcing a new link in the config file", func() {
			When("the config file doesn't exist", func() {
				BeforeEach(func() {
					// Save the config file by moving it to a temporary location
					err := os.Rename(cfg.Path(), cfg.Path()+".tmp")
					Expect(err).ToNot(HaveOccurred())
				})

				AfterEach(func() {
					// Restore the config file
					err := os.Rename(cfg.Path()+".tmp", cfg.Path())
					Expect(err).ToNot(HaveOccurred())
				})
				It("should create the config file, add the default manifests, and the new link, and print nothing", func() {
					cmd.Args = append(cmd.Args, "https://example.com")
					err := cmd.Run()

					Expect(err).ToNot(HaveOccurred())
					Expect(stderr.String()).To(BeEmpty())

					// Read the config file
					osFile, err := os.Open(cfg.Path())
					Expect(err).ToNot(HaveOccurred())
					defer osFile.Close()

					osFileInfo, err := osFile.Stat()
					Expect(err).ToNot(HaveOccurred())

					// Check if the config file is not empty
					Expect(osFileInfo.Size()).To(BeNumerically(">", 0))

					// Check if the config file has 600 permissions
					Expect(osFileInfo.Mode().Perm()).To(Equal(os.FileMode(0o600)))

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

			When("the config file exists", func() {
				BeforeEach(func() {
					oldArgs := make([]string, len(cmd.Args))
					copy(oldArgs, cmd.Args)

					cmd.Args = append(cmd.Args, "https://example1.com")
					err := cmd.Run()

					Expect(err).ToNot(HaveOccurred())
					Expect(stderr.String()).To(BeEmpty())

					// Recreate the command to reuse
					stdout = fcmd.NewIOStream()
					stderr = fcmd.NewIOStream()

					cmd = fcmd.NewKraft(stdout, stderr, cfg.Path())
					cmd.Args = oldArgs
				})

				It("should leave the config file intact, add the new link, and print nothing", func() {
					cmd.Args = append(cmd.Args, "https://example2.com")
					err := cmd.Run()

					Expect(err).ToNot(HaveOccurred())
					Expect(stderr.String()).To(BeEmpty())

					// Read the config file
					osFile, err := os.Open(cfg.Path())
					Expect(err).ToNot(HaveOccurred())
					defer osFile.Close()

					osFileInfo, err := osFile.Stat()
					Expect(err).ToNot(HaveOccurred())

					// Check if the config file is not empty
					Expect(osFileInfo.Size()).To(BeNumerically(">", 0))

					// Check if the config file has 600 permissions
					Expect(osFileInfo.Mode().Perm()).To(Equal(os.FileMode(0o600)))

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
				It("should warn the user and leave the file intact", func() {
					oldArgs := make([]string, len(cmd.Args))
					copy(oldArgs, cmd.Args)

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

					// Recreate the command to reuse
					stdout = fcmd.NewIOStream()
					stderr = fcmd.NewIOStream()

					cmd = fcmd.NewKraft(stdout, stderr, cfg.Path())
					cmd.Args = oldArgs

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
			When("the config file was already present, and all links are unique", func() {
				It("should add all links and print nothing", func() {
					cmd.Args = append(cmd.Args, "https://example1.com")
					cmd.Args = append(cmd.Args, "https://example2.com")
					cmd.Args = append(cmd.Args, "https://example3.com")
					err := cmd.Run()

					Expect(err).ToNot(HaveOccurred())
					Expect(stderr.String()).To(BeEmpty())

					// Read the config file
					osFile, err := os.Open(cfg.Path())
					Expect(err).ToNot(HaveOccurred())
					defer osFile.Close()

					osFileInfo, err := osFile.Stat()
					Expect(err).ToNot(HaveOccurred())

					// Check if the config file is not empty
					Expect(osFileInfo.Size()).To(BeNumerically(">", 0))

					// Check if the config file has 600 permissions
					Expect(osFileInfo.Mode().Perm()).To(Equal(os.FileMode(0o600)))

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

			When("the config file was already present, and a link is duplicate", func() {
				It("should add links until the first error is met", func() {
					cmd.Args = append(cmd.Args, "https://example.com")
					cmd.Args = append(cmd.Args, "https://example.com")
					cmd.Args = append(cmd.Args, "https://example2.com")
					err := cmd.Run()

					Expect(err).ToNot(HaveOccurred())
					Expect(stderr.String()).To(BeEmpty())

					// Read the config file
					osFile, err := os.Open(cfg.Path())
					Expect(err).ToNot(HaveOccurred())
					defer osFile.Close()

					osFileInfo, err := osFile.Stat()
					Expect(err).ToNot(HaveOccurred())

					// Check if the config file is not empty
					Expect(osFileInfo.Size()).To(BeNumerically(">", 0))

					// Check if the config file has 600 permissions
					Expect(osFileInfo.Mode().Perm()).To(Equal(os.FileMode(0o600)))

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
})
