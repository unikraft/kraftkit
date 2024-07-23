// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package packmanager

import "fmt"

// MergeStrategy is a method to describe how to approach creating packages that
// have the same canonical name.  This is useful when deciding whether an
// existing package simply needs to be updated to include additional artifacts,
// e.g. targets, or whether the package should be overwritten because it is no
// longer required.
type MergeStrategy string

const (
	// The 'abort' strategy is used to error-out and indicate that no strategy was
	// provided and that further operations cannot be completed.
	StrategyAbort = MergeStrategy("abort")

	// The 'overwrite' strategy removes the existing package with the same
	// canonical name and replaces it with a new package entirely.
	StrategyOverwrite = MergeStrategy("overwrite")

	// The 'merge' strategy attempts to combine an existing package with new
	// artifacts such the updated package contains the artifacts from before plus
	// the artifacts.
	//
	// This is useful in contexts, for example, where packages represents an
	// application which has multiple targets and a new target is added to the
	// already packaged application.
	StrategyMerge = MergeStrategy("merge")

	// The 'prompt' strategy is an "unlisted" strategy that's used in TTY contexts
	// where the user is given the opportunity to decide the merge strategy before
	// the package operation is performed.  The result of this decision should
	// yield one of the above merge strategies.
	StrategyPrompt = MergeStrategy("prompt")
)

var _ fmt.Stringer = (*MergeStrategy)(nil)

// String implements fmt.Stringer
func (strategy MergeStrategy) String() string {
	return string(strategy)
}

// MergeStrategies returns the list of possible package merge strategies.
func MergeStrategies() []MergeStrategy {
	return []MergeStrategy{
		StrategyAbort,
		StrategyOverwrite,
		StrategyMerge,
	}
}

// MergeStrategyNames returns the string representation of all possible
// package merge strategies.
func MergeStrategyNames() []string {
	strategies := []string{}
	for _, strategy := range MergeStrategies() {
		strategies = append(strategies, strategy.String())
	}

	return strategies
}
