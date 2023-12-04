// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Acorn Labs, Inc; All rights reserved.
// Copyright 2022 Unikraft GmbH; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package confirm

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/erikgeiser/promptkit/confirmation"
)

var queryMark = lipgloss.NewStyle().
	Background(lipgloss.Color("12")).
	Foreground(lipgloss.AdaptiveColor{
		Light: "10",
		Dark:  "0",
	}).
	Render

// NewConfirm is a utility method used in a CLI context to prompt the user with
// a yes/no question.
func NewConfirm(question string) (bool, error) {
	input := confirmation.New(
		queryMark("[?] ")+
			question,
		confirmation.NewValue(true),
	)
	input.Template = confirmation.TemplateYN
	input.ResultTemplate = confirmation.ResultTemplateYN
	input.KeyMap.SelectYes = append(input.KeyMap.SelectYes, "+")
	input.KeyMap.SelectNo = append(input.KeyMap.SelectNo, "-")

	return input.RunPrompt()
}
