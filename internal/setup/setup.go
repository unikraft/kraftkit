// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package setup

import (
	"context"
)

type Setup struct {
	WithOS   string `long:"with-os" usage:"Force the host OS"`
	WithArch string `long:"with-arch" usage:"Force the architecture of the host"`
	WithVMM  string `long:"with-vmm" usage:"Force the use of a specific VMM on the host"`
	WithPM   string `long:"with-pm" usage:"Force the use of a specific package manager on the host"`
}

func DoSetup(ctx context.Context, sopts ...SetupOption) error {
	s := Setup{}
	for _, opts := range sopts {
		if err := opts(&s); err != nil {
			return err
		}
	}

	return nil
}
