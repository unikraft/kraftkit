// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package create

import "fmt"

// RolloutStrategy is the mechanism to describe how to approach managing
// existing instances part of an existing service during deployment.
type RolloutStrategy string

const (
	// The 'abort' strategy is used to error-out and indicate that no strategy was
	// provided and that further operations cannot be completed.
	RolloutStrategyAbort = RolloutStrategy("abort")

	// The 'stop' strategy stops the qualified existing instance(s) within the
	// same service and starts the new instance(s).
	RolloutStrategyStop = RolloutStrategy("stop")

	// The 'remove' strategy removes the qualified existing instance(s) within
	// the same service and starts the new instance(s).
	RolloutStrategyRemove = RolloutStrategy("remove")

	// The 'sequential' strategy removes the existing qualified instance(s)
	// within the same service and starts the new instance(s) one by one.
	RolloutStrategyRemoveSequential = RolloutStrategy("remove_sequential")

	// The 'keep' strategy keeps the existing qualified instance(s) within the
	// same service and starts the new instance(s).
	RolloutStrategyKeep = RolloutStrategy("keep")

	// The 'prompt' strategy is an "unlisted" strategy that's used in TTY contexts
	// where the user is given the opportunity to decide the rollout strategy
	// before the rollout operation is performed.  The result of this decision
	// should yield one of the above rollout strategies.
	StrategyPrompt = RolloutStrategy("prompt")
)

var _ fmt.Stringer = (*RolloutStrategy)(nil)

// String implements fmt.Stringer
func (strategy RolloutStrategy) String() string {
	return string(strategy)
}

// RolloutStrategies returns the list of possible rollout strategies.
func RolloutStrategies() []RolloutStrategy {
	return []RolloutStrategy{
		RolloutStrategyAbort,
		RolloutStrategyStop,
		RolloutStrategyRemove,
		RolloutStrategyRemoveSequential,
		RolloutStrategyKeep,
	}
}

// RolloutStrategyNames returns the string representation of all possible
// rollout strategies.
func RolloutStrategyNames() []string {
	strategies := []string{}
	for _, strategy := range RolloutStrategies() {
		strategies = append(strategies, strategy.String())
	}

	return strategies
}
