package tui

import (
	"fmt"
	"os"
	"strings"
)

// TerminalWindowTitle is fixed: the launcher always names its terminal window,
// so it is recognisable in a task bar full of shells.
const TerminalWindowTitle = "MCLauncher TUI"

// windowTitleSequence builds the OSC 2 sequence that renames the terminal
// window. Control characters are stripped so a title cannot inject escapes.
func windowTitleSequence(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		title = TerminalWindowTitle
	}
	title = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, title)
	return "\x1b]2;" + title + "\x07"
}

// SetWindowTitle writes the terminal window title using OSC 2. Terminals that
// do not understand it simply ignore the sequence.
func SetWindowTitle(title string) {
	fmt.Fprint(os.Stdout, windowTitleSequence(title))
}

// ApplyWindowTitle names the terminal window the launcher runs in.
func (m *Model) ApplyWindowTitle() {
	SetWindowTitle(TerminalWindowTitle)
}

// gameWindowTitle is the title the player wants on the Minecraft window, or an
// empty string to leave the game's own title alone.
func (m *Model) gameWindowTitle() string {
	return strings.TrimSpace(m.cfg.Settings.GameWindowTitle)
}

// gameTitleValue is what the Settings row shows; an em dash means the game keeps
// its own title, matching how the JVM arguments row renders an empty value.
func (m *Model) gameTitleValue() string {
	if title := m.gameWindowTitle(); title != "" {
		return title
	}
	return "—"
}
