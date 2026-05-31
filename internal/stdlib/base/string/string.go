package stringlib

import (
	"strconv"
	"strings"
	"unicode"
)

func Upper(s string) string { return strings.ToUpper(s) }

func Lower(s string) string { return strings.ToLower(s) }

func Repeat(s string, n int) string { return strings.Repeat(s, n) }

func RepeatJoin(s string, n int, sep string) string {
	parts := make([]string, n)
	for i := range parts {
		parts[i] = s
	}
	return strings.Join(parts, sep)
}

func Reverse(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

func TrimSpace(s string) string { return strings.TrimSpace(s) }

func Trim(s, cutset string) string { return strings.Trim(s, cutset) }

func TrimLeftSpace(s string) string { return strings.TrimLeftFunc(s, unicode.IsSpace) }

func TrimLeft(s, cutset string) string { return strings.TrimLeft(s, cutset) }

func TrimRightSpace(s string) string { return strings.TrimRightFunc(s, unicode.IsSpace) }

func TrimRight(s, cutset string) string { return strings.TrimRight(s, cutset) }

func HasPrefix(s, prefix string) bool { return strings.HasPrefix(s, prefix) }

func HasSuffix(s, suffix string) bool { return strings.HasSuffix(s, suffix) }

func Contains(s, substr string) bool { return strings.Contains(s, substr) }

func Count(s, substr string) int { return strings.Count(s, substr) }

func ReplaceAll(s, old, new string) string { return strings.ReplaceAll(s, old, new) }

func Join(parts []string, sep string) string { return strings.Join(parts, sep) }

func Title(s string) string {
	prev := ' '
	result := make([]rune, 0, len(s))
	for _, r := range s {
		if unicode.IsSpace(prev) {
			result = append(result, unicode.ToUpper(r))
		} else {
			result = append(result, r)
		}
		prev = r
	}
	return string(result)
}

func PadLeft(s string, n int, pad string) string {
	if pad == "" {
		pad = " "
	}
	for len(s) < n {
		s = pad + s
	}
	if len(s) > n {
		s = s[len(s)-n:]
	}
	return s
}

func PadRight(s string, n int, pad string) string {
	if pad == "" {
		pad = " "
	}
	for len(s) < n {
		s += pad
	}
	if len(s) > n {
		s = s[:n]
	}
	return s
}

func IsNumeric(s string) bool {
	trimmed := strings.TrimSpace(s)
	_, err := strconv.ParseFloat(trimmed, 64)
	return err == nil && trimmed != ""
}
