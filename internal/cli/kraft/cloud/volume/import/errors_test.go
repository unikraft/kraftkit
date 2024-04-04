// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package vimport_test

import (
	"errors"
	"fmt"
	"testing"

	"kraftkit.sh/internal/cli/kraft/cloud/volume/import"
)

func TestLoggableError(t *testing.T) {
	genericErr := errors.New("an error")
	nologErr := vimport.NotLoggable(genericErr)

	// generic
	if !vimport.IsLoggable(genericErr) {
		t.Error("Expected generic error to be loggable")
	}
	if !vimport.IsLoggable(fmt.Errorf("wrapped: %w", genericErr)) {
		t.Error("Expected wrapped generic error to be loggable")
	}

	// nolog
	if vimport.IsLoggable(nologErr) {
		t.Error("Expected nolog error to not be loggable")
	}
	if vimport.IsLoggable(fmt.Errorf("wrapped: %w", nologErr)) {
		t.Error("Expected wrapped nolog error to not be loggable")
	}
}
