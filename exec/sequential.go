// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package exec

import "context"

type SequentialProcesses struct {
	sequence []*Process
}

// NewSequential returns a newly generated SequentialProcesses structure with
// the provided processes
func NewSequential(sequence ...*Process) (*SequentialProcesses, error) {
	sp := &SequentialProcesses{
		sequence: sequence,
	}

	return sp, nil
}

// StartAndWait sequentially starts the list of processes and waits for it to
// complete before starting the next.
func (sq *SequentialProcesses) StartAndWait(ctx context.Context) error {
	for _, process := range sq.sequence {
		if err := process.StartAndWait(ctx); err != nil {
			return err
		}
	}

	return nil
}
