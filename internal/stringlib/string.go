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

func JoinProjectedLen(parts []string, sep string) int {
	total := 0
	for _, part := range parts {
		total += len(part)
	}
	if len(parts) > 1 {
		total += len(sep) * (len(parts) - 1)
	}
	return total
}

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

func LuaSub(s string, start, end int64, hasEnd bool) string {
	slen := len(s)
	i := int(start)
	j := slen
	if hasEnd {
		j = int(end)
	}
	if i < 0 {
		i = slen + i + 1
	}
	if i < 1 {
		i = 1
	}
	if j < 0 {
		j = slen + j + 1
	}
	if j > slen {
		j = slen
	}
	if i > j {
		return ""
	}
	return s[i-1 : j]
}

// LuaByteRange converts Lua's 1-based inclusive byte range into zero-based
// inclusive indexes. The bool is false when the normalized range is empty.
func LuaByteRange(s string, start, end int64, hasEnd bool) (int, int, bool) {
	slen := len(s)
	i := int(start)
	j := i
	if hasEnd {
		j = int(end)
	}
	if i < 0 {
		i = slen + i + 1
	}
	if j < 0 {
		j = slen + j + 1
	}
	if i < 1 {
		i = 1
	}
	if j > slen {
		j = slen
	}
	if i > j {
		return 0, 0, false
	}
	return i - 1, j - 1, true
}

func LuaByteAt(s string, index int64) (byte, bool) {
	start, _, ok := LuaByteRange(s, index, index, false)
	if !ok || start >= len(s) {
		return 0, false
	}
	return s[start], true
}

func LuaBytes(s string, start, end int64, hasEnd bool) ([]byte, bool) {
	i, j, ok := LuaByteRange(s, start, end, hasEnd)
	if !ok {
		return nil, false
	}
	buf := make([]byte, 0, j-i+1)
	for k := i; k <= j; k++ {
		buf = append(buf, s[k])
	}
	return buf, true
}

func CharBytes(values []int64) ([]byte, bool) {
	buf := make([]byte, 0, len(values))
	for _, n := range values {
		if n < 0 || n > 255 {
			return nil, false
		}
		buf = append(buf, byte(n))
	}
	return buf, true
}

func LuaSearchStart(s string, init int64) (int, string, bool) {
	start := int(init)
	if start < 0 {
		start = len(s) + start + 1
	}
	if start < 1 {
		start = 1
	}
	if start > len(s)+1 {
		return 0, "", false
	}
	return start, s[start-1:], true
}

func Split(s, sep string) []string {
	parts := make([]string, 0, 8)
	SplitEach(s, sep, func(part string) {
		parts = append(parts, part)
	})
	return parts
}

func SplitEach(s, sep string, yield func(string)) {
	if sep == "" {
		for i := 0; i < len(s); i++ {
			yield(string(s[i]))
		}
		return
	}
	if len(sep) != 1 {
		start := 0
		for {
			next := strings.Index(s[start:], sep)
			if next < 0 {
				yield(s[start:])
				return
			}
			end := start + next
			yield(s[start:end])
			start = end + len(sep)
		}
	}
	sepByte := sep[0]
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] != sepByte {
			continue
		}
		yield(s[start:i])
		start = i + 1
	}
	yield(s[start:])
}

func SplitProject(s, sep string, index int64) (string, bool) {
	if sep == "" {
		i := int(index) - 1
		if i < 0 || i >= len(s) {
			return "", false
		}
		return string(s[i]), true
	}

	token := int64(1)
	start := 0
	if len(sep) == 1 {
		sepByte := sep[0]
		for i := 0; i < len(s); i++ {
			if s[i] != sepByte {
				continue
			}
			if token == index {
				return s[start:i], true
			}
			token++
			start = i + 1
		}
		if token == index {
			return s[start:], true
		}
		return "", false
	}

	for {
		next := strings.Index(s[start:], sep)
		if next < 0 {
			if token == index {
				return s[start:], true
			}
			return "", false
		}
		end := start + next
		if token == index {
			return s[start:end], true
		}
		token++
		start = end + len(sep)
	}
}
