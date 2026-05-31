package uuid

import (
	"crypto/rand"
	"fmt"
	"io"
	"regexp"
	"strings"
)

var pattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

type ParseResult struct {
	Version int64
	Variant string
	Bytes   string
}

func V4() (string, error) {
	return V4From(rand.Reader)
}

func V4Raw() (string, error) {
	b, err := v4Bytes(rand.Reader)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", b[:]), nil
}

func V4From(r io.Reader) (string, error) {
	b, err := v4Bytes(r)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

func IsValid(s string) bool {
	return pattern.MatchString(s)
}

func Parse(s string) (ParseResult, bool) {
	if !IsValid(s) {
		return ParseResult{}, false
	}

	version := hexNibble(s[14])
	variantBits := hexNibble(s[19])
	variant := "Future"
	if variantBits&0x8 == 0 {
		variant = "NCS"
	} else if variantBits&0x4 == 0 {
		variant = "RFC4122"
	} else if variantBits&0x2 == 0 {
		variant = "Microsoft"
	}

	return ParseResult{
		Version: version,
		Variant: variant,
		Bytes:   strings.ReplaceAll(s, "-", ""),
	}, true
}

func Nil() string {
	return "00000000-0000-0000-0000-000000000000"
}

func v4Bytes(r io.Reader) ([16]byte, error) {
	var b [16]byte
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return b, fmt.Errorf("failed to generate random bytes: %w", err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return b, nil
}

func hexNibble(c byte) int64 {
	switch {
	case c >= '0' && c <= '9':
		return int64(c - '0')
	case c >= 'a' && c <= 'f':
		return int64(c - 'a' + 10)
	case c >= 'A' && c <= 'F':
		return int64(c - 'A' + 10)
	default:
		return 0
	}
}
