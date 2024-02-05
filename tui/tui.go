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
		Foreground(lipgloss.Color("9")).
		Render

	TextWhiteBgRed = lipgloss.NewStyle().
			Background(lipgloss.Color("9")).
			Foreground(lipgloss.AdaptiveColor{
			Light: "15",
			Dark:  "0",
		}).
		Render

	TextGreen = lipgloss.NewStyle().
			Foreground(lipgloss.Color("10")).
			Render

	TextWhiteBgGreen = lipgloss.NewStyle().
				Background(lipgloss.Color("10")).
				Foreground(lipgloss.AdaptiveColor{
			Light: "15",
			Dark:  "0",
		}).
		Render

	TextBlue = lipgloss.NewStyle().
			Foreground(lipgloss.Color("12")).
			Render

	TextLightBlue = lipgloss.NewStyle().
			Foreground(lipgloss.Color("14")).
			Render

	TextWhiteBgBlue = lipgloss.NewStyle().
			Background(lipgloss.Color("12")).
			Foreground(lipgloss.AdaptiveColor{
			Light: "15",
			Dark:  "0",
		}).
		Render

	TextLightGray = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")).
			Render

	TextYellow = lipgloss.NewStyle().
			Foreground(lipgloss.Color("11")).
			Render
)
