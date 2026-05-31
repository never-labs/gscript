package runtime

import (
	"fmt"
	"strconv"
	"strings"
	"unsafe"

	basestring "github.com/never-labs/gscript/internal/stdlib/base/string"
)

// stdlib_string_sub.go holds the string.sub / string.byte fast-path value
// helpers and the raw substring / string-to-number primitives they share.
//
// Pure code movement from stdlib_string.go; no behavior change.

func stringSubValue(args []Value) (Value, error) {
	if len(args) < 2 {
		return NilValue(), fmt.Errorf("bad argument to 'string.sub'")
	}
	if !args[0].IsString() {
		return NilValue(), fmt.Errorf("bad argument #1 to 'string.sub' (string expected)")
	}
	j := int64(0)
	hasEnd := false
	if len(args) >= 3 {
		j = toInt(args[2])
		hasEnd = true
	}
	return StringValue(basestring.LuaSub(args[0].Str(), toInt(args[1]), j, hasEnd)), nil
}

func stringSub2Value(sv, iv Value) (Value, error) {
	if !sv.IsString() {
		return NilValue(), fmt.Errorf("bad argument #1 to 'string.sub' (string expected)")
	}
	return StringValue(basestring.LuaSub(sv.Str(), toInt(iv), 0, false)), nil
}

func stringSub3Value(sv, iv, jv Value) (Value, error) {
	if !sv.IsString() {
		return NilValue(), fmt.Errorf("bad argument #1 to 'string.sub' (string expected)")
	}
	return StringValue(basestring.LuaSub(sv.Str(), toInt(iv), toInt(jv), true)), nil
}

func stringByte1Value(sv Value) (Value, error) {
	if !sv.IsString() {
		return NilValue(), fmt.Errorf("bad argument #1 to 'string.byte' (string expected)")
	}
	s := sv.Str()
	b, ok := basestring.LuaByteAt(s, 1)
	if !ok {
		return NilValue(), nil
	}
	return IntValue(int64(b)), nil
}

func stringByte2Value(sv, iv Value) (Value, error) {
	if !sv.IsString() {
		return NilValue(), fmt.Errorf("bad argument #1 to 'string.byte' (string expected)")
	}
	s := sv.Str()
	b, ok := basestring.LuaByteAt(s, toInt(iv))
	if !ok {
		return NilValue(), nil
	}
	return IntValue(int64(b)), nil
}

func StdStringSubIdentityPtr() unsafe.Pointer {
	return unsafe.Pointer(&stdStringSubIdentity)
}

func stringSubRaw(s string, start, end int64, hasEnd bool) string {
	return basestring.LuaSub(s, start, end, hasEnd)
}

func stringToNumberRaw(raw string) (Value, bool) {
	if v, ok := parseFastDecimalInt(raw); ok {
		return v, true
	}
	s := strings.TrimSpace(raw)
	if s != raw {
		if v, ok := parseFastDecimalInt(s); ok {
			return v, true
		}
	}
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return IntValue(i), true
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return FloatValue(f), true
	}
	return NilValue(), false
}
