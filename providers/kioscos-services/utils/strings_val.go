package utils

import "strings"

func IsEmptyString(text string) bool {
	return strings.TrimSpace(text) == ""
}
