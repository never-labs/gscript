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
