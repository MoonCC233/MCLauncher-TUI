//go:build windows

package i18n

import "golang.org/x/sys/windows"

func systemLocale() string {
	if langs, err := windows.GetUserPreferredUILanguages(windows.MUI_LANGUAGE_NAME); err == nil && len(langs) > 0 {
		return langs[0]
	}
	if langs, err := windows.GetSystemPreferredUILanguages(windows.MUI_LANGUAGE_NAME); err == nil && len(langs) > 0 {
		return langs[0]
	}
	return ""
}
