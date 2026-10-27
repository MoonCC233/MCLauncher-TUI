package auth

import (
	"regexp"
	"strings"
	"testing"
)

func TestValidateUsername(t *testing.T) {
	valid := []string{
		"abc",
		"MoonCC",
		"a_b_1",
		"1234567890123456",
		"___",
	}
	for _, name := range valid {
		if err := ValidateUsername(name); err != nil {
			t.Errorf("ValidateUsername(%q) = %v, want nil", name, err)
		}
	}

	invalid := []string{
		"",
		"ab",
		"12345678901234567",
		"has space",
		"hyphen-name",
		"dot.name",
		"ümlaut",
		"semi;colon",
	}
	for _, name := range invalid {
		if err := ValidateUsername(name); err == nil {
			t.Errorf("ValidateUsername(%q) = nil, want error", name)
		}
	}
}

func TestOfflineUUID(t *testing.T) {
	const notch = "b50ad385-829d-3141-a216-7e7d7539ba7f"
	if got := OfflineUUID("Notch"); got != notch {
		t.Fatalf("OfflineUUID(Notch) = %q, want %q", got, notch)
	}

	// Java's UUID.nameUUIDFromBytes always sets the version to 3 and the
	// RFC 4122 variant bits.
	format := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-3[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	for _, name := range []string{"MoonCC", "Steve", "Alex_99"} {
		got := OfflineUUID(name)
		if !format.MatchString(got) {
			t.Errorf("OfflineUUID(%q) = %q, not a v3 UUID", name, got)
		}
		if again := OfflineUUID(name); again != got {
			t.Errorf("OfflineUUID(%q) is not deterministic: %q != %q", name, got, again)
		}
	}

	if OfflineUUID("Notch") == OfflineUUID("notch") {
		t.Error("offline UUIDs should be case-sensitive")
	}
	if strings.Contains(OfflineUUID("Notch"), "OfflinePlayer") {
		t.Error("offline UUID leaked the salt")
	}
}
