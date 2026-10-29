//go:build windows

package java

import "testing"

func TestWindowsDrivePatterns(t *testing.T) {
	patterns := windowsDrivePatterns(`D:\`)
	want := `D:\Program Files\Java\*\bin\java.exe`
	for _, pattern := range patterns {
		if pattern == want {
			return
		}
	}
	t.Fatalf("windowsDrivePatterns is missing %q: %#v", want, patterns)
}
