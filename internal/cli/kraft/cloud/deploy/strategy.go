// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package deploy

import "fmt"

// RolloutStrategy is a method to describe how to approach deployments which
// have the same canonical name.  This is useful when deciding whether an
// existing deployment simply needs to be updated.
type RolloutStrategy string

const (
	// The 'exit' strategy is used to error-out and indicate that no strategy was
	// provided and that further operations cannot be completed.
	StrategyExit = RolloutStrategy("exit")

	// The 'overwrite' strategy removes the existing deployment with the same
	// canonical name and replaces it with a new deployment entirely.
	StrategyOverwrite = RolloutStrategy("overwrite")

	// The 'prompt' strategy is an "unlisted" strategy that's used in TTY contexts
	// where the user is given the opportunity to decide the rollout strategy
	// before the package operation is performed.  The result of this decision
	// should yield one of the above rollout strategies.
	StrategyPrompt = RolloutStrategy("prompt")
)

var _ fmt.Stringer = (*RolloutStrategy)(nil)

// String implements fmt.Stringer
func (strategy RolloutStrategy) String() string {
	return string(strategy)
}

// RolloutStrategies returns the list of possible package rollout strategies.
func RolloutStrategies() []RolloutStrategy {
	return []RolloutStrategy{
		StrategyExit,
		StrategyOverwrite,
	}
}

// RolloutStrategyNames returns the string representation of all possible
// package merge strategies.
func RolloutStrategyNames() []string {
	strategies := []string{}
	for _, strategy := range RolloutStrategies() {
		strategies = append(strategies, strategy.String())
	}

	return strategies
}
