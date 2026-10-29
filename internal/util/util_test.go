package util

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDownloadFileAbortsStalledBody(t *testing.T) {
	old := downloadStallTimeout
	downloadStallTimeout = 200 * time.Millisecond
	defer func() { downloadStallTimeout = old }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		select {
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
		}
	}))
	defer server.Close()

	dest := filepath.Join(t.TempDir(), "stalled.bin")
	start := time.Now()
	err := DownloadFile(context.Background(), server.Client(), DownloadSpec{
		URL:  server.URL,
		Dest: dest,
		Size: 100,
	}, nil)
	if err == nil {
		t.Fatal("DownloadFile succeeded, want a stall error")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("DownloadFile took %s, the stall watchdog did not fire", elapsed)
	}
}

func TestSafeJoin(t *testing.T) {
	root := t.TempDir()

	got, err := SafeJoin(root, "a/b.txt")
	if err != nil {
		t.Fatalf("SafeJoin(valid) error: %v", err)
	}
	if want := filepath.Join(root, "a", "b.txt"); got != want {
		t.Errorf("SafeJoin = %q, want %q", got, want)
	}

	got, err = SafeJoin(root, `a\b.txt`)
	if err != nil {
		t.Fatalf("SafeJoin(backslashes) error: %v", err)
	}
	if want := filepath.Join(root, "a", "b.txt"); got != want {
		t.Errorf("SafeJoin(backslashes) = %q, want %q", got, want)
	}

	for _, rel := range []string{
		"",
		"/etc/passwd",
		"../evil",
		`..\evil`,
		"a/../../evil",
		".",
		"..",
	} {
		if _, err := SafeJoin(root, rel); err == nil {
			t.Errorf("SafeJoin(%q) = nil error, want rejection", rel)
		}
	}
}

func TestJSONRoundTrip(t *testing.T) {
	type payload struct {
		Name string `json:"name"`
		N    int    `json:"n"`
	}
	path := filepath.Join(t.TempDir(), "nested", "config.json")
	want := payload{Name: "Survival", N: 3}
	if err := WriteJSONAtomic(path, want); err != nil {
		t.Fatalf("WriteJSONAtomic: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.HasSuffix(string(data), "\n") {
		t.Error("JSON file should end with a newline")
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Error("temporary file was not cleaned up")
	}

	var got payload
	if err := ReadJSON(path, &got); err != nil {
		t.Fatalf("ReadJSON: %v", err)
	}
	if got != want {
		t.Errorf("round trip = %+v, want %+v", got, want)
	}

	if err := ReadJSON(filepath.Join(t.TempDir(), "missing.json"), &got); err == nil {
		t.Error("ReadJSON(missing) = nil error, want error")
	}
	bad := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(bad, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ReadJSON(bad, &got); err == nil {
		t.Error("ReadJSON(invalid) = nil error, want error")
	}

	if err := WriteJSONAtomic(filepath.Join(t.TempDir(), "bad.json"), map[string]any{"ch": make(chan int)}); err == nil {
		t.Error("WriteJSONAtomic(unsupported type) = nil error, want error")
	}
}

func TestChecksums(t *testing.T) {
	path := filepath.Join(t.TempDir(), "abc.txt")
	if err := os.WriteFile(path, []byte("abc"), 0o644); err != nil {
		t.Fatal(err)
	}
	const sum = "a9993e364706816aba3e25717850c26c9cd0d89d"

	got, err := SHA1File(path)
	if err != nil {
		t.Fatalf("SHA1File: %v", err)
	}
	if got != sum {
		t.Errorf("SHA1File = %q, want %q", got, sum)
	}

	cases := []struct {
		name   string
		sha    string
		size   int64
		expect bool
	}{
		{"matching", sum, 3, true},
		{"uppercase", strings.ToUpper(sum), 3, true},
		{"size only", "", 3, true},
		{"bad hash", "deadbeef", 3, false},
		{"bad size", sum, 4, false},
	}
	for _, tc := range cases {
		if got := VerifyFile(path, tc.sha, tc.size); got != tc.expect {
			t.Errorf("VerifyFile(%s) = %v, want %v", tc.name, got, tc.expect)
		}
	}
	if VerifyFile(filepath.Join(t.TempDir(), "missing"), "", 0) {
		t.Error("VerifyFile(missing) = true, want false")
	}
	if VerifyFile(t.TempDir(), "", 0) {
		t.Error("VerifyFile(directory) = true, want false")
	}
}

func TestTruncate(t *testing.T) {
	cases := []struct {
		in   string
		max  int
		want string
	}{
		{"hello", 0, ""},
		{"hello", 1, "h"},
		{"hello", 4, "hel…"},
		{"hello", 5, "hello"},
		{"hello", 9, "hello"},
		{"你好世界", 2, "你…"},
		{"你好世界", 4, "你好世界"},
		{"你好世界", 1, "你"},
	}
	for _, tc := range cases {
		if got := Truncate(tc.in, tc.max); got != tc.want {
			t.Errorf("Truncate(%q, %d) = %q, want %q", tc.in, tc.max, got, tc.want)
		}
	}
}

func TestSplitArgs(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"   \t\r\n ", nil},
		{"-Xmx2G -Dfoo=bar", []string{"-Xmx2G", "-Dfoo=bar"}},
		{`-Dfoo="bar baz" plain`, []string{"-Dfoo=bar baz", "plain"}},
		{`-Dx='a b' 'c d'`, []string{"-Dx=a b", "c d"}},
		{`""`, []string{""}},
		{`'unterminated rest`, []string{"unterminated rest"}},
		{"a\tb\nc", []string{"a", "b", "c"}},
	}
	for _, tc := range cases {
		got := SplitArgs(tc.in)
		if len(got) != len(tc.want) {
			t.Errorf("SplitArgs(%q) = %#v, want %#v", tc.in, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("SplitArgs(%q)[%d] = %q, want %q", tc.in, i, got[i], tc.want[i])
			}
		}
	}
}

func TestFormatting(t *testing.T) {
	if got := HumanBytes(0); got != "0 B" {
		t.Errorf("HumanBytes(0) = %q", got)
	}
	if got := HumanBytes(1023); got != "1023 B" {
		t.Errorf("HumanBytes(1023) = %q", got)
	}
	if got := HumanBytes(1024); got != "1.0 KB" {
		t.Errorf("HumanBytes(1024) = %q", got)
	}
	if got := HumanBytes(1536); got != "1.5 KB" {
		t.Errorf("HumanBytes(1536) = %q", got)
	}
	if got := HumanBytes(1024 * 1024); got != "1.0 MB" {
		t.Errorf("HumanBytes(1 MiB) = %q", got)
	}
	if got := HumanBytes(-1); got != "—" {
		t.Errorf("HumanBytes(-1) = %q", got)
	}

	durations := []struct {
		in   time.Duration
		want string
	}{
		{30 * time.Second, "30s"},
		{90 * time.Second, "1m 30s"},
		{time.Hour, "1h 00m"},
		{25 * time.Hour, "1d 01h"},
		{-time.Minute, "0s"},
	}
	for _, tc := range durations {
		if got := HumanDuration(tc.in); got != tc.want {
			t.Errorf("HumanDuration(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestRelativeTime(t *testing.T) {
	now := time.Now()
	cases := []struct {
		ts   int64
		want string
	}{
		{0, "never"},
		{now.Add(-10 * time.Second).Unix(), "just now"},
		{now.Add(-5*time.Minute - 10*time.Second).Unix(), "5m ago"},
		{now.Add(-2*time.Hour - 10*time.Second).Unix(), "2h ago"},
		{now.Add(-3*24*time.Hour - time.Hour).Unix(), "3d ago"},
		{now.Add(10 * time.Second).Unix(), "just now"},
	}
	for _, tc := range cases {
		if got := RelativeTime(tc.ts); got != tc.want {
			t.Errorf("RelativeTime(%d) = %q, want %q", tc.ts, got, tc.want)
		}
	}
	if got := FormatUnix(0); got != "never" {
		t.Errorf("FormatUnix(0) = %q", got)
	}
	if got := FormatUnix(now.Unix()); len(got) != 16 || got[10] != ' ' {
		t.Errorf("FormatUnix(now) = %q, want YYYY-MM-DD HH:MM", got)
	}
}

func TestMiscHelpers(t *testing.T) {
	if got := ShortenPath("short", 10); got != "short" {
		t.Errorf("ShortenPath(short) = %q", got)
	}
	long := `C:\Users\MoonCC\AppData\Roaming\mclauncher\instances\a1b2c3`
	short := ShortenPath(long, 24)
	if !strings.Contains(short, "…") || len(short) >= len(long) {
		t.Errorf("ShortenPath(%q, 24) = %q, want an ellipsized path", long, short)
	}
	if !strings.HasPrefix(short, long[:3]) {
		t.Errorf("ShortenPath lost its prefix: %q", short)
	}

	sizes := []struct {
		in   string
		w, h int
		ok   bool
	}{
		{"854x480", 854, 480, true},
		{"854×480", 854, 480, true},
		{"1920X1080", 1920, 1080, true},
		{"319x480", 0, 0, false},
		{"854x239", 0, 0, false},
		{"8000x1080", 0, 0, false},
		{"abc", 0, 0, false},
		{"", 0, 0, false},
	}
	for _, tc := range sizes {
		w, h, ok := ParseWindowSize(tc.in)
		if w != tc.w || h != tc.h || ok != tc.ok {
			t.Errorf("ParseWindowSize(%q) = (%d,%d,%v), want (%d,%d,%v)", tc.in, w, h, ok, tc.w, tc.h, tc.ok)
		}
	}

	clamps := []struct{ v, lo, hi, want int }{
		{-1, 0, 10, 0},
		{5, 0, 10, 5},
		{11, 0, 10, 10},
	}
	for _, tc := range clamps {
		if got := Clamp(tc.v, tc.lo, tc.hi); got != tc.want {
			t.Errorf("Clamp(%d,%d,%d) = %d, want %d", tc.v, tc.lo, tc.hi, got, tc.want)
		}
	}
}
