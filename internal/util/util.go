package util

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func EnsureDir(path string) error {
	return os.MkdirAll(path, 0o755)
}

func ReadJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	return nil
}

// WriteJSONAtomic writes JSON through a temporary file and renames it into
// place, so a crash never leaves a half-written config behind.
func WriteJSONAtomic(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return WriteFileAtomic(path, data, 0o644)
}

// WriteFileAtomic writes data through a temporary file and renames it into
// place. Windows keeps the target open for a moment now and then (antivirus,
// search indexer, a second launcher reading it), which makes the rename fail
// even though nothing is wrong with the file; retry briefly and then fall back
// to writing in place so a transient lock does not lose the user's data.
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	if err := EnsureDir(filepath.Dir(path)); err != nil {
		return err
	}
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		for attempt := 0; attempt < 5; attempt++ {
			time.Sleep(time.Duration(20*(attempt+1)) * time.Millisecond)
			if retryErr := os.Rename(tmp, path); retryErr == nil {
				return nil
			}
		}
		if writeErr := os.WriteFile(path, data, perm); writeErr != nil {
			_ = os.Remove(tmp)
			return fmt.Errorf("write %s: %w", path, err)
		}
		_ = os.Remove(tmp)
	}
	return nil
}

func SHA1File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha1.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// VerifyFile checks size first (cheap) then SHA-1. An empty checksum skips the
// hash comparison.
func VerifyFile(path, wantSHA1 string, wantSize int64) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	if wantSize > 0 && info.Size() != wantSize {
		return false
	}
	if wantSHA1 == "" {
		return true
	}
	got, err := SHA1File(path)
	if err != nil {
		return false
	}
	return strings.EqualFold(got, wantSHA1)
}

// SafeJoin joins a relative download path to root and guarantees the result
// stays inside root.
func SafeJoin(root, rel string) (string, error) {
	rel = strings.ReplaceAll(rel, "\\", "/")
	if rel == "" || strings.HasPrefix(rel, "/") {
		return "", fmt.Errorf("unsafe path %q", rel)
	}
	clean := filepath.Clean(filepath.FromSlash(rel))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || filepath.IsAbs(clean) {
		return "", fmt.Errorf("unsafe path %q", rel)
	}
	joined := filepath.Join(root, clean)
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	absJoined, err := filepath.Abs(joined)
	if err != nil {
		return "", err
	}
	if absJoined != absRoot && !strings.HasPrefix(absJoined, absRoot+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes root: %q", rel)
	}
	return joined, nil
}

func TimeNow() int64 { return time.Now().Unix() }

func FormatUnix(ts int64) string {
	if ts <= 0 {
		return "never"
	}
	return time.Unix(ts, 0).Local().Format("2006-01-02 15:04")
}

func RelativeTime(ts int64) string {
	if ts <= 0 {
		return "never"
	}
	d := time.Since(time.Unix(ts, 0))
	if d < 0 {
		d = -d
	}
	switch {
	case d < 45*time.Second:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	case d < 365*24*time.Hour:
		return time.Unix(ts, 0).Format("Jan 02")
	default:
		return time.Unix(ts, 0).Format("2006-01-02")
	}
}

func HumanDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	seconds := int(d.Seconds())
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}
	minutes := seconds / 60
	if minutes < 60 {
		return fmt.Sprintf("%dm %02ds", minutes, seconds%60)
	}
	hours := minutes / 60
	if hours < 24 {
		return fmt.Sprintf("%dh %02dm", hours, minutes%60)
	}
	return fmt.Sprintf("%dd %02dh", hours/24, hours%24)
}

func HumanBytes(n int64) string {
	if n < 0 {
		return "—"
	}
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	units := []string{"KB", "MB", "GB", "TB"}
	return fmt.Sprintf("%.1f %s", float64(n)/float64(div), units[exp])
}

func Truncate(s string, max int) string {
	if max <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max <= 1 {
		return string(r[:max])
	}
	return string(r[:max-1]) + "…"
}

func ShortenPath(path string, width int) string {
	if len(path) <= width {
		return path
	}
	if width <= 4 {
		return Truncate(path, width)
	}
	half := (width - 3) / 2
	return path[:half] + "…" + path[len(path)-(width-3-half):]
}

func SplitArgs(s string) []string {
	var (
		args    []string
		current strings.Builder
		quote   rune
		started bool
	)
	flush := func() {
		if started {
			args = append(args, current.String())
			current.Reset()
			started = false
		}
	}
	for _, r := range s {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				current.WriteRune(r)
			}
			started = true
		case r == '"' || r == '\'':
			quote = r
			started = true
		case r == ' ' || r == '\t' || r == '\n' || r == '\r':
			flush()
		default:
			current.WriteRune(r)
			started = true
		}
	}
	flush()
	return args
}

func ParseWindowSize(s string) (int, int, bool) {
	var w, h int
	if _, err := fmt.Sscanf(strings.TrimSpace(strings.ToLower(strings.ReplaceAll(s, "×", "x"))), "%dx%d", &w, &h); err != nil {
		return 0, 0, false
	}
	if w < 320 || h < 240 || w > 7680 || h > 4320 {
		return 0, 0, false
	}
	return w, h, true
}

func Clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

var ErrNotInstalled = errors.New("not installed")
