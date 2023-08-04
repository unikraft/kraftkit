// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package buildkit

import (
	"fmt"
	"os/exec"
	"testing"

	e2eutil "kraftkit.sh/tools/dockerfile-llb-frontend/test"
)

type BuildKitDriver struct {
	e2eutil.CmdExec
}

func (driver BuildKitDriver) Run(localRegistry string, pluginImage string, app string, t *testing.T) ([]byte, error) {
	if err := driver.checkBuildkitWorkersAvailability(); err != nil {
		// We skip this suite if the buildkit daemon is not available.
		// The docker end to end test suite calls buildkit indirectly anyway.
		// Running this in CI adds a lot of complexity and doesn't test
		// anything the docker end to end suite doesn't cover.
		// The buildkit suite is useful when you want to run your own buildkit daemon
		// with a debugger to step through the LLB solving steps.
		t.Skipf("buildkit daemon not ready for testing: %v", err)
	}
	// Define frontend image name
	frontendImage := fmt.Sprintf("%s/%s", localRegistry, pluginImage)

	clonedAppPath, err := e2eutil.ShallowCloneToTempDir(t, app)
	if err != nil {
		return nil, fmt.Errorf("failed cloning app: %w", err)
	}

	out, err := driver.Exec("buildctl", "build", "--frontend", "gateway.v0", "--opt", "source="+frontendImage, "--local", "context="+clonedAppPath)

	// Emit output for greater visibility
	t.Logf("%s", out)

	// Run buildctl command
	return out, err
}

// A pre-flight check to ensure we have the buildkit daemon running
// and that it does have workers available.
func (driver BuildKitDriver) checkBuildkitWorkersAvailability() error {
	cmd := exec.Command("buildctl", "debug", "workers")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("debug workers output %s :%w", output, err)
	}

	return nil
}

func TestBuildsWithBuildkit(t *testing.T) {
	e2eutil.RunWithDriver(BuildKitDriver{}, e2eutil.Apps, t, false)
}
