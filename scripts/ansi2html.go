//go:build ignore

// ansi2html converts a stream of ANSI-colored text into a standalone HTML
// page. It is only used to render documentation screenshots:
//
//	go run scripts/ansi2html.go preview.ansi preview.html "mclauncher"
package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"html"
	"os"
	"strconv"
	"strings"
	"unicode/utf16"
)

type state struct {
	fg, bg string
	bold   bool
	dim    bool
}

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: ansi2html <input.ansi> <output.html> [title]")
		os.Exit(2)
	}
	raw, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	text := string(raw)
	switch {
	case len(raw) >= 2 && raw[0] == 0xff && raw[1] == 0xfe:
		text = decodeUTF16(raw[2:], binary.LittleEndian)
	case len(raw) >= 2 && raw[0] == 0xfe && raw[1] == 0xff:
		text = decodeUTF16(raw[2:], binary.BigEndian)
	case strings.HasPrefix(text, "\ufeff"):
		text = strings.TrimPrefix(text, "\ufeff")
	}
	title := "mclauncher"
	if len(os.Args) > 3 {
		title = os.Args[3]
	}

	var body strings.Builder
	scanner := bufio.NewScanner(strings.NewReader(text))
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	st := state{}
	for scanner.Scan() {
		line := scanner.Text()
		writeLine(&body, line, &st)
		body.WriteString("\n")
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	page := fmt.Sprintf(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>%s</title>
<style>
  * { box-sizing: border-box; }
  html, body { margin: 0; padding: 0; background: #060607; }
  body {
    min-height: 100vh;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 32px;
    font-family: "Cascadia Mono", "JetBrains Mono", "SFMono-Regular", Consolas, monospace;
  }
  .window {
    background: #0b0b0d;
    border: 1px solid #26262c;
    border-radius: 14px;
    box-shadow: 0 28px 80px rgba(0,0,0,.7), 0 2px 0 rgba(255,255,255,.03) inset;
    overflow: hidden;
  }
  .titlebar {
    height: 40px;
    display: flex;
    align-items: center;
    gap: 9px;
    padding: 0 16px;
    background: #121216;
    border-bottom: 1px solid #232328;
    color: #6f6f79;
    font-size: 12px;
    letter-spacing: .02em;
  }
  .dot { width: 11px; height: 11px; border-radius: 50%%; }
  .red { background: #ff5f57; }
  .yellow { background: #febc2e; }
  .green { background: #28c840; }
  .title { margin-left: 8px; }
  pre {
    margin: 0;
    padding: 20px 22px 24px;
    font-family: "Cascadia Mono", "Cascadia Code", Consolas, "Courier New", monospace;
    font-size: 14px;
    font-variant-ligatures: none;
    line-height: 1.24;
    color: #e6e6e6;
    white-space: pre;
    tab-size: 4;
  }
  .bold { font-weight: 700; }
</style>
</head>
<body>
  <div class="window">
    <div class="titlebar">
      <span class="dot red"></span><span class="dot yellow"></span><span class="dot green"></span>
      <span class="title">%s — 120 × 38</span>
    </div>
    <pre>%s</pre>
  </div>
</body>
</html>
`, html.EscapeString(title), html.EscapeString(title), body.String())
	if err := os.WriteFile(os.Args[2], []byte(page), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func decodeUTF16(raw []byte, order binary.ByteOrder) string {
	units := make([]uint16, len(raw)/2)
	for i := range units {
		units[i] = order.Uint16(raw[i*2:])
	}
	return string(utf16.Decode(units))
}

func writeLine(out *strings.Builder, line string, st *state) {
	var span strings.Builder
	flush := func() {
		if span.Len() == 0 {
			return
		}
		var styles []string
		if st.fg != "" {
			styles = append(styles, "color:"+st.fg)
		}
		if st.bg != "" {
			styles = append(styles, "background:"+st.bg)
		}
		if st.bold {
			styles = append(styles, "font-weight:700")
		}
		if st.dim {
			styles = append(styles, "opacity:.75")
		}
		if len(styles) > 0 {
			out.WriteString(`<span style="` + strings.Join(styles, ";") + `">` + span.String() + `</span>`)
		} else {
			out.WriteString(span.String())
		}
		span.Reset()
	}
	i := 0
	for i < len(line) {
		if line[i] == 0x1b && i+1 < len(line) && line[i+1] == '[' {
			j := i + 2
			for j < len(line) && (line[j] < 'A' || line[j] > 'Z') && (line[j] < 'a' || line[j] > 'z') {
				j++
			}
			if j < len(line) {
				code := line[i+2 : j]
				if line[j] == 'm' {
					flush()
					applySGR(st, code)
				}
				i = j + 1
				continue
			}
		}
		if line[i] == 0x1b {
			// OSC sequences (for example a window title) and truncated escape
			// codes must be consumed so the outer loop always makes progress.
			if i+1 < len(line) && line[i+1] == ']' {
				j := i + 2
				for j < len(line) {
					if line[j] == 0x07 {
						j++
						break
					}
					if line[j] == 0x1b && j+1 < len(line) && line[j+1] == '\\' {
						j += 2
						break
					}
					j++
				}
				i = j
			} else {
				i++
			}
			continue
		}
		start := i
		for i < len(line) && line[i] != 0x1b {
			i++
		}
		span.WriteString(html.EscapeString(line[start:i]))
	}
	flush()
}

func applySGR(st *state, code string) {
	if code == "" {
		code = "0"
	}
	parts := strings.Split(code, ";")
	for k := 0; k < len(parts); k++ {
		n, _ := strconv.Atoi(parts[k])
		switch {
		case n == 0:
			*st = state{}
		case n == 1:
			st.bold = true
		case n == 2:
			st.dim = true
		case n == 22:
			st.bold, st.dim = false, false
		case n >= 30 && n <= 37:
			st.fg = ansi256Or16(n)
		case n >= 90 && n <= 97:
			st.fg = ansi256Or16(n)
		case n >= 40 && n <= 47:
			st.bg = ansi256Or16(n)
		case n >= 100 && n <= 107:
			st.bg = ansi256Or16(n)
		case n == 39:
			st.fg = ""
		case n == 49:
			st.bg = ""
		case (n == 38 || n == 48) && k+1 < len(parts):
			isFg := n == 38
			mode, _ := strconv.Atoi(parts[k+1])
			if mode == 2 && k+4 < len(parts) {
				r, _ := strconv.Atoi(parts[k+2])
				g, _ := strconv.Atoi(parts[k+3])
				b, _ := strconv.Atoi(parts[k+4])
				color := fmt.Sprintf("rgb(%d,%d,%d)", r, g, b)
				if isFg {
					st.fg = color
				} else {
					st.bg = color
				}
				k += 4
			} else if mode == 5 && k+2 < len(parts) {
				idx, _ := strconv.Atoi(parts[k+2])
				color := xterm256(idx)
				if isFg {
					st.fg = color
				} else {
					st.bg = color
				}
				k += 2
			}
		}
	}
}

func ansi256Or16(n int) string {
	palette := map[int]string{
		30: "#1c1c20", 31: "#e06c75", 32: "#7fd88f", 33: "#e5c07b",
		34: "#61afef", 35: "#c678dd", 36: "#56b6c2", 37: "#e6e6e6",
		90: "#5b5b64", 91: "#e06c75", 92: "#7fd88f", 93: "#e5c07b",
		94: "#61afef", 95: "#c678dd", 96: "#56b6c2", 97: "#ffffff",
	}
	if c, ok := palette[n]; ok {
		return c
	}
	if n >= 40 && n <= 47 {
		return ansi256Or16(n - 10)
	}
	if n >= 100 && n <= 107 {
		return ansi256Or16(n - 10)
	}
	return ""
}

func xterm256(idx int) string {
	base := []string{
		"#000000", "#cd3131", "#0dbc79", "#e5e510",
		"#2472c8", "#bc3fbc", "#11a8cd", "#e5e5e5",
		"#666666", "#f14c4c", "#23d18b", "#f5f543",
		"#3b8eea", "#d670d6", "#29b8db", "#ffffff",
	}
	if idx < 16 {
		return base[idx]
	}
	if idx >= 232 {
		v := 8 + (idx-232)*10
		return fmt.Sprintf("rgb(%d,%d,%d)", v, v, v)
	}
	idx -= 16
	levels := []int{0, 95, 135, 175, 215, 255}
	r := levels[idx/36]
	g := levels[(idx/6)%6]
	b := levels[idx%6]
	return fmt.Sprintf("rgb(%d,%d,%d)", r, g, b)
}
