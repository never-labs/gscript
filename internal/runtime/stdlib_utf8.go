package runtime

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// buildUTF8Lib creates the "utf8" standard library table.
func buildUTF8Lib() *Table {
	t := NewTable()

	set := func(name string, fn func([]Value) ([]Value, error)) {
		t.RawSet(StringValue(name), FunctionValue(&GoFunction{
			Name: "utf8." + name,
			Fn:   fn,
		}))
	}

	// Constants
	t.RawSet(StringValue("charpattern"), StringValue("[\x00-\x7F\xC2-\xFD][\x80-\xBF]*"))

	// utf8.len(s) → int, or nil, errMsg if invalid
	set("len", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'utf8.len'")
		}
		s := args[0].Str()
		i := int64(1)
		j := int64(len(s))
		if len(args) >= 2 {
			i = toInt(args[1])
		}
		if len(args) >= 3 {
			j = toInt(args[2])
		}
		if i < 0 {
			i = int64(len(s)) + i + 1
		}
		if j < 0 {
			j = int64(len(s)) + j + 1
		}
		if i < 1 || i > int64(len(s)+1) || j < 0 || j > int64(len(s)) {
			return nil, fmt.Errorf("position out of bounds")
		}
		if j < i {
			return []Value{IntValue(0)}, nil
		}
		count := int64(0)
		for pos := int(i) - 1; pos <= int(j)-1; {
			r, size := utf8.DecodeRuneInString(s[pos:])
			if r == utf8.RuneError && size == 1 {
				return []Value{NilValue(), IntValue(int64(pos + 1))}, nil
			}
			pos += size
			count++
		}
		return []Value{IntValue(count)}, nil
	})

	// utf8.char(...) → string
	set("char", func(args []Value) ([]Value, error) {
		var buf strings.Builder
		for _, arg := range args {
			n := toInt(arg)
			if n < 0 || n > utf8.MaxRune || !utf8.ValidRune(rune(n)) {
				return nil, fmt.Errorf("bad argument to 'utf8.char' (value out of range)")
			}
			cp := rune(n)
			buf.WriteRune(cp)
		}
		return []Value{StringValue(buf.String())}, nil
	})

	// utf8.codepoint(s, i [, j]) -> codepoints... (1-based byte indices)
	set("codepoint", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'utf8.codepoint'")
		}
		return utf8CodepointValues(args)
	})
	if gf := t.RawGetString("codepoint").GoFunction(); gf != nil {
		gf.FastArg1 = func(s Value) (Value, error) {
			return utf8SingleCodepointValue(s, IntValue(1))
		}
		gf.FastArg2 = utf8SingleCodepointValue
	}

	// utf8.codes(s) -> iterator returning byte position and codepoint.
	set("codes", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'utf8.codes'")
		}
		return []Value{utf8CodesIteratorValue(args[0])}, nil
	})
	if gf := t.RawGetString("codes").GoFunction(); gf != nil {
		gf.FastArg1 = func(s Value) (Value, error) {
			return utf8CodesIteratorValue(s), nil
		}
		gf.Fast1 = func(args []Value) (Value, error) {
			if len(args) < 1 {
				return NilValue(), fmt.Errorf("bad argument #1 to 'utf8.codes'")
			}
			return utf8CodesIteratorValue(args[0]), nil
		}
	}

	// utf8.offset(s, n [, i]) → int (byte position of nth codepoint, 1-based)
	set("offset", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'utf8.offset'")
		}
		s := args[0].Str()
		n := toInt(args[1])
		start := int64(1)
		if len(args) >= 3 {
			start = toInt(args[2])
		} else if n < 0 {
			start = int64(len(s) + 1)
		}
		if start < 0 {
			start = int64(len(s)) + start + 1
		}
		if start < 1 || start > int64(len(s)+1) {
			return nil, fmt.Errorf("position out of bounds")
		}
		if n != 0 && start <= int64(len(s)) && isUTF8ContinuationByte(s[start-1]) {
			return nil, fmt.Errorf("initial position is a continuation byte")
		}
		starts := utf8CodepointStarts(s)
		if n == 0 {
			pos := int(start) - 1
			if pos < 0 || pos >= len(s) {
				return []Value{NilValue()}, nil
			}
			for i := len(starts) - 1; i >= 0; i-- {
				if starts[i] <= pos {
					return []Value{IntValue(int64(starts[i] + 1))}, nil
				}
			}
			return []Value{NilValue()}, nil
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
			return []Value{IntValue(int64(starts[target] + 1))}, nil
		}
		if target == len(starts) {
			return []Value{IntValue(int64(len(s) + 1))}, nil
		}
		return []Value{NilValue()}, nil
	})

	// utf8.valid(s) → bool
	set("valid", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'utf8.valid'")
		}
		return []Value{BoolValue(utf8.ValidString(args[0].Str()))}, nil
	})

	// utf8.validate(s) -> {valid, byteCount, runeCount, error?, pos?}
	set("validate", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'utf8.validate'")
		}
		report := utf8ValidationReport(args[0].Str())
		return []Value{TableValue(report)}, nil
	})

	// utf8.sanitize(s [, replacement]) -> string
	set("sanitize", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'utf8.sanitize'")
		}
		replacement := "\uFFFD"
		if len(args) >= 2 {
			replacement = args[1].Str()
		}
		return []Value{StringValue(utf8Sanitize(args[0].Str(), replacement))}, nil
	})

	// utf8.reverse(s) → string (reverse by codepoint)
	set("reverse", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'utf8.reverse'")
		}
		runes := []rune(args[0].Str())
		for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
			runes[i], runes[j] = runes[j], runes[i]
		}
		return []Value{StringValue(string(runes))}, nil
	})

	// utf8.sub(s, i [, j]) → string (substring by codepoint indices, 1-based)
	set("sub", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'utf8.sub'")
		}
		runes := []rune(args[0].Str())
		i := int(toInt(args[1]))
		j := len(runes)
		if len(args) >= 3 {
			j = int(toInt(args[2]))
		}

		// Convert to 0-based
		if i < 0 {
			i = len(runes) + i + 1
		}
		if j < 0 {
			j = len(runes) + j + 1
		}

		// Clamp
		if i < 1 {
			i = 1
		}
		if j > len(runes) {
			j = len(runes)
		}
		if i > j+1 {
			return []Value{StringValue("")}, nil
		}

		return []Value{StringValue(string(runes[i-1 : j]))}, nil
	})

	// utf8.upper(s) → string
	set("upper", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'utf8.upper'")
		}
		runes := []rune(args[0].Str())
		for i, r := range runes {
			runes[i] = unicode.ToUpper(r)
		}
		return []Value{StringValue(string(runes))}, nil
	})

	// utf8.lower(s) → string
	set("lower", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'utf8.lower'")
		}
		runes := []rune(args[0].Str())
		for i, r := range runes {
			runes[i] = unicode.ToLower(r)
		}
		return []Value{StringValue(string(runes))}, nil
	})

	// utf8.charclass(cp) → string ("L", "N", "P", "S", "O")
	set("charclass", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'utf8.charclass'")
		}
		cp := rune(toInt(args[0]))
		var class string
		switch {
		case unicode.IsLetter(cp):
			class = "L"
		case unicode.IsDigit(cp):
			class = "N"
		case unicode.IsSpace(cp):
			class = "S"
		case unicode.IsPunct(cp) || unicode.IsSymbol(cp):
			class = "P"
		default:
			class = "O"
		}
		return []Value{StringValue(class)}, nil
	})

	return t
}

func utf8CodepointValues(args []Value) ([]Value, error) {
	s := args[0].Str()
	i := int64(1)
	if len(args) >= 2 {
		i = toInt(args[1])
	}
	j := i
	if len(args) >= 3 {
		j = toInt(args[2])
	}
	if i < 0 {
		i = int64(len(s)) + i + 1
	}
	if j < 0 {
		j = int64(len(s)) + j + 1
	}
	if i < 1 || i > int64(len(s)+1) || j > int64(len(s)) {
		return nil, fmt.Errorf("out of bounds")
	}
	if j < i {
		return []Value{}, nil
	}

	var results []Value
	for pos := int(i) - 1; pos < len(s) && pos <= int(j)-1; {
		r, size := utf8.DecodeRuneInString(s[pos:])
		if r == utf8.RuneError && size == 1 {
			return nil, fmt.Errorf("invalid UTF-8 code")
		}
		results = append(results, IntValue(int64(r)))
		pos += size
	}
	return results, nil
}

func utf8SingleCodepointValue(sv, iv Value) (Value, error) {
	results, err := utf8CodepointValues([]Value{sv, iv})
	if err != nil {
		return NilValue(), err
	}
	if len(results) == 0 {
		return NilValue(), nil
	}
	return results[0], nil
}

func utf8CodesIteratorValue(sv Value) Value {
	s := sv.Str()
	bytePos := -1
	iter := &GoFunction{
		Name: "utf8.codes_iterator",
	}
	next := func() (Value, Value, int, error) {
		bytePos++
		if bytePos >= len(s) {
			return NilValue(), NilValue(), 0, nil
		}
		r, size := utf8.DecodeRuneInString(s[bytePos:])
		if r == utf8.RuneError && size == 1 {
			return NilValue(), NilValue(), 0, fmt.Errorf("invalid UTF-8 code")
		}
		pos := bytePos + 1
		bytePos += size - 1
		return IntValue(int64(pos)), IntValue(int64(r)), 2, nil
	}
	iter.FastArg2Ret2 = func(_, _ Value) (Value, Value, int, error) {
		return next()
	}
	iter.Fn = func(_ []Value) ([]Value, error) {
		pos, cp, n, err := next()
		if err != nil {
			return nil, err
		}
		if n == 0 {
			return []Value{NilValue()}, nil
		}
		return []Value{pos, cp}, nil
	}
	return FunctionValue(iter)
}

func utf8CodepointStarts(s string) []int {
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

func isUTF8ContinuationByte(b byte) bool {
	return b >= 0x80 && b <= 0xBF
}

func utf8ValidationReport(s string) *Table {
	tbl := NewTable()
	tbl.RawSetString("byteCount", IntValue(int64(len(s))))
	runeCount := int64(0)
	for pos := 0; pos < len(s); {
		r, size := utf8.DecodeRuneInString(s[pos:])
		if r == utf8.RuneError && size == 1 {
			tbl.RawSetString("valid", BoolValue(false))
			tbl.RawSetString("pos", IntValue(int64(pos+1)))
			tbl.RawSetString("runeCount", IntValue(runeCount))
			tbl.RawSetString("error", StringValue("invalid UTF-8 encoding"))
			return tbl
		}
		pos += size
		runeCount++
	}
	tbl.RawSetString("valid", BoolValue(true))
	tbl.RawSetString("pos", NilValue())
	tbl.RawSetString("runeCount", IntValue(runeCount))
	tbl.RawSetString("error", NilValue())
	return tbl
}

func utf8Sanitize(s, replacement string) string {
	var out strings.Builder
	for pos := 0; pos < len(s); {
		r, size := utf8.DecodeRuneInString(s[pos:])
		if r == utf8.RuneError && size == 1 {
			out.WriteString(replacement)
			pos++
			continue
		}
		out.WriteString(s[pos : pos+size])
		pos += size
	}
	return out.String()
}
