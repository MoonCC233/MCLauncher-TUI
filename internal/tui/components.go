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

// hintSpec is a clickable keyboard hint shown in a footer or modal.
type hintSpec struct {
	Key   string
	Label string
	Msg   tea.KeyMsg
}

func hint(key, label string) hintSpec {
	return hintSpec{Key: key, Label: label, Msg: keyMsg(key)}
}

func hintAction(key, label, action string) hintSpec {
	return hintSpec{Key: key, Label: label, Msg: keyMsg(action)}
}

func keyHint(key, label string) string {
	return stylePrimaryB.Render(key) + " " + styleDim.Render(label)
}

func hints(parts ...string) string {
	return strings.Join(parts, styleBorder.Render("  ·  "))
}

// renderHints draws a row of clickable hints and records the hit regions at
// the given screen coordinates. In RTL languages the order is reversed.
func (m *Model) renderHints(x, y int, parts ...hintSpec) string {
	return m.renderHintsWidth(x, y, 0, parts...)
}

// renderHintsWidth is renderHints with a maximum width. Hints that do not fit
// are dropped instead of being cut in half. The logical order (which hints are
// most important) decides what fits; RTL only mirrors the result afterwards.
func (m *Model) renderHintsWidth(x, y, maxWidth int, parts ...hintSpec) string {
	sep := styleBorder.Render("  ·  ")
	sepW := ansi.StringWidth(sep)
	if maxWidth > 0 {
		width, fit := 0, 0
		for i, part := range parts {
			w := ansi.StringWidth(keyHint(part.Key, part.Label))
			if i > 0 {
				w += sepW
			}
			if width+w > maxWidth {
				break
			}
			width += w
			fit++
		}
		parts = parts[:fit]
	}
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
			b.WriteString(sep)
		}
		start := x + ansi.StringWidth(b.String())
		text := keyHint(part.Key, part.Label)
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
