// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package hyperlight_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2" //nolint:stylecheck
	. "github.com/onsi/gomega"    //nolint:stylecheck

	fcmd "kraftkit.sh/test/e2e/framework/cmd"
	fcfg "kraftkit.sh/test/e2e/framework/config"
)

var _ = Describe("kraft hyperlight run validation", Label("validation"), func() {
	var cmd *fcmd.Cmd
	var stdout *fcmd.IOStream
	var stderr *fcmd.IOStream
	var cfg *fcfg.Config

	BeforeEach(func() {
		skipUnlessLinux()

		stdout = fcmd.NewIOStream()
		stderr = fcmd.NewIOStream()
		cfg = fcfg.NewTempConfig()
		cmd = newKraft(stdout, stderr, cfg)
		isolateHome(cmd, cfg)
	})

	When("invoked with --help", func() {
		BeforeEach(func() {
			cmd.Args = append(cmd.Args, "run", "--help")
		})

		It("should list hyperlight-specific flags", func() {
			err := cmd.Run()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())
			Expect(stdout.String()).To(ContainSubstring("--hyperlight-stack"))
			Expect(stdout.String()).To(ContainSubstring("--hyperlight-repeat"))
			Expect(stdout.String()).To(ContainSubstring("--hyperlight-net"))
		})
	})
})
