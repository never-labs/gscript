package bytes

import (
	stdbytes "bytes"
	"encoding/hex"
	"fmt"
	"strings"
)

func DecodeHex(s string) ([]byte, error) {
	return hex.DecodeString(s)
}

func ToHex(s string) string {
	return hex.EncodeToString([]byte(s))
}

func EncodedHexLen(n int) int {
	return hex.EncodedLen(n)
}

func XOR(s1, s2 string) (string, error) {
	b1 := []byte(s1)
	b2 := []byte(s2)
	if len(b1) != len(b2) {
		return "", fmt.Errorf("strings must have equal length (got %d and %d)", len(b1), len(b2))
	}
	out := make([]byte, len(b1))
	for i := range b1 {
		out[i] = b1[i] ^ b2[i]
	}
	return string(out), nil
}

func Compare(s1, s2 string) int {
	return stdbytes.Compare([]byte(s1), []byte(s2))
}

func Repeat(s string, n int) (string, error) {
	if n <= 0 {
		return "", nil
	}
	if _, err := RepeatLen(s, n); err != nil {
		return "", err
	}
	return strings.Repeat(s, n), nil
}

func RepeatLen(s string, n int) (int, error) {
	if n <= 0 {
		return 0, nil
	}
	if len(s) > 0 && n > int(^uint(0)>>1)/len(s) {
		return 0, fmt.Errorf("result too large")
	}
	return len(s) * n, nil
}
