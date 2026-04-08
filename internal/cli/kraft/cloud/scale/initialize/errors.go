// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package initialize

import "errors"

var (
	ErrServiceIdentifierRequired = errors.New("specify a service name or UUID")
	ErrWarmupTimeTooLow          = errors.New("warmup time must be at least 10ms")
	ErrCooldownTimeTooLow        = errors.New("cooldown time must be at least 10ms")
	ErrTemplateRequiredNoPrompt  = errors.New("specify an instance template UUID or name via --template")
	ErrNoInstanceTemplateFound   = errors.New("no instance template found in service")
)
