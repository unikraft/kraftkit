// SPDX-License-Identifier: Apache-2.0
// Copyright 2014 Docker, Inc.
// Copyright 2023 Unikraft GmbH and The KraftKit Authors

package validate

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"kraftkit.sh/libmocktainer/configs"
)

type check func(config *configs.Config) error

func Validate(config *configs.Config) error {
	checks := []check{
		rootfs,
	}
	for _, c := range checks {
		if err := c(config); err != nil {
			return err
		}
	}
	return nil
}

// rootfs validates if the rootfs is an absolute path and is not a symlink
// to the container's root filesystem.
func rootfs(config *configs.Config) error {
	if _, err := os.Stat(config.Rootfs); err != nil {
		return fmt.Errorf("invalid rootfs: %w", err)
	}
	cleaned, err := filepath.Abs(config.Rootfs)
	if err != nil {
		return fmt.Errorf("invalid rootfs: %w", err)
	}
	if cleaned, err = filepath.EvalSymlinks(cleaned); err != nil {
		return fmt.Errorf("invalid rootfs: %w", err)
	}
	if filepath.Clean(config.Rootfs) != cleaned {
		return errors.New("invalid rootfs: not an absolute path, or a symlink")
	}
	return nil
}
