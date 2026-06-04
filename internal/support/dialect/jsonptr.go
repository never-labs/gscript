package dialect

import (
	"fmt"
	"strconv"
	"strings"
)

func ParseJSONPointer(pointer string) ([]string, error) {
	if pointer == "" {
		return nil, nil
	}
	if !strings.HasPrefix(pointer, "/") {
		return nil, fmt.Errorf("jsonptr: pointer must be empty or start with /")
	}
	raw := strings.Split(pointer[1:], "/")
	out := make([]string, len(raw))
	for i, token := range raw {
		decoded, err := decodeJSONPointerToken(token)
		if err != nil {
			return nil, err
		}
		out[i] = decoded
	}
	return out, nil
}

func EncodeJSONPointer(tokens []string) string {
	if len(tokens) == 0 {
		return ""
	}
	encoded := make([]string, len(tokens))
	for i, token := range tokens {
		encoded[i] = strings.ReplaceAll(strings.ReplaceAll(token, "~", "~0"), "/", "~1")
	}
	return "/" + strings.Join(encoded, "/")
}

func JSONPointerIndex(token string) (int, bool) {
	if token == "" || token == "-" {
		return 0, false
	}
	if len(token) > 1 && token[0] == '0' {
		return 0, false
	}
	n, err := strconv.Atoi(token)
	if err != nil || n < 0 {
		return 0, false
	}
	return n, true
}

func decodeJSONPointerToken(token string) (string, error) {
	var b strings.Builder
	for i := 0; i < len(token); i++ {
		if token[i] != '~' {
			b.WriteByte(token[i])
			continue
		}
		if i+1 >= len(token) {
			return "", fmt.Errorf("jsonptr: invalid escape")
		}
		i++
		switch token[i] {
		case '0':
			b.WriteByte('~')
		case '1':
			b.WriteByte('/')
		default:
			return "", fmt.Errorf("jsonptr: invalid escape ~%c", token[i])
		}
	}
	return b.String(), nil
}
