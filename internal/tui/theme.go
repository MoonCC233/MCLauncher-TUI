package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// The palette mirrors the OpenCode TUI: near-black background, warm orange
// accent, muted grays and a handful of semantic colors.
var (
	colBackground = lipgloss.Color("#0b0b0d")
	colPanel      = lipgloss.Color("#141417")
	colPanelAlt   = lipgloss.Color("#1c1c21")
	colBorder     = lipgloss.Color("#2b2b31")
	colBorderSoft = lipgloss.Color("#232329")
	colText       = lipgloss.Color("#e6e6e6")
	colMuted      = lipgloss.Color("#8b8b95")
	colDim        = lipgloss.Color("#5b5b64")
	colPrimary    = lipgloss.Color("#fab283")
	colPrimaryDim = lipgloss.Color("#c98f68")
	colGreen      = lipgloss.Color("#7fd88f")
	colRed        = lipgloss.Color("#e06c75")
	colCyan       = lipgloss.Color("#56b6c2")
	colPurple     = lipgloss.Color("#c678dd")
	colYellow     = lipgloss.Color("#e5c07b")
	colBlue       = lipgloss.Color("#61afef")
)

var (
	styleText     = lipgloss.NewStyle().Foreground(colText)
	styleBold     = lipgloss.NewStyle().Foreground(colText).Bold(true)
	styleMuted    = lipgloss.NewStyle().Foreground(colMuted)
	styleDim      = lipgloss.NewStyle().Foreground(colDim)
	stylePrimary  = lipgloss.NewStyle().Foreground(colPrimary)
	stylePrimaryB = lipgloss.NewStyle().Foreground(colPrimary).Bold(true)
	styleGreen    = lipgloss.NewStyle().Foreground(colGreen)
	styleRed      = lipgloss.NewStyle().Foreground(colRed)
	styleCyan     = lipgloss.NewStyle().Foreground(colCyan)
	stylePurple   = lipgloss.NewStyle().Foreground(colPurple)
	styleYellow   = lipgloss.NewStyle().Foreground(colYellow)
	styleBlue     = lipgloss.NewStyle().Foreground(colBlue)
	styleBorder   = lipgloss.NewStyle().Foreground(colBorder)
)

func accentGradient(line string, index int) string {
	colors := []lipgloss.Color{"#fab283", "#f3a675", "#eb9a68", "#e28f5c", "#d88452", "#cc7a49"}
	if index < 0 {
		index = 0
	}
	return lipgloss.NewStyle().Foreground(colors[index%len(colors)]).Render(line)
}
