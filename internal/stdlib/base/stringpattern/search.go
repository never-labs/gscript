package stringpattern

// ScanASCIIDigits returns the first byte offset at or after pos that is not an
// ASCII decimal digit. Negative positions are treated as zero.
func ScanASCIIDigits(s string, pos int) int {
	if pos < 0 {
		pos = 0
	}
	for pos < len(s) && IsASCIIDigit(s[pos]) {
		pos++
	}
	return pos
}

func IsASCIIDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

func HasStringAt(s string, pos int, needle string) bool {
	return pos >= 0 && pos <= len(s) && len(needle) <= len(s)-pos && s[pos:pos+len(needle)] == needle
}

// NextSearchStart advances gmatch/gsub search position after a match.
// Zero-width matches move by one byte, matching the existing Lua-pattern
// runtime behavior.
func NextSearchStart(s string, start, end int) int {
	if end != start {
		return end
	}
	if end >= len(s) {
		return len(s) + 1
	}
	return end + 1
}

func ParseStandaloneBalancedPattern(pattern string) (bool, byte, byte) {
	if len(pattern) != 4 || pattern[0] != '%' || pattern[1] != 'b' {
		return false, 0, 0
	}
	return true, pattern[2], pattern[3]
}

func FindBalancedRange(s string, open, close byte, from int) []int {
	for i := from; i < len(s); i++ {
		if s[i] != open {
			continue
		}
		if open == close {
			for j := i + 1; j < len(s); j++ {
				if s[j] == close {
					return []int{i, j + 1}
				}
			}
			return nil
		}
		depth := 1
		for j := i + 1; j < len(s); j++ {
			if s[j] == open {
				depth++
			}
			if s[j] == close {
				depth--
				if depth == 0 {
					return []int{i, j + 1}
				}
			}
		}
		return nil
	}
	return nil
}

func FindAllBalancedRanges(s string, open, close byte) [][]int {
	var ranges [][]int
	next := 0
	for next < len(s) {
		loc := FindBalancedRange(s, open, close, next)
		if loc == nil {
			break
		}
		ranges = append(ranges, loc)
		if loc[1] > next {
			next = loc[1]
		} else {
			next++
		}
	}
	return ranges
}
