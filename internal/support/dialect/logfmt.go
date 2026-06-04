package dialect

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
)

type LogfmtPair struct {
	Key   string
	Value string
}

func ParseLogfmt(src string) ([]LogfmtPair, error) {
	var out []LogfmtPair
	i := 0
	for {
		i = skipLogfmtSpace(src, i)
		if i >= len(src) {
			return out, nil
		}
		keyStart := i
		for i < len(src) && !unicode.IsSpace(rune(src[i])) && src[i] != '=' {
			i++
		}
		key := src[keyStart:i]
		if key == "" {
			return nil, fmt.Errorf("logfmt: empty key at byte %d", keyStart)
		}
		value := "true"
		if i < len(src) && src[i] == '=' {
			i++
			parsed, next, err := parseLogfmtValue(src, i)
			if err != nil {
				return nil, err
			}
			value = parsed
			i = next
		}
		out = append(out, LogfmtPair{Key: key, Value: value})
	}
}

func EncodeLogfmt(values map[string]string) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+quoteLogfmtValue(values[key]))
	}
	return strings.Join(parts, " ")
}

func parseLogfmtValue(src string, i int) (string, int, error) {
	if i >= len(src) {
		return "", i, nil
	}
	if src[i] != '"' {
		start := i
		for i < len(src) && !unicode.IsSpace(rune(src[i])) {
			i++
		}
		return src[start:i], i, nil
	}
	i++
	var b strings.Builder
	for i < len(src) {
		ch := src[i]
		i++
		switch ch {
		case '"':
			return b.String(), i, nil
		case '\\':
			if i >= len(src) {
				return "", i, fmt.Errorf("logfmt: unfinished escape")
			}
			esc := src[i]
			i++
			switch esc {
			case '\\', '"':
				b.WriteByte(esc)
			case 'n':
				b.WriteByte('\n')
			case 'r':
				b.WriteByte('\r')
			case 't':
				b.WriteByte('\t')
			default:
				b.WriteByte(esc)
			}
		default:
			b.WriteByte(ch)
		}
	}
	return "", i, fmt.Errorf("logfmt: unterminated quoted value")
}

func quoteLogfmtValue(value string) string {
	if value != "" && !needsLogfmtQuote(value) {
		return value
	}
	var b strings.Builder
	b.WriteByte('"')
	for i := 0; i < len(value); i++ {
		switch value[i] {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			b.WriteByte(value[i])
		}
	}
	b.WriteByte('"')
	return b.String()
}

func needsLogfmtQuote(value string) bool {
	for _, r := range value {
		if unicode.IsSpace(r) || r == '"' || r == '=' {
			return true
		}
	}
	return false
}

func skipLogfmtSpace(src string, i int) int {
	for i < len(src) && unicode.IsSpace(rune(src[i])) {
		i++
	}
	return i
}
