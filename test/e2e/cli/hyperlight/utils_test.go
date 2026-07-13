// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package hyperlight_test

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

const projectName = "go-http-hyperlight"

func fixturesDir() (string, error) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("cannot determine path of fixtures directory")
	}
	return filepath.Join(filepath.Dir(filename), "fixtures"), nil
}

// WriteProject copies the fixture project into dir.
func WriteProject(dir string) error {
	srcDir, err := fixturesDir()
	if err != nil {
		return err
	}
	return os.CopyFS(dir, os.DirFS(srcDir))
}

func ArtifactPaths(projectDir string) (kernel, initrd string) {
	buildDir := filepath.Join(projectDir, ".unikraft", "build")
	return filepath.Join(buildDir, projectName+"_hyperlight-x86_64"),
		filepath.Join(buildDir, "initramfs-x86_64.cpio")
}
