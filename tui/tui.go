// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package tui

import "github.com/charmbracelet/lipgloss"

var (
	TextTitle = lipgloss.NewStyle().
			Bold(true).
			Render

	TextRed = lipgloss.NewStyle().
		Background(lipgloss.Color("9")).
		Foreground(lipgloss.CompleteAdaptiveColor{
			Light: lipgloss.CompleteColor{TrueColor: "#ffffff", ANSI256: "15", ANSI: "15"},
			Dark:  lipgloss.CompleteColor{TrueColor: "#5f0000", ANSI256: "52", ANSI: "0"},
		}).
		Render

	TextGreen = lipgloss.NewStyle().
			Background(lipgloss.Color("10")).
			Foreground(lipgloss.AdaptiveColor{
			Light: "10",
			Dark:  "0",
		}).
		Render

	TextBlue = lipgloss.NewStyle().
			Background(lipgloss.Color("4")).
			Foreground(lipgloss.AdaptiveColor{
			Light: "10",
			Dark:  "0",
		}).
		Render

	TextLightGray = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")).
			Render
)
