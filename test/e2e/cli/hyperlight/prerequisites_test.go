// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package hyperlight_test

import (
	"os"
	"os/exec"
	"runtime"

	. "github.com/onsi/ginkgo/v2" //nolint:stylecheck
)

func skipUnlessLinux() {
	if runtime.GOOS != "linux" {
		Skip("hyperlight e2e tests only support Linux")
	}
}

func SkipUnlessEnvReady() {
	skipUnlessLinux()

	f, err := os.OpenFile("/dev/kvm", os.O_RDWR, 0)
	if err != nil {
		Skip("hyperlight e2e tests require read/write access to /dev/kvm: " + err.Error())
	}
	_ = f.Close()

	if _, err := exec.LookPath("hyperlight-unikraft"); err != nil {
		Skip("hyperlight e2e tests require hyperlight-unikraft on $PATH")
	}

	if _, err := exec.LookPath("docker"); err != nil {
		Skip("hyperlight e2e tests require docker on $PATH for kraft build rootfs")
	}

	if err := exec.Command("docker", "info").Run(); err != nil {
		Skip("hyperlight e2e tests require a running Docker daemon for kraft build rootfs: " + err.Error())
	}
}
