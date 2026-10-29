package java

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGlobCandidatesAcceptsFiles(t *testing.T) {
	root := t.TempDir()
	exe := filepath.Join(root, "jdk-21", "bin", javaExe())
	if err := os.MkdirAll(filepath.Dir(exe), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(exe, []byte("java"), 0o755); err != nil {
		t.Fatal(err)
	}
	var out []candidate
	globCandidates([]string{filepath.Join(root, "jdk-*", "bin", javaExe())}, "test", &out)
	if len(out) != 1 || !strings.EqualFold(out[0].path, exe) {
		t.Fatalf("globCandidates = %#v, want %q", out, exe)
	}
}

func TestCandidatesExtraRoot(t *testing.T) {
	root := t.TempDir()
	exe := filepath.Join(root, "temurin-21", "bin", javaExe())
	if err := os.MkdirAll(filepath.Dir(exe), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(exe, []byte("java"), 0o755); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range Candidates(root) {
		if strings.EqualFold(c.path, exe) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Candidates(%q) did not include %q", root, exe)
	}
}

func TestJavaDownloadURLs(t *testing.T) {
	t.Setenv("MCLAUNCHER_JAVA_MIRROR", "https://mirror.test/{url}")
	raw := "https://github.com/adoptium/temurin21-binaries/releases/download/x/y.zip"
	urls := javaDownloadURLs(raw)
	if len(urls) == 0 || urls[0] != strings.ReplaceAll("https://mirror.test/{url}", "{url}", raw) {
		t.Errorf("download URLs = %#v", urls)
	}
	t.Setenv("MCLAUNCHER_JAVA_MIRROR", "")
	local := "https://example.test/java.zip"
	urls = javaDownloadURLs(local)
	if len(urls) != 1 || urls[0] != local {
		t.Errorf("local download URLs = %#v", urls)
	}
}

func TestDownloadJavaVerifiesChecksum(t *testing.T) {
	t.Setenv("MCLAUNCHER_JAVA_MIRROR", "")
	data := bytes.Repeat([]byte("0123456789"), 30000)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "jdk.zip", time.Time{}, bytes.NewReader(data))
	}))
	defer server.Close()
	sum := sha256.Sum256(data)
	dest := filepath.Join(t.TempDir(), "jdk.zip")
	var last int64
	if err := downloadJava(context.Background(), server.Client(), server.URL, dest, int64(len(data)),
		hex.EncodeToString(sum[:]), func(n int64) { last = n }); err != nil {
		t.Fatal(err)
	}
	if last != int64(len(data)) {
		t.Errorf("last progress = %d, want %d", last, len(data))
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, data) {
		t.Error("downloaded data does not match")
	}
}

func TestDownloadJavaResumesPartialFile(t *testing.T) {
	t.Setenv("MCLAUNCHER_JAVA_MIRROR", "")
	data := bytes.Repeat([]byte("abcdefghij"), 20000)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "jdk.zip", time.Time{}, bytes.NewReader(data))
	}))
	defer server.Close()
	dir := t.TempDir()
	dest := filepath.Join(dir, "jdk.zip")
	if err := os.WriteFile(dest+".part", data[:5000], 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	if err := downloadJava(context.Background(), server.Client(), server.URL, dest, int64(len(data)),
		hex.EncodeToString(sum[:]), nil); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, data) {
		t.Error("resumed data does not match")
	}
}

func TestExtractJavaZip(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "jdk.zip")
	f, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	payload := []byte("zip-java")
	w, err := zw.Create("jdk-21/bin/" + javaExe())
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.Write(payload)
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()
	dest := filepath.Join(dir, "jdk")
	if err := extractJavaArchive(archive, dest); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dest, "bin", javaExe()))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(payload) {
		t.Errorf("extracted = %q", got)
	}
}

func TestFindJavaExecutableMacLayout(t *testing.T) {
	root := t.TempDir()
	exe := filepath.Join(root, "Contents", "Home", "bin", javaExe())
	if err := os.MkdirAll(filepath.Dir(exe), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(exe, []byte("java"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := findJavaExecutable(root); got != exe {
		t.Errorf("findJavaExecutable = %q, want %q", got, exe)
	}
}

func TestMajorOf(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"1.8.0_431", 8},
		{"1.8", 8},
		{"1.7.0_80", 7},
		{" 1.8.0 ", 8},
		{"17.0.13", 17},
		{"21", 21},
		{"21-ea", 21},
		{"25.0.1+8", 25},
		{"openjdk", 0},
		{"", 0},
	}
	for _, tc := range cases {
		if got := majorOf(tc.in); got != tc.want {
			t.Errorf("majorOf(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestVendorOf(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"OpenJDK 64-Bit Server VM Temurin-21.0.5+11", "Temurin"},
		{"Eclipse Adoptium", "Adoptium"},
		{"Microsoft build of OpenJDK", "Microsoft"},
		{"Amazon Corretto", "Corretto"},
		{"Azul Zulu", "Zulu"},
		{"BellSoft Liberica", "BellSoft"},
		{"GraalVM CE", "GraalVM"},
		{"Java(TM) SE Runtime Environment (Oracle)", "Oracle"},
		{"OpenJDK Runtime Environment", "OpenJDK"},
		{"Some unknown JDK", ""},
	}
	for _, tc := range cases {
		if got := vendorOf(tc.in); got != tc.want {
			t.Errorf("vendorOf(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"21.0.5", "21.0.1", 1},
		{"17.0.13", "21", -1},
		{"21.0.5", "21.0.5", 0},
		{"1.8.0_431", "1.8.0_441", -1},
		{"21", "21.0.1", -1},
		{"21.0.1", "21", 1},
	}
	for _, tc := range cases {
		if got := compareVersions(tc.a, tc.b); got != tc.want {
			t.Errorf("compareVersions(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestSelect(t *testing.T) {
	java8 := Info{Path: "java8", Version: "1.8.0_431", Major: 8}
	java17 := Info{Path: "java17", Version: "17.0.13", Major: 17}
	java21 := Info{Path: "java21", Version: "21.0.5", Major: 21}
	java22 := Info{Path: "java22", Version: "22.0.1", Major: 22}
	java25 := Info{Path: "java25", Version: "25", Major: 25}

	cases := []struct {
		name     string
		runtimes []Info
		required int
		want     string
		warn     bool
	}{
		{"exact match", []Info{java17, java21}, 21, "java21", false},
		{"java 8", []Info{java21, java8}, 8, "java8", false},
		{"required below 8 clamps", []Info{java8}, 5, "java8", false},
		{"newer runtime with warning", []Info{java17, java22, java25}, 21, "java22", true},
		{"prefer newest build of the nearest major", []Info{
			{Path: "22a", Version: "22.0.9", Major: 22},
			{Path: "22b", Version: "22.0.12", Major: 22},
		}, 21, "22b", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, warning, err := Select(tc.runtimes, tc.required, "")
			if err != nil {
				t.Fatalf("Select: %v", err)
			}
			if got.Path != tc.want {
				t.Errorf("Select = %q, want %q", got.Path, tc.want)
			}
			if (warning != "") != tc.warn {
				t.Errorf("warning = %q, want warning=%v", warning, tc.warn)
			}
		})
	}

	if _, _, err := Select(nil, 21, ""); err == nil || !strings.Contains(err.Error(), "no Java runtime") {
		t.Errorf("Select(no runtimes) error = %v", err)
	}
	if _, _, err := Select([]Info{java17}, 21, ""); err == nil || !strings.Contains(err.Error(), "21 or newer") {
		t.Errorf("Select(too old) error = %v", err)
	}
	if _, _, err := Select([]Info{java21}, 8, ""); err == nil || !strings.Contains(err.Error(), "Java 8") {
		t.Errorf("Select(no Java 8) error = %v", err)
	}

	missing := filepath.Join(t.TempDir(), "no-java.exe")
	if _, _, err := Select([]Info{java21}, 21, missing); err == nil || !strings.Contains(err.Error(), "no Java runtime at") {
		t.Errorf("Select(bad override) error = %v", err)
	}
}

func TestInfoLabels(t *testing.T) {
	info := Info{Version: "21.0.5", Major: 21, Vendor: "Temurin"}
	if got := info.Label(); got != "Java 21.0.5 · Temurin" {
		t.Errorf("Label = %q", got)
	}
	if got := info.Short(); got != "Java 21 Temurin" {
		t.Errorf("Short = %q", got)
	}
	info.Vendor = ""
	if got := info.Label(); got != "Java 21.0.5" {
		t.Errorf("Label(no vendor) = %q", got)
	}
	if got := info.Short(); got != "Java 21" {
		t.Errorf("Short(no vendor) = %q", got)
	}
}
