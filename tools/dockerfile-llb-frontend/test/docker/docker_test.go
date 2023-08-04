// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package docker

import (
	"fmt"
	"io"
	"os"
	"testing"

	e2eutil "kraftkit.sh/tools/dockerfile-llb-frontend/test"
)

type DockerDriver struct {
	e2eutil.CmdExec
	OmitPluginSelector bool // Useful for simulating negative test cases
}

func (driver DockerDriver) Run(localRegistry string, pluginImage string, app string, t *testing.T) ([]byte, error) {
	// We simply take the base app and prepend the kraft.yaml with a reference to our plugin.
	clonedAppPath, err := e2eutil.ShallowCloneToTempDir(t, app)
	if err != nil {
		return nil, fmt.Errorf("failed cloning app: %w", err)
	}

	// Construct the syntax reference for docker build.
	frontendImage := fmt.Sprintf("#syntax=%s/%s", localRegistry, pluginImage)

	// We make this configurable to enable negative test cases.
	if !driver.OmitPluginSelector {
		PrependToFile(clonedAppPath+"/kraft.yaml", frontendImage)
	}

	out, err := driver.DockerExec("build", "-f", fmt.Sprintf("%s/kraft.yaml", clonedAppPath), clonedAppPath)

	// Emit output for greater visibility
	t.Logf("%s", out)

	return out, err
}

// Happy paths
func TestBuildsWithDocker(t *testing.T) {
	e2eutil.RunWithDriver(DockerDriver{}, e2eutil.Apps, t, false)
}

// A negative case with missing plugin selector in kraft.yaml
func TestDockerBuildFailsNoPluginReference(t *testing.T) {
	e2eutil.RunWithDriver(DockerDriver{OmitPluginSelector: true}, e2eutil.Apps, t, true)
}

// PrependToFile prepends content to a file.
func PrependToFile(filename, content string) error {
	// Open the file in read-only mode.
	inputFile, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer inputFile.Close()

	// Read the original content from the file.
	originalContent, err := io.ReadAll(inputFile)
	if err != nil {
		return err
	}

	// Create a new string that is the content to prepend plus the original content.
	// Add a newline at the end of the content string to ensure it's on its own line.
	newContent := content + "\n" + string(originalContent)

	// Open the file in write-only mode.
	outputFile, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer outputFile.Close()

	// Write the new content to the file.
	_, err = outputFile.WriteString(newContent)
	if err != nil {
		return err
	}

	return nil
}
