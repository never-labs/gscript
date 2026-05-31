package stringlib

import (
	"strconv"
	"strings"
)

type DecimalNumberKind uint8

const (
	DecimalNumberInt DecimalNumberKind = iota
	DecimalNumberFloat
)

type DecimalNumber struct {
	Kind  DecimalNumberKind
	Int   int64
	Float float64
}

func ParseDecimalNumber(raw string) (DecimalNumber, bool) {
	if i, ok := parseFastDecimalInt(raw); ok {
		return DecimalNumber{Kind: DecimalNumberInt, Int: i}, true
	}
	s := strings.TrimSpace(raw)
	if s != raw {
		if i, ok := parseFastDecimalInt(s); ok {
			return DecimalNumber{Kind: DecimalNumberInt, Int: i}, true
		}
	}
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return DecimalNumber{Kind: DecimalNumberInt, Int: i}, true
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return DecimalNumber{Kind: DecimalNumberFloat, Float: f}, true
	}
	return DecimalNumber{}, false
}

func parseFastDecimalInt(s string) (int64, bool) {
	if s == "" {
		return 0, false
	}
	neg := false
	i := 0
	switch s[0] {
	case '-':
		neg = true
		i = 1
	case '+':
		i = 1
	}
	if i == len(s) {
		return 0, false
	}
	var n uint64
	const maxPos = uint64(^uint64(0) >> 1)
	limit := maxPos
	if neg {
		limit++
	}
	for ; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return 0, false
		}
		d := uint64(c - '0')
		if n > (limit-d)/10 {
			return 0, false
		}
		n = n*10 + d
	}
	if neg {
		if n == maxPos+1 {
			return -1 << 63, true
		}
		return -int64(n), true
	}
	return int64(n), true
}
