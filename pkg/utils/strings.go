package utils

import (
	"crypto/rand"
	"math/big"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func ContainsString(strings []string, target string) bool {
	for _, s := range strings {
		if s == target {
			return true
		}
	}
	return false
}

func RemoveString(strings []string, target string) []string {
	result := []string{}
	for _, s := range strings {
		if s != target {
			result = append(result, s)
		}
	}
	return result
}

func GenerateRandomPassword(length int) (string, error) {
	const asciiPrintableStart = 33
	const asciiPrintableEnd = 126
	const asciiPrintableRange = asciiPrintableEnd - asciiPrintableStart + 1

	result := make([]byte, length)
	for i := 0; i < length; i++ {
		num, err := rand.Int(rand.Reader, big.NewInt(asciiPrintableRange))
		if err != nil {
			return "", err
		}
		result[i] = byte(asciiPrintableStart + num.Int64())
	}
	return string(result), nil
}

func Capitalize(value string) string {
	if len(value) == 0 {
		return ""
	}
	capitalizer := cases.Title(language.English)
	first := ([]rune)(value)[:1]
	tail := ([]rune)(value)[1:]
	return capitalizer.String(string(first)) + string(tail)
}
