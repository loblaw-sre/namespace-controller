package utils

import "strings"

// Slug returns a slug representation of the user's email
func Slug(s string) string {
	s = strings.ReplaceAll(s, "_", "-")
	s = strings.ReplaceAll(s, "@", "-")
	s = strings.ReplaceAll(s, ".", "-")
	return s
}
