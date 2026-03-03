// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

// Package spellcheck provides fuzzy matching for KConfig option names,
// suggesting corrections when users misspell configuration symbols.
package spellcheck

import (
	"bufio"
	"os"
	"strings"

	edlib "github.com/hbollon/go-edlib"
)

const (
	// defaultSimilarityThreshold is the minimum similarity score (0.0–1.0)
	// for suggesting a correction. 0.7 is a standard threshold for
	// technical symbol matching.
	defaultSimilarityThreshold float32 = 0.7

	// configPrefix is the standard KConfig option prefix.
	configPrefix = "CONFIG_"
)

// AllKConfigSymbols parses a .config file and returns all valid KConfig
// symbol names, including both active entries (CONFIG_X=value) and
// commented-out entries (# CONFIG_X is not set).
func AllKConfigSymbols(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	seen := map[string]struct{}{}
	var symbols []string

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Match commented-out entries: "# CONFIG_X is not set"
		if strings.HasPrefix(line, "# "+configPrefix) && strings.HasSuffix(line, " is not set") {
			name := strings.TrimSuffix(strings.TrimPrefix(line, "# "), " is not set")
			if _, ok := seen[name]; !ok {
				seen[name] = struct{}{}
				symbols = append(symbols, name)
			}
			continue
		}

		// Skip other comments
		if strings.HasPrefix(line, "#") {
			continue
		}

		// Match active entries: "CONFIG_X=value"
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 && parts[0] != "" {
			if _, ok := seen[parts[0]]; !ok {
				seen[parts[0]] = struct{}{}
				symbols = append(symbols, parts[0])
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return symbols, nil
}

// Option is the KConfig symbol name without the CONFIG_ prefix
type Result struct {
	// Option is the user-provided option name (e.g. "CONFIG_LIBWFSCORE").
	Option string

	// Suggestion is the closest known option if found (e.g. "CONFIG_LIBVFSCORE"),
	// empty string if no close match exists.
	Suggestion string

	// Similarity is the Damerau-Levenshtein similarity score (0.0–1.0)
	// between Option and Suggestion.
	Similarity float32
}

// Option configures the spellcheck behavior.
type Option func(*spellcheckConfig)

type spellcheckConfig struct {
	similarityThreshold float32
}

// WithSimilarityThreshold sets the minimum similarity score for suggestions.
func WithSimilarityThreshold(t float32) Option {
	return func(c *spellcheckConfig) {
		c.similarityThreshold = t
	}
}

// stripPrefix removes the CONFIG_ prefix from an option name if present.
func stripPrefix(option string) string {
	return strings.TrimPrefix(option, configPrefix)
}

// Validate checks user-provided KConfig option names against a set of
// known config symbols. Returns a Result for each unknown option.
// knownConfigs can include CONFIG_ prefix or not — both are handled.
func Validate(userOptions []string, knownConfigs []string, opts ...Option) []Result {
	cfg := &spellcheckConfig{
		similarityThreshold: defaultSimilarityThreshold,
	}
	for _, o := range opts {
		o(cfg)
	}

	// Normalize known configs by stripping CONFIG_ prefix
	normalized := make([]string, len(knownConfigs))
	knownSet := make(map[string]bool, len(knownConfigs))
	for i, k := range knownConfigs {
		stripped := stripPrefix(k)
		normalized[i] = stripped
		knownSet[stripped] = true
	}

	var results []Result

	for _, userOpt := range userOptions {
		// Extract the option name, stripping CONFIG_ prefix and any =value suffix
		name := stripPrefix(userOpt)
		if idx := strings.IndexRune(name, '='); idx >= 0 {
			name = name[:idx]
		}

		// Skip if the option is known
		if knownSet[name] {
			continue
		}

		result := Result{Option: name}

		// Find the closest match using Damerau-Levenshtein
		if len(normalized) > 0 {
			match, err := edlib.FuzzySearch(name, normalized, edlib.DamerauLevenshtein)
			if err == nil && match != "" {
				sim, err := edlib.StringsSimilarity(name, match, edlib.DamerauLevenshtein)
				if err == nil && float32(sim) >= cfg.similarityThreshold {
					result.Suggestion = match
					result.Similarity = float32(sim)
				}
			}
		}

		results = append(results, result)
	}

	return results
}
