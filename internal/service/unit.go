package service

import (
	"strings"
	"unicode/utf8"
)

func normalizeUnit(unit string) (string, bool) {
	unit = strings.TrimSpace(unit)
	return unit, unit != "" && utf8.RuneCountInString(unit) <= 50
}
