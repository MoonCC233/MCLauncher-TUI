package tui

import (
	"math"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/MoonCC233/MCLauncher-TUI/internal/config"
)

// homeTitle is the product name. The Home screen draws it as ASCII art and falls
// back to a letter-spaced line when the terminal is too narrow.
const homeTitle = "MCTL"

// titleFonts holds the selectable glyph styles. Both are four rows tall so the
// layout does not shift when the style changes.
var titleFonts = map[string]map[rune][]string{
	"block": blockGlyphs,
	"line":  lineGlyphs,
}

// blockGlyphs is a solid four-row font. M and T keep five cells so their shapes
// stay readable, the rest need four.
var blockGlyphs = map[rune][]string{
	'M': {"█   █", "██ ██", "█ █ █", "█   █"},
	'C': {"████", "█   ", "█   ", "████"},
	'T': {"█████", "  █  ", "  █  ", "  █  "},
	'L': {"█   ", "█   ", "█   ", "████"},
}

// lineGlyphs is the classic figlet-style outline font built from ASCII strokes.
var lineGlyphs = map[rune][]string{
	'M': {" __  __ ", "|  \\/  |", "| |\\/| |", "|_|  |_|"},
	'C': {"  ___ ", " / __|", "| (__ ", " \\___|"},
	'T': {" _____ ", "|_   _|", "  | |  ", "  |_|  "},
	'L': {" _     ", "| |    ", "| |___ ", "|_____|"},
}

// titleFontGlyphs returns the glyph table for a font id.
func titleFontGlyphs(font string) map[rune][]string {
	if glyphs, ok := titleFonts[font]; ok {
		return glyphs
	}
	return titleFonts[config.DefaultTitleFont]
}

// Animation tuning for the home title. Frames arrive every 120ms; the numbers
// are expressed in cells and frames so the motion stays terminal independent.
const (
	titleSheenWidth   = 4.5  // width of the travelling highlight, in cells
	titleSheenPeriod  = 34.0 // cells between two highlights
	titleSheenStep    = 1.0  // frames it takes the highlight to move one cell
	titleWavePeriod   = 22.0 // cells per wave crest
	titleWaveStep     = 0.22 // radians the wave advances per frame
	titleBreatheStep  = 0.14 // radians the whole title advances per frame
	titleRestingLevel = 0.42 // share of the palette used as the resting level
	titleRowSkew      = 2.5  // cells a row trails the one above it
)

// titleIndent is the left margin viewHome puts in front of the title, so the
// title itself has to fit in width-titleIndent columns.
const titleIndent = 2

// titleLines renders the Home title for the width available.
func (m *Model) titleLines(width int) []string {
	anim := m.titleAnim()
	art := titleArt(homeTitle, m.titleFont())
	if widest := titleArtWidth(art); len(art) > 0 && widest+titleIndent <= width {
		return paintTitleArt(art, m.animFrame, anim)
	}
	return []string{renderTitle(titleDisplay(homeTitle), m.animFrame, anim, width)}
}

func (m *Model) titleAnim() string {
	anim := strings.ToLower(strings.TrimSpace(m.cfg.Settings.TitleAnim))
	if !config.ValidTitleAnim(anim) {
		return config.DefaultTitleAnim
	}
	return anim
}

func (m *Model) titleFont() string {
	font := strings.ToLower(strings.TrimSpace(m.cfg.Settings.TitleFont))
	if !config.ValidTitleFont(font) {
		return config.DefaultTitleFont
	}
	return font
}

// titleDisplay letter-spaces the wordmark for narrow terminals.
func titleDisplay(text string) string {
	runes := []rune(text)
	parts := make([]string, 0, len(runes)*2)
	for i, r := range runes {
		if i > 0 {
			parts = append(parts, " ")
		}
		parts = append(parts, string(r))
	}
	return strings.Join(parts, "")
}

// titleArt builds the plain, uncoloured ASCII wordmark for a font id.
func titleArt(text, font string) []string {
	glyphs := titleFontGlyphs(font)
	rows := make([]string, len(glyphs['M']))
	for _, r := range text {
		glyph, ok := glyphs[r]
		if !ok {
			continue
		}
		for i := range rows {
			if rows[i] != "" {
				rows[i] += " "
			}
			rows[i] += glyph[i]
		}
	}
	for i := range rows {
		rows[i] = strings.TrimRight(rows[i], " ")
	}
	return rows
}

func titleArtWidth(rows []string) int {
	widest := 0
	for _, row := range rows {
		if w := len([]rune(row)); w > widest {
			widest = w
		}
	}
	return widest
}

// paintTitleArt colours the wordmark one cell at a time. Adjacent cells that
// share a shade are grouped so a frame stays a handful of escape sequences.
func paintTitleArt(rows []string, frame int, anim string) []string {
	length := titleArtWidth(rows)
	out := make([]string, len(rows))
	for i, row := range rows {
		out[i] = paintTitleRow(row, i, length, frame, anim)
	}
	return out
}

func paintTitleRow(line string, row, length, frame int, anim string) string {
	runes := []rune(line)
	if len(runes) == 0 || len(titleStyles) == 0 {
		return line
	}
	var b strings.Builder
	for start := 0; start < len(runes); {
		shade := titleShadeIndex(row, start, length, frame, anim)
		end := start + 1
		for end < len(runes) && titleShadeIndex(row, end, length, frame, anim) == shade {
			end++
		}
		b.WriteString(titleStyle(shade).Render(string(runes[start:end])))
		start = end
	}
	return b.String()
}

// renderTitle colours a single line of text with the same animation. width is
// the space viewHome has for the title, indentation included.
func renderTitle(text string, frame int, anim string, width int) string {
	limit := width - titleIndent
	if limit < 1 {
		limit = 1
	}
	text = truncateRunes(text, limit)
	return paintTitleRow(text, 0, len([]rune(text)), frame, anim)
}

func titleStyle(shade int) lipgloss.Style {
	if len(titleStyles) == 0 {
		return lipgloss.NewStyle().Bold(true)
	}
	if shade < 0 {
		shade = 0
	}
	if shade >= len(titleStyles) {
		shade = len(titleStyles) - 1
	}
	return titleStyles[shade]
}

// titleShadeIndex maps a cell to a palette level for the given moment. Rows are
// skewed so the motion reads as a diagonal sweep rather than a flat blink.
func titleShadeIndex(row, col, length, frame int, anim string) int {
	levels := len(titleStyles)
	if levels == 0 {
		return 0
	}
	rest := titleRestingLevel * float64(levels-1)
	span := float64(levels-1) - rest
	position := float64(col) + float64(row)*titleRowSkew
	total := float64(max(1, length-1))

	var level float64
	switch anim {
	case "off":
		// A static left-to-right fade.
		level = rest + span*(1-math.Min(1, position/total))
	case "wave":
		// Crests travel to the right and wrap around the title.
		phase := 2 * math.Pi * (position/titleWavePeriod - float64(frame)*titleWaveStep/(2*math.Pi))
		level = rest + span*(0.5+0.5*math.Sin(phase))
	case "breathe":
		// The whole title pulses together.
		level = rest + span*(0.5+0.5*math.Sin(float64(frame)*titleBreatheStep))
	default:
		// sheen: one Gaussian highlight sweeping past the letters.
		offset := math.Mod(position-float64(frame)/titleSheenStep, titleSheenPeriod)
		if offset < 0 {
			offset += titleSheenPeriod
		}
		distance := math.Min(offset, titleSheenPeriod-offset)
		if distance > titleSheenWidth*4 {
			level = rest
			break
		}
		level = rest + span*math.Exp(-(distance*distance)/(2*titleSheenWidth*titleSheenWidth))
	}
	shade := int(math.Round(level))
	if shade < 0 {
		shade = 0
	}
	if shade >= levels {
		shade = levels - 1
	}
	return shade
}

// animVisible reports whether the animated title is on screen right now. The
// animation only costs frames while it can actually be seen.
func (m *Model) animVisible() bool {
	return m.screen == ScreenHome && m.modal == modalNone && m.titleAnim() != "off"
}

// ensureAnimTick schedules the next animation frame when the title is visible
// and no tick is pending yet.
func (m *Model) ensureAnimTick() tea.Cmd {
	if m.animRunning || !m.animVisible() {
		return nil
	}
	m.animRunning = true
	return animTickCmd()
}
