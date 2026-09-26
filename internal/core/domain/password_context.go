package domain

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

func containsContextualTerm(password string, terms []string) bool {
	normalizedPassword := normalizePasswordText(password)
	for _, term := range terms {
		for _, token := range splitContextualTerm(term) {
			if utf8RuneCount(token) >= 3 && strings.Contains(normalizedPassword, token) {
				return true
			}
		}
	}
	return false
}

func normalizePasswordText(value string) string {
	return strings.ToLower(norm.NFKC.String(value))
}

func splitContextualTerm(value string) []string {
	return strings.FieldsFunc(normalizePasswordText(value), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
}

func utf8RuneCount(value string) int {
	count := 0
	for range value {
		count++
	}
	return count
}
