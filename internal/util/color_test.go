package util

import "testing"

func TestNormalizeHexColor(t *testing.T) {
	tests := []struct {
		in   string
		want string
		ok   bool
	}{
		{"#fab283", "#FAB283", true},
		{"FAB283", "#FAB283", true},
		{" #abc ", "#AABBCC", true},
		{"abc", "#AABBCC", true},
		{"", "", false},
		{"#abcd", "", false},
		{"#gggggg", "", false},
		{"0x1234", "", false},
		{"#12345", "", false},
	}
	for _, test := range tests {
		got, ok := NormalizeHexColor(test.in)
		if got != test.want || ok != test.ok {
			t.Errorf("NormalizeHexColor(%q) = %q, %v; want %q, %v", test.in, got, ok, test.want, test.ok)
		}
	}
}

func TestShadeHex(t *testing.T) {
	if got := ShadeHex("#fab283", 0.5); got != "#7D5941" {
		t.Errorf("ShadeHex half = %q, want #7D5941", got)
	}
	if got := ShadeHex("#808080", 2); got != "#FFFFFF" {
		t.Errorf("ShadeHex clamps bright values: %q", got)
	}
	if got := ShadeHex("#808080", -1); got != "#000000" {
		t.Errorf("ShadeHex clamps negative values: %q", got)
	}
	if got := ShadeHex("not-a-color", 0.5); got != "not-a-color" {
		t.Errorf("ShadeHex should preserve invalid colors: %q", got)
	}
}
