package runtime

import (
	"fmt"
	"strings"
	"unsafe"
)

// stdlib_string_split.go holds string.split and its split-projection helpers
// (whole-table split plus single-token projection fast paths).
//
// Pure code movement from stdlib_string.go; no behavior change.

func stringSplitValue(args []Value) (Value, error) {
	if len(args) < 2 {
		return NilValue(), fmt.Errorf("bad argument to 'string.split'")
	}
	if !args[0].IsString() || !args[1].IsString() {
		return NilValue(), fmt.Errorf("bad argument to 'string.split' (string expected)")
	}
	return stringSplitStrings(args[0].Str(), args[1].Str()), nil
}

func stringSplit2Value(sv, sepv Value) (Value, error) {
	if !sv.IsString() || !sepv.IsString() {
		return NilValue(), fmt.Errorf("bad argument to 'string.split' (string expected)")
	}
	return stringSplitStrings(sv.Str(), sepv.Str()), nil
}

func stringSplitStrings(s, sep string) Value {
	if sep == "" {
		tbl := NewSequentialArrayTable(len(s))
		for i := 0; i < len(s); i++ {
			tbl.array[i+1] = StringValue(string(s[i]))
		}
		return TableValue(tbl)
	}

	tbl := NewAppendArrayTable(8)
	if len(sep) == 1 {
		sepByte := sep[0]
		start := 0
		for i := 0; i < len(s); i++ {
			if s[i] != sepByte {
				continue
			}
			arenaAppendValue(DefaultHeap, &tbl.array, StringValue(s[start:i]))
			start = i + 1
		}
		arenaAppendValue(DefaultHeap, &tbl.array, StringValue(s[start:]))
		return TableValue(tbl)
	}

	start := 0
	for {
		next := strings.Index(s[start:], sep)
		if next < 0 {
			arenaAppendValue(DefaultHeap, &tbl.array, StringValue(s[start:]))
			break
		}
		end := start + next
		arenaAppendValue(DefaultHeap, &tbl.array, StringValue(s[start:end]))
		start = end + len(sep)
	}
	return TableValue(tbl)
}

func StdStringSplitIdentityPtr() unsafe.Pointer {
	return unsafe.Pointer(&stdStringSplitIdentity)
}

func IsStdStringSplitFunction(v Value) bool {
	gf := v.GoFunction()
	return gf != nil &&
		gf.NativeKind == NativeKindStdStringSplit &&
		gf.NativeData == StdStringSplitIdentityPtr() &&
		gf.FastArg2 != nil
}

func StringSplitProject(sv, sepv Value, index int64) (Value, error) {
	if !sv.IsString() || !sepv.IsString() {
		return NilValue(), fmt.Errorf("bad argument to 'string.split' (string expected)")
	}
	if index < 1 {
		return NilValue(), nil
	}
	return stringSplitProjectStrings(sv.Str(), sepv.Str(), index), nil
}

func StringSplitProjectSub(sv, sepv Value, index, start, end int64, hasEnd bool) (Value, error) {
	if !sv.IsString() || !sepv.IsString() {
		return NilValue(), fmt.Errorf("bad argument to 'string.split' (string expected)")
	}
	if index < 1 {
		return NilValue(), fmt.Errorf("bad argument #1 to 'string.sub' (string expected)")
	}
	token, ok := stringSplitProjectSlice(sv.Str(), sepv.Str(), index)
	if !ok {
		return NilValue(), fmt.Errorf("bad argument #1 to 'string.sub' (string expected)")
	}
	return StringValue(stringSubRaw(token, start, end, hasEnd)), nil
}

func StringSplitProjectSubToNumber(sv, sepv Value, index, start, end int64, hasEnd bool) (Value, error) {
	if !sv.IsString() || !sepv.IsString() {
		return NilValue(), fmt.Errorf("bad argument to 'string.split' (string expected)")
	}
	if index < 1 {
		return NilValue(), fmt.Errorf("bad argument #1 to 'string.sub' (string expected)")
	}
	token, ok := stringSplitProjectSlice(sv.Str(), sepv.Str(), index)
	if !ok {
		return NilValue(), fmt.Errorf("bad argument #1 to 'string.sub' (string expected)")
	}
	if v, ok := stringToNumberRaw(stringSubRaw(token, start, end, hasEnd)); ok {
		return v, nil
	}
	return NilValue(), nil
}

func stringSplitProjectSlice(s, sep string, index int64) (string, bool) {
	if sep == "" {
		i := int(index) - 1
		if i < 0 || i >= len(s) {
			return "", false
		}
		return string(s[i]), true
	}

	token := int64(1)
	start := 0
	if len(sep) == 1 {
		sepByte := sep[0]
		for i := 0; i < len(s); i++ {
			if s[i] != sepByte {
				continue
			}
			if token == index {
				return s[start:i], true
			}
			token++
			start = i + 1
		}
		if token == index {
			return s[start:], true
		}
		return "", false
	}

	for {
		next := strings.Index(s[start:], sep)
		if next < 0 {
			if token == index {
				return s[start:], true
			}
			return "", false
		}
		end := start + next
		if token == index {
			return s[start:end], true
		}
		token++
		start = end + len(sep)
	}
}

func stringSplitProjectStrings(s, sep string, index int64) Value {
	if sep == "" {
		i := int(index) - 1
		if i < 0 || i >= len(s) {
			return NilValue()
		}
		return StringValue(string(s[i]))
	}

	token := int64(1)
	start := 0
	if len(sep) == 1 {
		sepByte := sep[0]
		for i := 0; i < len(s); i++ {
			if s[i] != sepByte {
				continue
			}
			if token == index {
				return StringValue(s[start:i])
			}
			token++
			start = i + 1
		}
		if token == index {
			return StringValue(s[start:])
		}
		return NilValue()
	}

	for {
		next := strings.Index(s[start:], sep)
		if next < 0 {
			if token == index {
				return StringValue(s[start:])
			}
			return NilValue()
		}
		end := start + next
		if token == index {
			return StringValue(s[start:end])
		}
		token++
		start = end + len(sep)
	}
}
