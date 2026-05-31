package utf8x

import "unicode/utf8"

type ValidationReport struct {
	Valid     bool
	ByteCount int
	RuneCount int
	ErrorPos  int
	Error     string
}

func CodepointStarts(s string) []int {
	starts := make([]int, 0, utf8.RuneCountInString(s))
	for i := 0; i < len(s); {
		starts = append(starts, i)
		_, size := utf8.DecodeRuneInString(s[i:])
		if size <= 0 {
			size = 1
		}
		i += size
	}
	return starts
}

func IsContinuationByte(b byte) bool {
	return b >= 0x80 && b <= 0xBF
}

func Validate(s string) ValidationReport {
	report := ValidationReport{
		Valid:     true,
		ByteCount: len(s),
	}
	for pos := 0; pos < len(s); {
		r, size := utf8.DecodeRuneInString(s[pos:])
		if r == utf8.RuneError && size == 1 {
			report.Valid = false
			report.ErrorPos = pos + 1
			report.Error = "invalid UTF-8 encoding"
			return report
		}
		pos += size
		report.RuneCount++
	}
	return report
}

func Sanitize(s, replacement string) string {
	out := make([]byte, 0, len(s))
	for pos := 0; pos < len(s); {
		r, size := utf8.DecodeRuneInString(s[pos:])
		if r == utf8.RuneError && size == 1 {
			out = append(out, replacement...)
			pos++
			continue
		}
		out = append(out, s[pos:pos+size]...)
		pos += size
	}
	return string(out)
}

func Reverse(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

func Sub(s string, i, j int) string {
	runes := []rune(s)

	if i < 0 {
		i = len(runes) + i + 1
	}
	if j < 0 {
		j = len(runes) + j + 1
	}

	if i < 1 {
		i = 1
	}
	if j > len(runes) {
		j = len(runes)
	}
	if i > j+1 {
		return ""
	}

	return string(runes[i-1 : j])
}

func Offset(s string, n, start int64) (int64, bool) {
	starts := CodepointStarts(s)
	if n == 0 {
		pos := int(start) - 1
		if pos < 0 || pos >= len(s) {
			return 0, false
		}
		for i := len(starts) - 1; i >= 0; i-- {
			if starts[i] <= pos {
				return int64(starts[i] + 1), true
			}
		}
		return 0, false
	}

	pos := int(start) - 1
	idx := 0
	for idx < len(starts) && starts[idx] < pos {
		idx++
	}
	var target int
	if n > 0 {
		target = idx + int(n) - 1
	} else {
		target = idx + int(n)
	}
	if target >= 0 && target < len(starts) {
		return int64(starts[target] + 1), true
	}
	if target == len(starts) {
		return int64(len(s) + 1), true
	}
	return 0, false
}
