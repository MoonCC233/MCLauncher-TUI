package version

import "testing"

func TestFull(t *testing.T) {
	oldVersion, oldCommit := Version, Commit
	defer func() {
		Version = oldVersion
		Commit = oldCommit
	}()

	Version = "1.2.3"
	Commit = "none"
	if got := Full(); got != "1.2.3" {
		t.Errorf("Full() = %q, want %q", got, "1.2.3")
	}
	Commit = ""
	if got := Full(); got != "1.2.3" {
		t.Errorf("Full(empty commit) = %q, want %q", got, "1.2.3")
	}
	Commit = "abcdef123456"
	if got := Full(); got != "1.2.3 (abcdef1)" {
		t.Errorf("Full(long commit) = %q, want %q", got, "1.2.3 (abcdef1)")
	}
	Commit = "abc"
	if got := Full(); got != "1.2.3 (abc)" {
		t.Errorf("Full(short commit) = %q, want %q", got, "1.2.3 (abc)")
	}
}
