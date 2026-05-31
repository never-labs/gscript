package stringlib

import (
	"math"
	"strconv"
	"strings"
)

func IsFormatFlag(b byte) bool {
	switch b {
	case '-', '+', ' ', '#', '0':
		return true
	default:
		return false
	}
}

func WriteFastIntegerFormat(buf *strings.Builder, fmtSpec string, spec byte, n int64) bool {
	if len(fmtSpec) < 2 || fmtSpec[0] != '%' || fmtSpec[len(fmtSpec)-1] != spec {
		return false
	}
	pos := 1
	pad := byte(' ')
	if pos < len(fmtSpec)-1 && fmtSpec[pos] == '0' {
		pad = '0'
		pos++
	}
	width := 0
	for pos < len(fmtSpec)-1 && fmtSpec[pos] >= '0' && fmtSpec[pos] <= '9' {
		width = width*10 + int(fmtSpec[pos]-'0')
		pos++
	}
	if pos != len(fmtSpec)-1 {
		return false
	}

	var scratch [64]byte
	digits := scratch[:0]
	switch spec {
	case 'd', 'i', 'u':
		digits = strconv.AppendInt(digits, n, 10)
	case 'x':
		digits = strconv.AppendInt(digits, n, 16)
	case 'X':
		digits = strconv.AppendInt(digits, n, 16)
		upperHexDigits(digits)
	case 'o':
		digits = strconv.AppendInt(digits, n, 8)
	default:
		return false
	}

	writePaddedDigits(buf, digits, pad, width)
	return true
}

func WritePaddedInteger(buf *strings.Builder, verb byte, pad byte, width int, n int64) {
	var scratch [64]byte
	digits := scratch[:0]
	switch verb {
	case 'd', 'i', 'u':
		digits = strconv.AppendInt(digits, n, 10)
	case 'x':
		digits = strconv.AppendInt(digits, n, 16)
	case 'X':
		digits = strconv.AppendInt(digits, n, 16)
		upperHexDigits(digits)
	case 'o':
		digits = strconv.AppendInt(digits, n, 8)
	default:
		return
	}
	writePaddedDigits(buf, digits, pad, width)
}

func LuaQuoteNil() string { return "nil" }

func LuaQuoteBool(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

func LuaQuoteInt(v int64) string {
	return strconv.FormatInt(v, 10)
}

func LuaQuoteFloat(v float64) string {
	if math.IsInf(v, 1) {
		return "1e9999"
	}
	if math.IsInf(v, -1) {
		return "-1e9999"
	}
	if math.IsNaN(v) {
		return "(0/0)"
	}
	return strconv.FormatFloat(v, 'g', -1, 64)
}

func LuaQuoteString(s string) string {
	var buf strings.Builder
	buf.WriteByte('"')
	for _, c := range s {
		switch c {
		case '"':
			buf.WriteString(`\"`)
		case '\\':
			buf.WriteString(`\\`)
		case '\n':
			buf.WriteString(`\n`)
		case '\r':
			buf.WriteString(`\r`)
		case '\000':
			buf.WriteString(`\0`)
		default:
			buf.WriteRune(c)
		}
	}
	buf.WriteByte('"')
	return buf.String()
}

func upperHexDigits(digits []byte) {
	for i, b := range digits {
		if b >= 'a' && b <= 'f' {
			digits[i] = b - ('a' - 'A')
		}
	}
}

func writePaddedDigits(buf *strings.Builder, digits []byte, pad byte, width int) {
	if width <= len(digits) {
		buf.Write(digits)
		return
	}
	padCount := width - len(digits)
	if pad == '0' && len(digits) > 0 && digits[0] == '-' {
		buf.WriteByte('-')
		for i := 0; i < padCount; i++ {
			buf.WriteByte('0')
		}
		buf.Write(digits[1:])
		return
	}
	for i := 0; i < padCount; i++ {
		buf.WriteByte(pad)
	}
	buf.Write(digits)
}
