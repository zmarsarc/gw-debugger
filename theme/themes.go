package theme

import "github.com/charmbracelet/lipgloss"

type Theme struct {
	TextLight         lipgloss.Color
	TextDark          lipgloss.Color
	Success           lipgloss.Color
	Warning           lipgloss.Color
	Error             lipgloss.Color
	PanelDark         lipgloss.Color
	PanelLight        lipgloss.Color
	Background        lipgloss.Color
	BackgroundInverse lipgloss.Color
}

var currentTheme = dark

func G() Theme {
	return currentTheme
}

var dark = Theme{
	TextLight:         lipgloss.Color("#FFFFFF"),
	TextDark:          lipgloss.Color("#000000"),
	Success:           lipgloss.Color("#8ac926"),
	Warning:           lipgloss.Color("#ffca3a"),
	Error:             lipgloss.Color("#ff595e"),
	PanelDark:         lipgloss.Color("#023047"),
	PanelLight:        lipgloss.Color("#219ebc"),
	Background:        lipgloss.Color("#000000"),
	BackgroundInverse: lipgloss.Color("#FFFFFF"),
}
