package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func fitLine(s string, width int) string {
	if width <= 0 {
		return ""
	}
	s = ansi.Truncate(s, width, "…")
	if gap := width - ansi.StringWidth(s); gap > 0 {
		s += strings.Repeat(" ", gap)
	}
	return s
}

func clipLines(s string, width, height int) string {
	if height <= 0 {
		return ""
	}
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = fitLine(lines[i], width)
	}
	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", width))
	}
	if len(lines) > height {
		lines = lines[:height]
	}
	return strings.Join(lines, "\n")
}

func padBlock(lines []string, width, height int) []string {
	out := make([]string, 0, height)
	for i := 0; i < height; i++ {
		if i < len(lines) {
			out = append(out, fitLine(lines[i], width))
		} else {
			out = append(out, strings.Repeat(" ", width))
		}
	}
	return out
}

func sectionHeader(title, right string, width int) string {
	left := stylePrimary.Render("▌") + " " + styleBold.Render(title)
	right = styleDim.Render(right)
	return sectionHeaderStyled(left, right, width)
}

func (m *Model) sectionHeader(title, right string, width int) string {
	left := stylePrimary.Render("▌") + " " + styleBold.Render(title)
	right = styleDim.Render(right)
	if m.rtl() {
		left, right = right, left
	}
	return sectionHeaderStyled(left, right, width)
}

func sectionHeaderStyled(left, right string, width int) string {
	leftW := ansi.StringWidth(left)
	rightW := ansi.StringWidth(right)
	ruleW := width - leftW - rightW - 3
	if ruleW < 1 {
		ruleW = 1
	}
	return left + " " + styleBorder.Render(strings.Repeat("─", ruleW)) + " " + right
}

// hintSpec is a clickable button shown in a footer or modal. Key is the
// internal action the button replays; it is no longer displayed, so buttons
// stay usable without knowing the keyboard shortcut.
type hintSpec struct {
	Key   string
	Label string
	Msg   tea.KeyMsg
}

const buttonGap = "  "

func hint(key, label string) hintSpec {
	return hintSpec{Key: key, Label: label, Msg: keyMsg(key)}
}

func hintAction(key, label, action string) hintSpec {
	return hintSpec{Key: key, Label: label, Msg: keyMsg(action)}
}

// buttonWidth is the cell width of the compact button.
func (m *Model) buttonWidth(label string) int {
	return ansi.StringWidth(label) + 2
}

// buttonAt renders one button. Normal buttons use a darker panel fill with an
// accent label; hover fills the button with the accent color; the pressed state
// uses the dimmed accent while the mouse button is held.
func (m *Model) buttonAt(label string, x, y int) string {
	w := m.buttonWidth(label)
	hovered := m.hoverValid && m.hoverY == y && m.hoverX >= x && m.hoverX < x+w
	text := " " + label + " "
	switch {
	case hovered && m.mouseDown:
		return lipgloss.NewStyle().Background(colPrimaryDim).Foreground(colBackground).Bold(true).Render(text)
	case hovered:
		return lipgloss.NewStyle().Background(colPrimary).Foreground(colBackground).Bold(true).Render(text)
	default:
		return lipgloss.NewStyle().Background(colPanel).Foreground(colPrimary).Bold(true).Render(text)
	}
}

// layoutButtons keeps the buttons that fit in maxWidth.
func (m *Model) layoutButtons(maxWidth int, parts []hintSpec) ([]hintSpec, int) {
	gap := ansi.StringWidth(buttonGap)
	width, fit := 0, 0
	for i, part := range parts {
		w := m.buttonWidth(part.Label)
		if i > 0 {
			w += gap
		}
		if maxWidth > 0 && width+w > maxWidth {
			break
		}
		width += w
		fit++
	}
	return parts[:fit], width
}

// layoutButtonRows wraps a button bar into as many rows as needed. Modal
// footers use this so a narrow panel never hides an action.
func (m *Model) layoutButtonRows(maxWidth int, parts []hintSpec) [][]hintSpec {
	if len(parts) == 0 {
		return nil
	}
	gap := ansi.StringWidth(buttonGap)
	var rows [][]hintSpec
	var row []hintSpec
	width := 0
	for _, part := range parts {
		w := m.buttonWidth(part.Label)
		if maxWidth > 0 && len(row) > 0 && width+gap+w > maxWidth {
			rows = append(rows, row)
			row = nil
			width = 0
		}
		if len(row) > 0 {
			width += gap
		}
		row = append(row, part)
		width += w
	}
	if len(row) > 0 {
		rows = append(rows, row)
	}
	return rows
}

// renderHints draws a row of clickable buttons and records the hit regions at
// the given screen coordinates. In RTL languages the order is reversed.
func (m *Model) renderHints(x, y int, parts ...hintSpec) string {
	return m.renderHintsWidth(x, y, 0, parts...)
}

// renderHintsWidth is renderHints with a maximum width. Buttons that do not fit
// are dropped instead of being cut in half. The logical order (which buttons
// are most important) decides what fits; RTL only mirrors the result afterwards.
func (m *Model) renderHintsWidth(x, y, maxWidth int, parts ...hintSpec) string {
	parts, _ = m.layoutButtons(maxWidth, parts)
	return m.renderHintParts(x, y, parts...)
}

// renderHintParts renders already-fitted buttons. Callers that need a total
// width can measure first and render at the final column afterwards, which
// keeps hover hit-testing accurate.
func (m *Model) renderHintParts(x, y int, parts ...hintSpec) string {
	if len(parts) == 0 {
		return ""
	}
	if m.rtl() {
		reversed := make([]hintSpec, len(parts))
		for i := range parts {
			reversed[len(parts)-1-i] = parts[i]
		}
		parts = reversed
	}
	var b strings.Builder
	for i, part := range parts {
		if i > 0 {
			b.WriteString(buttonGap)
		}
		start := x + ansi.StringWidth(b.String())
		text := m.buttonAt(part.Label, start, y)
		b.WriteString(text)
		if y >= 0 {
			m.addHit(start, y, ansi.StringWidth(text), 1, hitRegion{Kind: hitKey, Key: part.Msg})
		}
	}
	return b.String()
}

func pill(text string, color lipgloss.Color) string {
	return lipgloss.NewStyle().Foreground(color).Render(" " + text + " ")
}

func progressBar(done, total int64, width int) string {
	if width < 12 {
		width = 12
	}
	ratio := 0.0
	if total > 0 {
		ratio = float64(done) / float64(total)
	}
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	filled := int(ratio * float64(width))
	if filled > width {
		filled = width
	}
	bar := stylePrimary.Render(strings.Repeat("█", filled)) +
		styleMuted.Render(strings.Repeat("░", width-filled))
	return bar
}

func (m *Model) progressBar(done, total int64, width int) string {
	if !m.rtl() {
		return progressBar(done, total, width)
	}
	if width < 12 {
		width = 12
	}
	ratio := 0.0
	if total > 0 {
		ratio = float64(done) / float64(total)
	}
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	filled := int(ratio * float64(width))
	if filled > width {
		filled = width
	}
	return styleMuted.Render(strings.Repeat("░", width-filled)) +
		stylePrimary.Render(strings.Repeat("█", filled))
}

func wrapPlain(text string, width int) []string {
	if width < 4 {
		width = 4
	}
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}
	var lines []string
	current := ""
	for _, word := range words {
		if current == "" {
			current = word
			continue
		}
		if ansi.StringWidth(current)+1+ansi.StringWidth(word) <= width {
			current += " " + word
		} else {
			lines = append(lines, current)
			current = word
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}

func truncateRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max == 1 {
		return string(r[:1])
	}
	return string(r[:max-1]) + "…"
}

func spinnerFrame(i int) string {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	if i < 0 {
		i = -i
	}
	return frames[i%len(frames)]
}

func plural(n int, singular, plural string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, singular)
	}
	return fmt.Sprintf("%d %s", n, plural)
}

// wrapText breaks text into lines no wider than width. Explicit newlines are
// kept, spaces are preferred as break points, and CJK text without spaces falls
// back to rune boundaries.
func wrapText(text string, width int) []string {
	if width < 4 {
		width = 4
	}
	var out []string
	for _, paragraph := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		if strings.TrimSpace(paragraph) == "" {
			out = append(out, "")
			continue
		}
		line := ""
		flush := func() {
			if line != "" {
				out = append(out, line)
				line = ""
			}
		}
		for _, word := range strings.Fields(paragraph) {
			candidate := word
			if line != "" {
				candidate = line + " " + word
			}
			if ansi.StringWidth(candidate) <= width {
				line = candidate
				continue
			}
			flush()
			// A single word can still be wider than the line, which is the normal
			// case for Chinese text.
			for ansi.StringWidth(word) > width {
				runes := []rune(word)
				cut := 0
				for cut < len(runes) && ansi.StringWidth(string(runes[:cut+1])) <= width {
					cut++
				}
				if cut == 0 {
					cut = 1
				}
				out = append(out, string(runes[:cut]))
				word = string(runes[cut:])
			}
			line = word
		}
		flush()
	}
	return out
}

// padLabel pads a label by display width, so CJK and latin labels line up.
func padLabel(text string, width int) string {
	if gap := width - ansi.StringWidth(text); gap > 0 {
		return text + strings.Repeat(" ", gap)
	}
	return text
}
