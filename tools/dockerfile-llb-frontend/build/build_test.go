// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package build

//go:generate mockery --srcpkg=github.com/moby/buildkit/frontend/gateway/client --name=Client

import (
	"context"
	"fmt"
	"testing"

	"kraftkit.sh/tools/dockerfile-llb-frontend/build/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestBuildKitDriverInit(t *testing.T) {
	driver := DefaultBuildkitDriver{}

	assert.Error(t, driver.Init(nil, &Configuration{ContextName: "context"}))
	assert.Error(t, driver.Init(mocks.NewClient(t), nil))

	assert.NoError(t, driver.Init(mocks.NewClient(t), &Configuration{ContextName: "context"}))
}

func TestBuildRunInformsAboutFailedSolving(t *testing.T) {
	driver := DefaultBuildkitDriver{}

	client := mocks.NewClient(t)
	client.On("Solve", mock.Anything, mock.Anything).Return(nil, fmt.Errorf("failed solving"))

	driver.Init(client, &Configuration{ContextName: "context"})

	_, err := driver.Run(context.Background())

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to solve client request from definition")
}

func TestKraftkitBuildCommand(t *testing.T) {
	platform := "qemu"
	architecture := "arm64"
	expected := "kraft build . -p qemu -m arm64"

	assert.Equal(t, kraftkitBuildCommand(platform, architecture), expected)
}

func TestTargetSelector(t *testing.T) {
	platform := "qemu"
	architecture := "arm64"
	expected := "-p qemu -m arm64"

	assert.Equal(t, expected, targetSelector(platform, architecture))
}

func TestOutputPath(t *testing.T) {
	config := Configuration{
		OutputDir:    "/.unikraft/build",
		Platform:     "qemu",
		Architecture: "arm64",
	}
	project := "hello-world"
	expected := "/.unikraft/build/hello-world_qemu-arm64"

	assert.Equal(t, expected, outputPath(project, config))
}
