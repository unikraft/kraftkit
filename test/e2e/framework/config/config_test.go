// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package config_test

import (
	. "github.com/onsi/ginkgo/v2" //nolint:stylecheck
	. "github.com/onsi/gomega"    //nolint:stylecheck

	"sigs.k8s.io/kustomize/kyaml/yaml"

	fcfg "kraftkit.sh/test/e2e/framework/config"
)

var _ = Describe("Config", func() {
	var cfg *fcfg.Config

	BeforeEach(func() {
		/*
			Generates

			  paths:
			    manifests: /tmp/kraftkit-e2e-abc123xyz890/manifests
			    plugins: /tmp/kraftkit-e2e-abc123xyz890/plugins
			    sources: /tmp/kraftkit-e2e-abc123xyz890/sources
		*/
		cfg = fcfg.NewTempConfig()
	})

	When("the default configuration is untouched", func() {
		It("contains valid YAML with pre-populated paths", func() {
			configDoc, err := yaml.ReadFile(cfg.Path())
			Expect(err).ToNot(HaveOccurred())

			fields, err := configDoc.Fields()
			Expect(err).ToNot(HaveOccurred())
			Expect(fields).To(HaveLen(1))
			Expect(fields[0]).To(Equal("paths"))

			paths := configDoc.Field("paths")
			Expect(paths).ToNot(BeNil())
			fields, err = paths.Value.Fields()
			Expect(err).ToNot(HaveOccurred())
			Expect(fields).To(HaveLen(3))
			Expect(fields[0]).To(Equal("manifests"))
			Expect(fields[1]).To(Equal("plugins"))
			Expect(fields[2]).To(Equal("sources"))
		})

		Specify("its attributes and values can be read", func() {
			yamlNode := cfg.Read("paths", "manifests")
			Expect(yamlNode.IsNilOrEmpty()).ToNot(BeTrue())
			Expect(yamlNode.IsStringValue()).To(BeTrue())
			Expect(yaml.GetValue(yamlNode)).To(MatchRegexp(`^\/([\w-]+\/)+manifests$`))
		})
	})

	When("configuration values are written to the file", func() {
		BeforeEach(func() {
			/*
				Generates

				  paths:
				    manifests: /tmp/kraftkit-e2e-abc123xyz890/manifests
				    plugins: /tmp/kraftkit-e2e-abc123xyz890/plugins
				    sources: /tmp/kraftkit-e2e-abc123xyz890/sources
				    nested: 42
				  non_nested: 42
			*/
			val := yaml.NewRNode(&yaml.Node{
				Kind:  yaml.ScalarNode,
				Tag:   "!!int",
				Value: "42",
			})

			cfg.Write(
				yaml.Tee(
					yaml.Get("paths"),
					yaml.SetField("nested", val),
				),
				yaml.SetField("non_nested", val),
			)
		})

		It("contains the expected YAML document", func() {
			configDoc, err := yaml.ReadFile(cfg.Path())
			Expect(err).ToNot(HaveOccurred())

			configStr, err := configDoc.String()
			Expect(err).ToNot(HaveOccurred())
			Expect(configStr).To(SatisfyAll(
				MatchRegexp(`\/sources\n  nested: 42`),
				MatchRegexp(`\nnon_nested: 42`),
			))
		})
	})
})
