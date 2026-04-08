// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package add

import "errors"

var (
	ErrConfigIdentifierRequired = errors.New("specify a configuration UUID or NAME")
	ErrPolicyNameRequired       = errors.New("specify a policy name")
	ErrInvalidStepCount         = errors.New("specify between 1 and 4 steps")
	ErrInvalidStepFormat        = errors.New("could not parse step")
	ErrInvalidStepBounds        = errors.New("lower bound cannot be greater or equal than upper bound")
	ErrInvalidEmptyLowerBound   = errors.New("lower bound cannot be empty in a step after the first step")
	ErrInvalidEmptyUpperBound   = errors.New("upper bound cannot be empty in a step before the last step")
	ErrNonContiguousSteps       = errors.New("steps are not contiguous")
	ErrInvalidPolicyType        = errors.New("invalid policy type")
)
