// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package hyperlight_test

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2" //nolint:stylecheck
	. "github.com/onsi/gomega"    //nolint:stylecheck

	fcmd "kraftkit.sh/test/e2e/framework/cmd"
	fcfg "kraftkit.sh/test/e2e/framework/config"
)

const (
	runMemory = "64Mi"
	httpPort  = "8080:8080"

	buildTimeout   = 20 * time.Minute
	runtimeTimeout = 3 * time.Minute
)

var _ = Describe("kraft hyperlight go-http", Label("runtime"), Ordered, func() {
	var projectDir string
	var mountDir string
	var kernelPath string
	var initrdPath string
	var cfg *fcfg.Config

	BeforeAll(func(_ SpecContext) {
		SkipUnlessEnvReady()

		stdout := fcmd.NewIOStream()
		stderr := fcmd.NewIOStream()
		cfg = fcfg.NewTempConfig()

		var err error
		projectDir, err = os.MkdirTemp("", "kraftkit-hl-project-*")
		Expect(err).ToNot(HaveOccurred())
		DeferCleanup(func() error {
			return os.RemoveAll(projectDir)
		})

		Expect(WriteProject(projectDir)).To(Succeed())

		mountDir = filepath.Join(projectDir, "mount")

		build := newKraft(stdout, stderr, cfg)
		isolateHome(build, cfg)
		build.Dir = projectDir
		build.Args = append(build.Args,
			"build",
			"--plat", "hyperlight",
			"--arch", "x86_64",
		)

		err = build.Run()
		if err != nil {
			fmt.Print(build.DumpError(stdout, stderr, err))
		}
		Expect(err).ToNot(HaveOccurred())

		kernelPath, initrdPath = ArtifactPaths(projectDir)
		Expect(kernelPath).To(BeAnExistingFile())
		Expect(initrdPath).To(BeAnExistingFile())
	}, NodeTimeout(buildTimeout))

	runArgs := func(extra ...string) []string {
		args := []string{
			"run",
			"--plat", "hyperlight",
			"--memory", runMemory,
			"--volume", fmt.Sprintf("%s:/mnt", mountDir),
			"--hyperlight-net",
			"--port", httpPort,
			"--rootfs", initrdPath,
			kernelPath,
			"--as=kernel",
		}
		return append(args, extra...)
	}

	It("should serve HTTP from the guest and share a host volume", func(_ SpecContext) {
		const instanceName = "hl-go-http-serve"
		const mountPostBody = "volume-write-payload"

		reqPath := filepath.Join(mountDir, "req.txt")
		resPath := filepath.Join(mountDir, "res.txt")
		Expect(resPath).To(BeAnExistingFile())

		stdout := fcmd.NewIOStream()
		stderr := fcmd.NewIOStream()

		run := newKraft(stdout, stderr, cfg)
		isolateHome(run, cfg)
		run.Dir = projectDir
		run.Args = append(run.Args, runArgs("-d", "--name", instanceName)...)

		err := run.Run()
		if err != nil {
			fmt.Print(run.DumpError(stdout, stderr, err))
		}
		Expect(err).ToNot(HaveOccurred())

		DeferCleanup(func() {
			rm := newKraft(fcmd.NewIOStream(), fcmd.NewIOStream(), cfg)
			isolateHome(rm, cfg)
			rm.Args = append(rm.Args, "rm", instanceName)
			_ = rm.Run()
		})

		// Check that the instance is running and logs are available.
		Eventually(func(g Gomega) {
			psOut := fcmd.NewIOStream()
			psErr := fcmd.NewIOStream()
			ps := newKraft(psOut, psErr, cfg)
			isolateHome(ps, cfg)
			ps.Args = append(ps.Args, "ps", "--plat", "hyperlight")
			g.Expect(ps.Run()).To(Succeed())
			g.Expect(psOut.String()).To(ContainSubstring(instanceName))
		}, 30*time.Second, 1*time.Second).Should(Succeed())

		// Check that the instance logs contain the expected output.
		Eventually(func(g Gomega) {
			logsOut := fcmd.NewIOStream()
			logsErr := fcmd.NewIOStream()
			logs := newKraft(logsOut, logsErr, cfg)
			isolateHome(logs, cfg)
			logs.Args = append(logs.Args, "logs", instanceName)
			g.Expect(logs.Run()).To(Succeed())
			g.Expect(logsOut.String()).To(ContainSubstring("Server is running"))
		}, 60*time.Second, 2*time.Second).Should(Succeed())

		// Volume read: guest GET serves the host-mounted res.txt.
		Eventually(func(g Gomega) {
			curlOut := fcmd.NewIOStream()
			curlErr := fcmd.NewIOStream()
			curl := fcmd.NewCurl(curlOut, curlErr)
			curl.Args = append(curl.Args, "--max-time", "2", "http://localhost:8080/")
			err := curl.Run()
			g.Expect(err).ToNot(HaveOccurred(), "curl stderr: %s", curlErr.String())

			mountResBody, err := os.ReadFile(resPath)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(curlOut.String()).To(ContainSubstring(string(mountResBody)))
		}, 90*time.Second, 1*time.Second).Should(Succeed())

		// Volume write: guest POST writes the body to host-mounted req.txt.
		Eventually(func(g Gomega) {
			curlOut := fcmd.NewIOStream()
			curlErr := fcmd.NewIOStream()
			curl := fcmd.NewCurl(curlOut, curlErr)
			curl.Args = append(curl.Args,
				"--max-time", "2",
				"-X", "POST",
				"--data-binary", mountPostBody,
				"http://localhost:8080/",
			)
			err := curl.Run()
			g.Expect(err).ToNot(HaveOccurred(), "curl stderr: %s", curlErr.String())
			g.Expect(curlOut.String()).To(ContainSubstring("ok"))
		}, 90*time.Second, 1*time.Second).Should(Succeed())

		// Check that the POST request was written to the host-mounted req.txt.
		Eventually(func(g Gomega) {
			data, err := os.ReadFile(reqPath)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(string(data)).To(Equal(mountPostBody))
		}, 10*time.Second, 200*time.Millisecond).Should(Succeed())
	}, SpecTimeout(buildTimeout+runtimeTimeout))

	It("should manage detached machine lifecycle", func(_ SpecContext) {
		const instanceName = "hl-go-http-lifecycle"

		stdout := fcmd.NewIOStream()
		stderr := fcmd.NewIOStream()

		run := newKraft(stdout, stderr, cfg)
		isolateHome(run, cfg)
		run.Dir = projectDir
		run.Args = append(run.Args, runArgs("-d", "--name", instanceName)...)

		err := run.Run()
		if err != nil {
			fmt.Print(run.DumpError(stdout, stderr, err))
		}
		Expect(err).ToNot(HaveOccurred())

		DeferCleanup(func() {
			rm := newKraft(fcmd.NewIOStream(), fcmd.NewIOStream(), cfg)
			isolateHome(rm, cfg)
			rm.Args = append(rm.Args, "rm", instanceName)
			_ = rm.Run()
		})

		psOut := fcmd.NewIOStream()
		psErr := fcmd.NewIOStream()
		ps := newKraft(psOut, psErr, cfg)
		isolateHome(ps, cfg)
		ps.Args = append(ps.Args, "ps", "--plat", "hyperlight")

		err = ps.Run()
		if err != nil {
			fmt.Print(ps.DumpError(psOut, psErr, err))
		}
		Expect(err).ToNot(HaveOccurred())
		Expect(psOut.String()).To(ContainSubstring(instanceName))

		logsOut := fcmd.NewIOStream()
		logsErr := fcmd.NewIOStream()
		logs := newKraft(logsOut, logsErr, cfg)
		isolateHome(logs, cfg)
		logs.Args = append(logs.Args, "logs", instanceName)

		err = logs.Run()
		if err != nil {
			fmt.Print(logs.DumpError(logsOut, logsErr, err))
		}
		Expect(err).ToNot(HaveOccurred())

		stopOut := fcmd.NewIOStream()
		stopErr := fcmd.NewIOStream()
		stop := newKraft(stopOut, stopErr, cfg)
		isolateHome(stop, cfg)
		stop.Args = append(stop.Args, "stop", instanceName)

		err = stop.Run()
		if err != nil {
			fmt.Print(stop.DumpError(stopOut, stopErr, err))
		}
		Expect(err).ToNot(HaveOccurred())

		// After stop, the instance should not appear as running.
		psAfterOut := fcmd.NewIOStream()
		psAfterErr := fcmd.NewIOStream()
		psAfter := newKraft(psAfterOut, psAfterErr, cfg)
		isolateHome(psAfter, cfg)
		psAfter.Args = append(psAfter.Args, "ps", "--plat", "hyperlight")
		err = psAfter.Run()
		if err != nil {
			fmt.Print(psAfter.DumpError(psAfterOut, psAfterErr, err))
		}
		Expect(err).ToNot(HaveOccurred())
		Expect(psAfterOut.String()).ToNot(ContainSubstring(instanceName))

		rmOut := fcmd.NewIOStream()
		rmErr := fcmd.NewIOStream()
		rm := newKraft(rmOut, rmErr, cfg)
		isolateHome(rm, cfg)
		rm.Args = append(rm.Args, "rm", instanceName)
		err = rm.Run()
		if err != nil {
			fmt.Print(rm.DumpError(rmOut, rmErr, err))
		}
		Expect(err).ToNot(HaveOccurred())
	}, SpecTimeout(2*time.Minute))
})
