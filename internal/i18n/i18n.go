// Package i18n provides the UI language catalog and locale detection for
// mclauncher. English strings are used as translation keys, so a missing
// translation always falls back to readable English instead of a raw key.
package i18n

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type Lang string

const (
	EN     Lang = "en"
	ES     Lang = "es"
	FR     Lang = "fr"
	DE     Lang = "de"
	ZHHans Lang = "zh-Hans"
	ZHHant Lang = "zh-Hant"
	JA     Lang = "ja"
	AR     Lang = "ar"
	PTBR   Lang = "pt-BR"
	PTPT   Lang = "pt-PT"
	RU     Lang = "ru"
	KO     Lang = "ko"
	TR     Lang = "tr"
	VI     Lang = "vi"
)

type Info struct {
	Code    Lang
	English string
	Native  string
}

// Supported is the ordered list shown in the language picker.
var Supported = []Info{
	{EN, "English", "English"},
	{ES, "Spanish", "Español"},
	{FR, "French", "Français"},
	{DE, "German", "Deutsch"},
	{ZHHans, "Simplified Chinese", "简体中文"},
	{ZHHant, "Traditional Chinese", "繁體中文"},
	{JA, "Japanese", "日本語"},
	{AR, "Arabic", "العربية"},
	{PTBR, "Portuguese (Brazil)", "Português (Brasil)"},
	{PTPT, "Portuguese (Portugal)", "Português (Portugal)"},
	{RU, "Russian", "Русский"},
	{KO, "Korean", "한국어"},
	{TR, "Turkish", "Türkçe"},
	{VI, "Vietnamese", "Tiếng Việt"},
}

var byCode = func() map[Lang]Info {
	m := make(map[Lang]Info, len(Supported))
	for _, info := range Supported {
		m[info.Code] = info
	}
	return m
}()

// Canonical converts a locale tag (en-US, zh_CN.UTF-8, pt-BR, ...) to a
// supported language code.
func Canonical(tag string) (Lang, bool) {
	s := strings.ToLower(strings.TrimSpace(tag))
	if i := strings.IndexAny(s, ".@"); i >= 0 {
		s = s[:i]
	}
	s = strings.ReplaceAll(s, "_", "-")
	switch {
	case s == "" || s == "c" || s == "posix":
		return "", false
	case s == "zh" || strings.HasPrefix(s, "zh-hans") || strings.HasPrefix(s, "zh-cn") ||
		strings.HasPrefix(s, "zh-sg") || strings.HasPrefix(s, "zh-my"):
		return ZHHans, true
	case strings.HasPrefix(s, "zh-hant") || strings.HasPrefix(s, "zh-tw") ||
		strings.HasPrefix(s, "zh-hk") || strings.HasPrefix(s, "zh-mo"):
		return ZHHant, true
	case s == "pt" || strings.HasPrefix(s, "pt-br"):
		return PTBR, true
	case strings.HasPrefix(s, "pt-pt") || strings.HasPrefix(s, "pt-ao") ||
		strings.HasPrefix(s, "pt-mz") || strings.HasPrefix(s, "pt-cv") ||
		strings.HasPrefix(s, "pt-gw") || strings.HasPrefix(s, "pt-st") ||
		strings.HasPrefix(s, "pt-tl"):
		return PTPT, true
	}
	for _, info := range Supported {
		base := string(info.Code)
		if strings.HasPrefix(base, "zh-") {
			continue
		}
		if s == strings.ToLower(base) || strings.HasPrefix(s, strings.ToLower(base)+"-") {
			return info.Code, true
		}
	}
	return "", false
}

// Detect returns the best supported language for the current OS locale.
func Detect() Lang {
	for _, key := range []string{"LC_ALL", "LC_MESSAGES", "LANG", "LANGUAGE"} {
		if value := os.Getenv(key); value != "" {
			for _, part := range strings.Split(value, ":") {
				if lang, ok := Canonical(part); ok {
					return lang
				}
			}
		}
	}
	if name := systemLocale(); name != "" {
		if lang, ok := Canonical(name); ok {
			return lang
		}
	}
	return EN
}

// Resolve returns the explicit language when valid, otherwise the detected one.
func Resolve(setting string) Lang {
	if setting != "" {
		if lang, ok := Canonical(setting); ok {
			return lang
		}
	}
	return Detect()
}

func InfoFor(lang Lang) Info {
	if info, ok := byCode[lang]; ok {
		return info
	}
	return byCode[EN]
}

func NativeName(lang Lang) string { return InfoFor(lang).Native }

func IsRTL(lang Lang) bool { return lang == AR }

func Direction(lang Lang) string {
	if IsRTL(lang) {
		return "rtl"
	}
	return "ltr"
}

var fallbacks = map[Lang][]Lang{
	PTPT: {PTBR},
}

// T translates key for lang. English keys double as the default text.
func T(lang Lang, key string) string {
	if lang == "" || lang == EN {
		return key
	}
	if catalog := catalogs[lang]; catalog != nil {
		if value := catalog[key]; value != "" {
			return value
		}
	}
	for _, fallback := range fallbacks[lang] {
		if catalog := catalogs[fallback]; catalog != nil {
			if value := catalog[key]; value != "" {
				return value
			}
		}
	}
	return key
}

// Has reports whether a non-empty translation exists.
func Has(lang Lang, key string) bool {
	if lang == "" || lang == EN {
		return true
	}
	if catalog := catalogs[lang]; catalog != nil && catalog[key] != "" {
		return true
	}
	for _, fallback := range fallbacks[lang] {
		if catalog := catalogs[fallback]; catalog != nil && catalog[key] != "" {
			return true
		}
	}
	return false
}

// TF formats a translated template. Placeholders follow fmt.Sprintf.
func TF(lang Lang, key string, args ...any) string {
	return fmt.Sprintf(T(lang, key), args...)
}

// Count renders a number followed by a translated noun. English passes the
// singular/plural pair; other catalogs translate either form (or both to the
// same word, which is the norm for CJK languages).
func Count(lang Lang, n int, one, many string) string {
	word := many
	if n == 1 {
		word = one
	}
	return fmt.Sprintf("%d %s", n, T(lang, word))
}

// SupportedCode reports whether code is one of the selectable languages.
func SupportedCode(code Lang) bool {
	_, ok := byCode[code]
	return ok
}

// RelativeTime is the localized counterpart of util.RelativeTime.
func RelativeTime(lang Lang, ts int64) string {
	if ts <= 0 {
		return T(lang, "never")
	}
	d := time.Since(time.Unix(ts, 0))
	if d < 0 {
		d = -d
	}
	switch {
	case d < 45*time.Second:
		return T(lang, "just now")
	case d < time.Hour:
		return TF(lang, "%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return TF(lang, "%dh ago", int(d.Hours()))
	case d < 7*24*time.Hour:
		return TF(lang, "%dd ago", int(d.Hours()/24))
	default:
		return time.Unix(ts, 0).Format("2006-01-02")
	}
}
