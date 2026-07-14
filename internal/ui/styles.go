package ui

import "github.com/charmbracelet/lipgloss"

const (
	colWidth = 30
	colGap   = 1
)

var (
	colorSubtle = lipgloss.Color("240")
	colorAccent = lipgloss.Color("212")
	colorFaint  = lipgloss.Color("245")
	colorWarn   = lipgloss.Color("203")

	boardTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("255")).
			Padding(0, 1)

	statusBarStyle = lipgloss.NewStyle().
			Foreground(colorFaint).
			Padding(0, 1)

	helpStyle = lipgloss.NewStyle().
			Foreground(colorSubtle).
			Padding(0, 1)

	columnHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Width(colWidth-2).
				Padding(0, 1)

	columnHeaderFocusedStyle = columnHeaderStyle.
					Foreground(lipgloss.Color("0")).
					Background(colorAccent)

	columnHeaderBlurredStyle = columnHeaderStyle.
					Foreground(lipgloss.Color("255"))

	columnBoxStyle = lipgloss.NewStyle().
			Width(colWidth).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorSubtle).
			Padding(0, 0, 1, 0)

	columnBoxFocusedStyle = columnBoxStyle.
				BorderForeground(colorAccent)

	cardStyle = lipgloss.NewStyle().
			Width(colWidth-4).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorSubtle).
			Padding(0, 1).
			Margin(0, 0, 1, 1)

	cardFocusedStyle = cardStyle.
				BorderForeground(colorAccent).
				Bold(true)

	cardBodyPreviewStyle = lipgloss.NewStyle().
				Foreground(colorFaint)

	emptyColumnStyle = lipgloss.NewStyle().
				Foreground(colorSubtle).
				Italic(true).
				Padding(0, 1)

	modalTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorAccent).
			MarginBottom(1)

	modalStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorAccent).
			Padding(1, 2)

	modalHelpStyle = lipgloss.NewStyle().
			Foreground(colorSubtle).
			MarginTop(1)

	dangerStyle = lipgloss.NewStyle().
			Foreground(colorWarn).
			Bold(true)
)

// cardColors are preset swatches a card's accent border can cycle through.
var cardColors = []string{
	"",    // none
	"203", // red
	"215", // orange
	"221", // yellow
	"114", // green
	"75",  // blue
	"183", // purple
}

func cardColorStyle(hex string, base lipgloss.Style) lipgloss.Style {
	if hex == "" {
		return base
	}
	return base.BorderForeground(lipgloss.Color(hex))
}
