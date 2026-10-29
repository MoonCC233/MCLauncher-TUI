package util

import (
	"fmt"
	"strconv"
	"strings"
)

// NormalizeHexColor accepts "#rgb", "#rrggbb", "rgb" or "rrggbb" and returns
// an uppercase "#RRGGBB" string.
func NormalizeHexColor(value string) (string, bool) {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "#")
	if len(value) == 3 {
		value = string([]byte{value[0], value[0], value[1], value[1], value[2], value[2]})
	}
	if len(value) != 6 {
		return "", false
	}
	if strings.Trim(value, "0123456789abcdefABCDEF") != "" {
		return "", false
	}
	return "#" + strings.ToUpper(value), true
}

// ShadeHex multiplies a #RRGGBB color by factor and clamps the result.
func ShadeHex(color string, factor float64) string {
	normalized, ok := NormalizeHexColor(color)
	if !ok {
		return color
	}
	if factor < 0 {
		factor = 0
	}
	value, _ := strconv.ParseUint(strings.TrimPrefix(normalized, "#"), 16, 32)
	red := int(value>>16) & 0xff
	green := int(value>>8) & 0xff
	blue := int(value) & 0xff
	scale := func(component int) int {
		result := int(float64(component) * factor)
		if result < 0 {
			return 0
		}
		if result > 255 {
			return 255
		}
		return result
	}
	return fmt.Sprintf("#%02X%02X%02X", scale(red), scale(green), scale(blue))
}
