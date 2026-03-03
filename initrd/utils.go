// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package initrd

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
)

func compressFiles(output string, path string) error {
	reader, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("could not open initramfs file: %w", err)
	}

	fw, err := os.OpenFile(output+".gz", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("could not open gzip output file: %w", err)
	}

	gw := gzip.NewWriter(fw)

	if _, err := io.Copy(gw, reader); err != nil {
		return fmt.Errorf("could not compress initramfs file: %w", err)
	}

	err = gw.Close()
	if err != nil {
		return fmt.Errorf("could not close gzip writer: %w", err)
	}

	err = fw.Close()
	if err != nil {
		return fmt.Errorf("could not close compressed initramfs file: %w", err)
	}

	if err := os.Remove(output); err != nil {
		return fmt.Errorf("could not remove uncompressed initramfs: %w", err)
	}

	if err := os.Rename(output+".gz", output); err != nil {
		return fmt.Errorf("could not rename compressed initramfs: %w", err)
	}

	return nil
}
