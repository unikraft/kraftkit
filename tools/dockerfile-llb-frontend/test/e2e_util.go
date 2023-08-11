// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

// Package test contains our end to end test suite.
package test

import (
	"fmt"
	"os/exec"
	"testing"
)

const (
	// DockerfilePath points to the Dockerfile of the LLB plugin.
	DockerfilePath = "../../Dockerfile"
	// ImageName is the name of the built LLB plugin.
	ImageName = "kraftkit-llb-e2e-image"
	// LocalRegistry is the address of the temporary e2e container registry.
	LocalRegistry = "localhost:5000"
	// dockerRegistryContainerName is the name of the temporary e2e container registry container.
	dockerRegistryContainerName = "kraftkit-llb-e2e-registry"
)

// Apps is the slice of targets for end-to-end testing.
var Apps = []string{"https://github.com/unikraft/app-helloworld.git"}

// ExecBuildDriver is an interface for executing CLI-based test suites.
type ExecBuildDriver interface {
	Run(string, string, string, *testing.T) ([]byte, error)
	Exec(string, ...string) ([]byte, error)
}

// CmdExec allows executing CLI calls in the context of our e2e suite.
type CmdExec struct{}

// cmdExecutor is a singleton for running CLI calls.
var cmdExecutor = &CmdExec{}

// Exec runs a CLI command.
func (e CmdExec) Exec(command string, args ...string) ([]byte, error) {
	cmd := exec.Command(command, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed running command %s %s: %v output: %s", command, args, err, output)
	}

	return output, nil
}

// DockerExec runs a docker command with args.
func (e CmdExec) DockerExec(args ...string) ([]byte, error) {
	return e.Exec("docker", args...)
}

// ShallowCloneToTempDir clones a git repo and puts it in a temporary directory.
// The directory disappears after tests.
func ShallowCloneToTempDir(t *testing.T, repoURL string) (string, error) {
	tempDir := t.TempDir()

	out, err := cmdExecutor.Exec("git", "clone", "--depth=1", repoURL, tempDir)
	if err != nil {
		return "", fmt.Errorf("failed cloning repo %s to %s: %v %s", repoURL, tempDir, err, out)
	}

	return tempDir, nil
}

// RunWithDriver executes the e2e suite using an injected driver.
func RunWithDriver(e ExecBuildDriver, apps []string, t *testing.T, shouldErr bool) {
	registry, image, err := SetupEnv()
	if err != nil {
		t.Fatalf("failed setting up test env %v", err)
	}

	// Ensure the registry is stopped after tests are complete
	t.Cleanup(func() {
		if err := stopDockerRegistry(); err != nil {
			t.Logf("failed to stop the registry: %v", err)
		}
	})

	for _, a := range apps {
		app := a
		t.Run(app, func(t *testing.T) {
			_, err := e.Run(registry, image, app, t)
			if err != nil {
				if !shouldErr {
					t.Errorf("expected no error but got: %v", err)
				}
			}
		})
	}
}

// SetupEnv spins up our end to end test environment.
// On a high level, it spawns a temporary container registry,
// builds an image of our LLB plugin and stores it within.
func SetupEnv() (string, string, error) {
	if err := runDockerRegistry(); err != nil {
		return "", "", err
	}
	if err := buildPluginImage(DockerfilePath, ImageName); err != nil {
		return "", "", err
	}
	if err := pushPluginImageToRegistry(LocalRegistry, ImageName); err != nil {
		return "", "", err
	}

	return LocalRegistry, ImageName, nil
}

func pushPluginImageToRegistry(localRegistry, imageName string) error {
	out, err := cmdExecutor.DockerExec("tag", imageName, localRegistry+"/"+imageName)
	if err != nil {
		return fmt.Errorf("failed tagging the image %s with %s/%s: %w output %s", imageName, localRegistry, imageName, err, out)
	}

	out, err = cmdExecutor.DockerExec("push", localRegistry+"/"+imageName)
	if err != nil {
		return fmt.Errorf("failed pushing the image %s/%s: %v output %s", localRegistry, imageName, err, out)
	}

	return nil
}

func buildPluginImage(dockerfilePath, imageName string) error {
	out, err := cmdExecutor.DockerExec("build", "-t", imageName, "-f", dockerfilePath, "../../")
	if err != nil {
		return fmt.Errorf("failed building the image with docker %s: %v", out, err)
	}

	return nil
}

func runDockerRegistry() error {
	// Clean up in case the registry from previous run is still hanging around.
	running, err := isDockerRegistryRunning()
	if err != nil {
		return fmt.Errorf("failed checking if the docker registry is running: %v", err)
	}
	if running {
		if err := stopDockerRegistry(); err != nil {
			return fmt.Errorf("failed stopping existing registry container: %v", err)
		}
	}

	out, err := cmdExecutor.DockerExec("run", "-d", "-p", "5000:5000", "--name="+dockerRegistryContainerName, "registry:2")
	if err != nil {
		return fmt.Errorf("failed setting up container registry %v: %s", err, out)
	}

	return nil
}

func stopDockerRegistry() error {
	out, err := cmdExecutor.DockerExec("rm", "-f", dockerRegistryContainerName)
	if err != nil {
		return fmt.Errorf("failed stopping the registry container %v: %s", err, out)
	}

	return nil
}

func isDockerRegistryRunning() (bool, error) {
	output, err := cmdExecutor.DockerExec("ps", "-q", "--filter", "name="+dockerRegistryContainerName)
	if err != nil {
		return false, fmt.Errorf("error checking if docker registry is running: %v", err)
	}
	return len(output) > 0, nil
}
