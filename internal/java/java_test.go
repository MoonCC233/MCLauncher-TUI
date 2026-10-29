package java

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
