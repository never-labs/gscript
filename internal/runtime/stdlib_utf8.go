package runtime

import (
	"fmt"
	utf8x "github.com/never-labs/gscript/internal/stdlib/utf8x"
	"unicode"
	"unicode/utf8"
)

// buildUTF8Lib creates the "utf8" standard library table.
func buildUTF8Lib(interps ...*Interpreter) *Table {
	t := NewTable()
	maxHostResult := func() int64 {
		if len(interps) == 0 || interps[0] == nil {
			return 0
		}
		return interps[0].maxHostResult
	}

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
		buf := newHostResultBuffer(maxHostResult())
		for _, arg := range args {
			n := toInt(arg)
			if n < 0 || n > utf8.MaxRune || !utf8.ValidRune(rune(n)) {
				return nil, fmt.Errorf("bad argument to 'utf8.char' (value out of range)")
			}
			cp := rune(n)
			if _, err := buf.Write([]byte(string(cp))); err != nil {
				return nil, err
			}
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
		if n != 0 && start <= int64(len(s)) && utf8x.IsContinuationByte(s[start-1]) {
			return nil, fmt.Errorf("initial position is a continuation byte")
		}
		if n == 0 {
			pos, ok := utf8x.Offset(s, n, start)
			if !ok {
				return []Value{NilValue()}, nil
			}
			return []Value{IntValue(pos)}, nil
		}

		pos, ok := utf8x.Offset(s, n, start)
		if !ok {
			return []Value{NilValue()}, nil
		}
		return []Value{IntValue(pos)}, nil
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
		out := utf8x.Sanitize(args[0].Str(), replacement)
		if err := CheckProjectedHostStringBytes(maxHostResult(), len(out)); err != nil {
			return nil, err
		}
		return []Value{StringValue(out)}, nil
	})

	// utf8.reverse(s) → string (reverse by codepoint)
	set("reverse", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'utf8.reverse'")
		}
		out := utf8x.Reverse(args[0].Str())
		if err := CheckProjectedHostStringBytes(maxHostResult(), len(out)); err != nil {
			return nil, err
		}
		return []Value{StringValue(out)}, nil
	})

	// utf8.sub(s, i [, j]) → string (substring by codepoint indices, 1-based)
	set("sub", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'utf8.sub'")
		}
		i := int(toInt(args[1]))
		j := len([]rune(args[0].Str()))
		if len(args) >= 3 {
			j = int(toInt(args[2]))
		}

		out := utf8x.Sub(args[0].Str(), i, j)
		if err := CheckProjectedHostStringBytes(maxHostResult(), len(out)); err != nil {
			return nil, err
		}
		return []Value{StringValue(out)}, nil
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
		out := string(runes)
		if err := CheckProjectedHostStringBytes(maxHostResult(), len(out)); err != nil {
			return nil, err
		}
		return []Value{StringValue(out)}, nil
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
		out := string(runes)
		if err := CheckProjectedHostStringBytes(maxHostResult(), len(out)); err != nil {
			return nil, err
		}
		return []Value{StringValue(out)}, nil
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
	s := sv.Str()
	i := toInt(iv)
	if i < 0 {
		i = int64(len(s)) + i + 1
	}
	if i < 1 || i > int64(len(s)) {
		return NilValue(), fmt.Errorf("out of bounds")
	}
	r, size := utf8.DecodeRuneInString(s[int(i)-1:])
	if r == utf8.RuneError && size == 1 {
		return NilValue(), fmt.Errorf("invalid UTF-8 code")
	}
	return IntValue(int64(r)), nil
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

func utf8ValidationReport(s string) *Table {
	report := utf8x.Validate(s)
	tbl := NewTable()
	tbl.RawSetString("byteCount", IntValue(int64(report.ByteCount)))
	tbl.RawSetString("valid", BoolValue(report.Valid))
	tbl.RawSetString("runeCount", IntValue(int64(report.RuneCount)))
	if report.Valid {
		tbl.RawSetString("pos", NilValue())
		tbl.RawSetString("error", NilValue())
		return tbl
	}
	tbl.RawSetString("pos", IntValue(int64(report.ErrorPos)))
	tbl.RawSetString("error", StringValue(report.Error))
	return tbl
}
