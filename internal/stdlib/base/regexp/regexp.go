package regexp

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

func FastFindSubmatchIndex(pattern, s string) ([]int, bool) {
	switch pattern {
	case "([a-z]+)=([a-z0-9/]+)":
	default:
		return nil, false
	}
	locs := fastFindAllKeyValueSubmatchIndex(s, 1)
	if len(locs) == 0 {
		return nil, true
	}
	return locs[0], true
}

func FastFindAllSubmatchIndex(pattern, s string, n int) ([][]int, bool) {
	switch pattern {
	case "([a-z]+)=([a-z0-9/]+)":
		return fastFindAllKeyValueSubmatchIndex(s, n), true
	default:
		return nil, false
	}
}

func FastFindString(pattern, s string) (string, bool, bool) {
	switch pattern {
	case "[0-9]+", "\\d+":
		start, end := firstASCIIRun(s, isASCIIDigit)
		if start < 0 {
			return "", false, true
		}
		return s[start:end], true, true
	default:
		return "", false, false
	}
}

func FastFindAllStrings(pattern, s string, n int) ([]string, bool) {
	var pred func(byte) bool
	switch pattern {
	case "[0-9]+", "\\d+":
		pred = isASCIIDigit
	default:
		return nil, false
	}
	if n == 0 {
		return nil, true
	}
	out := make([]string, 0, 4)
	for pos := 0; pos < len(s) && (n < 0 || len(out) < n); {
		start, end := firstASCIIRun(s[pos:], pred)
		if start < 0 {
			break
		}
		start += pos
		end += pos
		out = append(out, s[start:end])
		pos = end
	}
	return out, true
}

func FastReplaceAllString(pattern, s, repl string) (string, bool) {
	switch pattern {
	case "[0-9]+", "\\d+":
	default:
		return "", false
	}
	start, end := firstASCIIRun(s, isASCIIDigit)
	if start < 0 {
		return s, true
	}
	var b strings.Builder
	b.Grow(len(s))
	pos := 0
	for start >= 0 {
		b.WriteString(s[pos:start])
		b.WriteString(repl)
		pos = end
		nextStart, nextEnd := firstASCIIRun(s[pos:], isASCIIDigit)
		if nextStart < 0 {
			break
		}
		start = pos + nextStart
		end = pos + nextEnd
	}
	b.WriteString(s[pos:])
	return b.String(), true
}

func FastSplitStrings(pattern, s string, n int) ([]string, bool) {
	switch pattern {
	case "\\s+", "[[:space:]]+":
	default:
		return nil, false
	}
	if n == 0 {
		return nil, true
	}
	out := make([]string, 0, 4)
	pos := 0
	for pos <= len(s) && (n < 0 || len(out)+1 < n) {
		start, end := firstSpaceRun(s[pos:])
		if start < 0 {
			break
		}
		start += pos
		end += pos
		out = append(out, s[pos:start])
		pos = end
	}
	out = append(out, s[pos:])
	return out, true
}

func fastFindAllKeyValueSubmatchIndex(s string, n int) [][]int {
	if n == 0 {
		return nil
	}
	out := make([][]int, 0, 4)
	for pos := 0; pos < len(s) && (n < 0 || len(out) < n); {
		keyStart := -1
		for pos < len(s) {
			if isASCIILower(s[pos]) {
				keyStart = pos
				break
			}
			pos++
		}
		if keyStart < 0 {
			break
		}
		keyEnd := keyStart + 1
		for keyEnd < len(s) && isASCIILower(s[keyEnd]) {
			keyEnd++
		}
		if keyEnd >= len(s) || s[keyEnd] != '=' {
			pos = keyStart + 1
			continue
		}
		valueStart := keyEnd + 1
		valueEnd := valueStart
		for valueEnd < len(s) && isASCIILowerDigitSlash(s[valueEnd]) {
			valueEnd++
		}
		if valueEnd == valueStart {
			pos = keyStart + 1
			continue
		}
		out = append(out, []int{keyStart, valueEnd, keyStart, keyEnd, valueStart, valueEnd})
		pos = valueEnd
	}
	return out
}

func firstASCIIRun(s string, pred func(byte) bool) (int, int) {
	for i := 0; i < len(s); i++ {
		if !pred(s[i]) {
			continue
		}
		j := i + 1
		for j < len(s) && pred(s[j]) {
			j++
		}
		return i, j
	}
	return -1, -1
}

func firstSpaceRun(s string) (int, int) {
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if !unicode.IsSpace(r) {
			i += size
			continue
		}
		j := i + size
		for j < len(s) {
			r, size = utf8.DecodeRuneInString(s[j:])
			if !unicode.IsSpace(r) {
				break
			}
			j += size
		}
		return i, j
	}
	return -1, -1
}

func isASCIIDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

func isASCIILower(b byte) bool {
	return b >= 'a' && b <= 'z'
}

func isASCIILowerDigitSlash(b byte) bool {
	return isASCIILower(b) || isASCIIDigit(b) || b == '/'
}
