package data

import (
	"fmt"
	"strings"
	"unicode"
)

// StringScalar normalizes Leia/q string-like scalars for reusable string
// helpers. It intentionally accepts symbols as categorical strings.
func StringScalar(name string, value any) (string, error) {
	switch x := value.(type) {
	case string:
		return x, nil
	case Symbol:
		return string(x), nil
	default:
		return "", fmt.Errorf("%s expects string or symbol values", name)
	}
}

func TransformStringValue(name string, value any, fn func(string) string) (any, error) {
	if array, ok := value.(Array); ok {
		out := make([]string, array.Len())
		for i := 0; i < array.Len(); i++ {
			item, ok := array.At(i)
			if !ok {
				return nil, fmt.Errorf("%s row %d out of range", name, i)
			}
			if IsNull(item) {
				out[i] = ""
				continue
			}
			text, err := StringScalar(name, item)
			if err != nil {
				return nil, err
			}
			out[i] = fn(text)
		}
		return NewString(out), nil
	}
	if IsNull(value) {
		return "", nil
	}
	text, err := StringScalar(name, value)
	if err != nil {
		return nil, err
	}
	return fn(text), nil
}

func TrimStringValue(value any) (any, error) {
	return TransformStringValue("trim", value, strings.TrimSpace)
}

func LTrimStringValue(value any) (any, error) {
	return TransformStringValue("ltrim", value, func(s string) string {
		return strings.TrimLeftFunc(s, unicode.IsSpace)
	})
}

func RTrimStringValue(value any) (any, error) {
	return TransformStringValue("rtrim", value, func(s string) string {
		return strings.TrimRightFunc(s, unicode.IsSpace)
	})
}

func StringSearch(haystack, needle string) Array {
	if needle == "" {
		return NewI64(nil)
	}
	var indexes []int64
	for offset := 0; offset <= len(haystack); {
		i := strings.Index(haystack[offset:], needle)
		if i < 0 {
			break
		}
		indexes = append(indexes, int64(len([]rune(haystack[:offset+i]))))
		offset += i + len(needle)
	}
	return NewI64(indexes)
}

func StringReplaceAll(source, old, repl string) string {
	return strings.ReplaceAll(source, old, repl)
}

func StringJoin(sep string, values []any) (string, error) {
	parts := make([]string, len(values))
	for i, value := range values {
		text, err := StringScalar("sv", value)
		if err != nil {
			return "", err
		}
		parts[i] = text
	}
	return strings.Join(parts, sep), nil
}

func StringSplit(sep, text string) Array {
	if sep == "" {
		parts := make([]string, 0, len([]rune(text)))
		for _, r := range text {
			parts = append(parts, string(r))
		}
		return NewString(parts)
	}
	return NewString(strings.Split(text, sep))
}
