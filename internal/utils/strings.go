package utils

import (
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func Capitalize(value string) string {
	if len(value) == 0 {
		return ""
	}
	capitalizer := cases.Title(language.English)
	first := ([]rune)(value)[:1]
	tail := ([]rune)(value)[1:]
	return capitalizer.String(string(first)) + string(tail)
}
