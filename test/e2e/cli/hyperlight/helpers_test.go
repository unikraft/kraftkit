// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package hyperlight_test

import (
	"os"
	"path/filepath"

	fcmd "kraftkit.sh/test/e2e/framework/cmd"
	fcfg "kraftkit.sh/test/e2e/framework/config"
)

func newKraft(stdout, stderr *fcmd.IOStream, cfg *fcfg.Config) *fcmd.Cmd {
	cmd := fcmd.NewKraft(stdout, stderr, cfg.Path())
	cmd.Args = append(cmd.Args, "--log-level", "info", "--log-type", "json")
	return cmd
}

func isolateHome(cmd *fcmd.Cmd, cfg *fcfg.Config) {
	tmpBase := filepath.Dir(filepath.Dir(cfg.Path()))
	cmd.Env = append(os.Environ(), "HOME="+tmpBase)
}
