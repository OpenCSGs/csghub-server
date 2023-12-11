package console

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

var (
	focusedBoxStyle = lipgloss.NewStyle().
			Bold(true).
			Border(lipgloss.NormalBorder()).
			Foreground(lipgloss.Color("205")).
			PaddingLeft(1).
			PaddingRight(1)

	successStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("2"))

	greyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("244"))

	errorStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("9"))

	warningStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("3"))
)

type message struct {
	content string
}

func (m *message) String() string {
	return m.content
}

func (m *message) Print() {
	fmt.Print(m.content)
}

func (m *message) Println() {
	fmt.Println(m.content)
}

func RenderGreyColor(msg string) *message {
	return &message{greyStyle.Render(msg)}
}

func RenderSuccess(msg string) *message {
	return &message{successStyle.Render(msg)}
}

func RenderWarning(msg string) *message {
	return &message{warningStyle.Render(msg)}
}

func RenderError(msg string) *message {
	return &message{errorStyle.Render(msg)}
}

func RenderBox(msg string) *message {
	return &message{focusedBoxStyle.Render(msg)}
}
