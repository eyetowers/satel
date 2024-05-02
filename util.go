package satel

import (
	"unicode"
)

func isUserCodeValid(s string) bool {
	if len(s) != 4 {
		return false
	}

	for _, char := range s {
		if !unicode.IsDigit(char) {
			return false
		}
	}
	return true
}
