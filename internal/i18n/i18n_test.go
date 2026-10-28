package i18n

import (
	"testing"
	"time"
)

func TestCanonical(t *testing.T) {
	cases := []struct {
		in   string
		want Lang
		ok   bool
	}{
		{"en", EN, true},
		{"en-US", EN, true},
		{"EN_us.UTF-8", EN, true},
		{"zh", ZHHans, true},
		{"zh-CN", ZHHans, true},
		{"zh-Hans", ZHHans, true},
		{"zh-TW", ZHHant, true},
		{"zh-Hant-HK", ZHHant, true},
		{"pt", PTBR, true},
		{"pt-BR", PTBR, true},
		{"pt-PT", PTPT, true},
		{"pt_AO.UTF-8", PTPT, true},
		{"es-MX", ES, true},
		{"fr-CA", FR, true},
		{"de-AT", DE, true},
		{"ja_JP", JA, true},
		{"ar-EG", AR, true},
		{"ru-RU", RU, true},
		{"ko-KR", KO, true},
		{"tr-TR", TR, true},
		{"vi-VN", VI, true},
		{"", "", false},
		{"C", "", false},
		{"xx-YY", "", false},
	}
	for _, tc := range cases {
		got, ok := Canonical(tc.in)
		if ok != tc.ok || got != tc.want {
			t.Errorf("Canonical(%q) = (%q, %v), want (%q, %v)", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

func TestCount(t *testing.T) {
	if got := Count(EN, 1, "instance", "instances"); got != "1 instance" {
		t.Errorf("Count(EN,1) = %q", got)
	}
	if got := Count(EN, 3, "instance", "instances"); got != "3 instances" {
		t.Errorf("Count(EN,3) = %q", got)
	}
	if got := Count(ZHHans, 3, "instance", "instances"); got != "3 个实例" {
		t.Errorf("Count(zh-Hans,3) = %q", got)
	}
}

func TestRelativeTime(t *testing.T) {
	now := time.Now()
	if got := RelativeTime(EN, 0); got != "never" {
		t.Errorf("RelativeTime(0) = %q", got)
	}
	if got := RelativeTime(EN, now.Add(-5*time.Minute).Unix()); got != "5m ago" {
		t.Errorf("RelativeTime(5m) = %q", got)
	}
	if got := RelativeTime(ZHHans, now.Add(-2*time.Hour).Unix()); got != "2 小时前" {
		t.Errorf("RelativeTime(zh,2h) = %q", got)
	}
}

func TestRTL(t *testing.T) {
	if !IsRTL(AR) {
		t.Error("Arabic should be RTL")
	}
	if IsRTL(EN) || IsRTL(ZHHans) {
		t.Error("English and Chinese should be LTR")
	}
}
