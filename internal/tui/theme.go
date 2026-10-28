package tui

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/MoonCC233/MCLauncher-TUI/internal/config"
	"github.com/MoonCC233/MCLauncher-TUI/internal/util"
)

// The default palette mirrors the OpenCode TUI: near-black background, warm
// orange accent, muted grays and a handful of semantic colors.
type themePalette struct {
	ID         string
	Name       string
	Background string
	Panel      string
	PanelAlt   string
	Border     string
	BorderSoft string
	Text       string
	Muted      string
	Dim        string
	Primary    string
	PrimaryDim string
	Green      string
	Red        string
	Cyan       string
	Purple     string
	Yellow     string
	Blue       string
}

var themePresets = []themePalette{
	{
		ID: "opencode", Name: "OpenCode",
		Background: "#0b0b0d", Panel: "#141417", PanelAlt: "#1c1c21",
		Border: "#2b2b31", BorderSoft: "#232329",
		Text: "#e6e6e6", Muted: "#8b8b95", Dim: "#5b5b64",
		Primary: "#fab283", PrimaryDim: "#c98f68",
		Green: "#7fd88f", Red: "#e06c75", Cyan: "#56b6c2",
		Purple: "#c678dd", Yellow: "#e5c07b", Blue: "#61afef",
	},
	{
		ID: "midnight", Name: "Midnight",
		Background: "#080b12", Panel: "#0f1520", PanelAlt: "#172033",
		Border: "#25324a", BorderSoft: "#1b2638",
		Text: "#e2e8f0", Muted: "#8492a6", Dim: "#56637a",
		Primary: "#7aa2f7", PrimaryDim: "#5f7fc4",
		Green: "#9ece6a", Red: "#f7768e", Cyan: "#7dcfff",
		Purple: "#bb9af7", Yellow: "#e0af68", Blue: "#7aa2f7",
	},
	{
		ID: "forest", Name: "Forest",
		Background: "#0a0f0b", Panel: "#111a13", PanelAlt: "#19261b",
		Border: "#2a3b2d", BorderSoft: "#1f2d22",
		Text: "#e4ece5", Muted: "#8fa394", Dim: "#5e6f62",
		Primary: "#9ece6a", PrimaryDim: "#7aa653",
		Green: "#9ece6a", Red: "#e06c75", Cyan: "#56b6c2",
		Purple: "#c678dd", Yellow: "#e5c07b", Blue: "#61afef",
	},
	{
		ID: "rose", Name: "Rose",
		Background: "#0f0a0e", Panel: "#181119", PanelAlt: "#241a26",
		Border: "#3a2a3c", BorderSoft: "#2a1f2d",
		Text: "#efe6ee", Muted: "#a08a9e", Dim: "#6e5c6c",
		Primary: "#f5a9c8", PrimaryDim: "#c97f9c",
		Green: "#7fd88f", Red: "#e06c75", Cyan: "#56b6c2",
		Purple: "#c678dd", Yellow: "#e5c07b", Blue: "#61afef",
	},
	{
		ID: "mono", Name: "Mono",
		Background: "#0b0b0b", Panel: "#141414", PanelAlt: "#1e1e1e",
		Border: "#2e2e2e", BorderSoft: "#232323",
		Text: "#ededed", Muted: "#9a9a9a", Dim: "#666666",
		Primary: "#d7d7d7", PrimaryDim: "#a8a8a8",
		Green: "#a8cfa8", Red: "#d99a9a", Cyan: "#a8c8cf",
		Purple: "#c8b3d9", Yellow: "#d9cfa8", Blue: "#a8bdd9",
	},
}

var (
	colBackground lipgloss.Color
	colPanel      lipgloss.Color
	colPanelAlt   lipgloss.Color
	colBorder     lipgloss.Color
	colBorderSoft lipgloss.Color
	colText       lipgloss.Color
	colMuted      lipgloss.Color
	colDim        lipgloss.Color
	colPrimary    lipgloss.Color
	colPrimaryDim lipgloss.Color
	colGreen      lipgloss.Color
	colRed        lipgloss.Color
	colCyan       lipgloss.Color
	colPurple     lipgloss.Color
	colYellow     lipgloss.Color
	colBlue       lipgloss.Color
)

var (
	styleText     lipgloss.Style
	styleBold     lipgloss.Style
	styleMuted    lipgloss.Style
	styleDim      lipgloss.Style
	stylePrimary  lipgloss.Style
	stylePrimaryB lipgloss.Style
	styleGreen    lipgloss.Style
	styleRed      lipgloss.Style
	styleCyan     lipgloss.Style
	stylePurple   lipgloss.Style
	styleYellow   lipgloss.Style
	styleBlue     lipgloss.Style
	styleBorder   lipgloss.Style
)

var accentGradientColors []lipgloss.Color

func themeByID(id string) (themePalette, bool) {
	id = normalizeThemeID(id)
	for _, preset := range themePresets {
		if preset.ID == id {
			return preset, true
		}
	}
	return themePresets[0], false
}

func normalizeThemeID(id string) string {
	if !config.ValidTheme(id) {
		return config.DefaultTheme
	}
	return id
}

func themeName(id string) string {
	preset, _ := themeByID(id)
	return preset.Name
}

func applyTheme(themeID, accent string) {
	preset, _ := themeByID(themeID)
	if normalized, ok := util.NormalizeHexColor(accent); ok {
		preset.Primary = normalized
		preset.PrimaryDim = util.ShadeHex(normalized, 0.78)
	}
	colBackground = lipgloss.Color(preset.Background)
	colPanel = lipgloss.Color(preset.Panel)
	colPanelAlt = lipgloss.Color(preset.PanelAlt)
	colBorder = lipgloss.Color(preset.Border)
	colBorderSoft = lipgloss.Color(preset.BorderSoft)
	colText = lipgloss.Color(preset.Text)
	colMuted = lipgloss.Color(preset.Muted)
	colDim = lipgloss.Color(preset.Dim)
	colPrimary = lipgloss.Color(preset.Primary)
	colPrimaryDim = lipgloss.Color(preset.PrimaryDim)
	colGreen = lipgloss.Color(preset.Green)
	colRed = lipgloss.Color(preset.Red)
	colCyan = lipgloss.Color(preset.Cyan)
	colPurple = lipgloss.Color(preset.Purple)
	colYellow = lipgloss.Color(preset.Yellow)
	colBlue = lipgloss.Color(preset.Blue)

	styleText = lipgloss.NewStyle().Foreground(colText)
	styleBold = lipgloss.NewStyle().Foreground(colText).Bold(true)
	styleMuted = lipgloss.NewStyle().Foreground(colMuted)
	styleDim = lipgloss.NewStyle().Foreground(colDim)
	stylePrimary = lipgloss.NewStyle().Foreground(colPrimary)
	stylePrimaryB = lipgloss.NewStyle().Foreground(colPrimary).Bold(true)
	styleGreen = lipgloss.NewStyle().Foreground(colGreen)
	styleRed = lipgloss.NewStyle().Foreground(colRed)
	styleCyan = lipgloss.NewStyle().Foreground(colCyan)
	stylePurple = lipgloss.NewStyle().Foreground(colPurple)
	styleYellow = lipgloss.NewStyle().Foreground(colYellow)
	styleBlue = lipgloss.NewStyle().Foreground(colBlue)
	styleBorder = lipgloss.NewStyle().Foreground(colBorder)

	accentGradientColors = buildAccentGradient(preset.Primary)
}

func buildAccentGradient(base string) []lipgloss.Color {
	factors := []float64{1.0, 0.96, 0.91, 0.86, 0.81, 0.76}
	colors := make([]lipgloss.Color, 0, len(factors))
	for _, factor := range factors {
		colors = append(colors, lipgloss.Color(util.ShadeHex(base, factor)))
	}
	return colors
}

func accentGradient(line string, index int) string {
	if len(accentGradientColors) == 0 {
		return line
	}
	if index < 0 {
		index = 0
	}
	return lipgloss.NewStyle().
		Foreground(accentGradientColors[index%len(accentGradientColors)]).
		Render(line)
}
