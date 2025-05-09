package style

import "github.com/charmbracelet/lipgloss"

type Width struct {
	XS lipgloss.Style
	S  lipgloss.Style
	M  lipgloss.Style
	L  lipgloss.Style
	XL lipgloss.Style
}

var current = w4

func W() Width {
	return current
}

var w4 = Width{
	XS: lipgloss.NewStyle().Width(4),
	S:  lipgloss.NewStyle().Width(6),
	M:  lipgloss.NewStyle().Width(12),
	L:  lipgloss.NewStyle().Width(24),
	XL: lipgloss.NewStyle().Width(48),
}
