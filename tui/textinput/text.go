package textinput

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/erikgeiser/promptkit/textinput"
)

var queryMark = lipgloss.NewStyle().
	Background(lipgloss.Color("12")).
	Foreground(lipgloss.AdaptiveColor{
		Light: "0",
		Dark:  "15",
	}).
	Render

// NewSpecify is a utility method used in a CLI context to prompt the user with
// a question to specify answer as string.
func NewTextInput(question, placeholder, defaultAns string) (string, error) {
	input := textinput.New(queryMark("[?] ") + question)
	input.Placeholder = placeholder
	input.InitialValue = defaultAns
	return input.RunPrompt()
}
