package runtime

import (
	"fmt"
	"strconv"
	"strings"
	"unsafe"
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
	s := args[0].Str()
	slen := len(s)

	i := int(toInt(args[1]))
	j := slen
	if len(args) >= 3 {
		j = int(toInt(args[2]))
	}

	// Convert Lua 1-based indexes to Go byte offsets.
	if i < 0 {
		i = slen + i + 1
	}
	if i < 1 {
		i = 1
	}
	if j < 0 {
		j = slen + j + 1
	}
	if j > slen {
		j = slen
	}
	if i > j {
		return StringValue(""), nil
	}
	return StringValue(s[i-1 : j]), nil
}

func stringSub2Value(sv, iv Value) (Value, error) {
	if !sv.IsString() {
		return NilValue(), fmt.Errorf("bad argument #1 to 'string.sub' (string expected)")
	}
	s := sv.Str()
	slen := len(s)
	i := int(toInt(iv))
	if i < 0 {
		i = slen + i + 1
	}
	if i < 1 {
		i = 1
	}
	if i > slen {
		return StringValue(""), nil
	}
	return StringValue(s[i-1:]), nil
}

func stringSub3Value(sv, iv, jv Value) (Value, error) {
	if !sv.IsString() {
		return NilValue(), fmt.Errorf("bad argument #1 to 'string.sub' (string expected)")
	}
	s := sv.Str()
	slen := len(s)
	i := int(toInt(iv))
	j := int(toInt(jv))
	if i < 0 {
		i = slen + i + 1
	}
	if i < 1 {
		i = 1
	}
	if j < 0 {
		j = slen + j + 1
	}
	if j > slen {
		j = slen
	}
	if i > j {
		return StringValue(""), nil
	}
	return StringValue(s[i-1 : j]), nil
}

func stringByte1Value(sv Value) (Value, error) {
	if !sv.IsString() {
		return NilValue(), fmt.Errorf("bad argument #1 to 'string.byte' (string expected)")
	}
	s := sv.Str()
	if len(s) == 0 {
		return NilValue(), nil
	}
	return IntValue(int64(s[0])), nil
}

func stringByte2Value(sv, iv Value) (Value, error) {
	if !sv.IsString() {
		return NilValue(), fmt.Errorf("bad argument #1 to 'string.byte' (string expected)")
	}
	s := sv.Str()
	i := int(toInt(iv))
	if i < 0 {
		i = len(s) + i + 1
	}
	if i < 1 || i > len(s) {
		return NilValue(), nil
	}
	return IntValue(int64(s[i-1])), nil
}

func StdStringSubIdentityPtr() unsafe.Pointer {
	return unsafe.Pointer(&stdStringSubIdentity)
}

func stringSubRaw(s string, start, end int64, hasEnd bool) string {
	slen := len(s)
	i := int(start)
	j := slen
	if hasEnd {
		j = int(end)
	}
	if i < 0 {
		i = slen + i + 1
	}
	if i < 1 {
		i = 1
	}
	if j < 0 {
		j = slen + j + 1
	}
	if j > slen {
		j = slen
	}
	if i > j {
		return ""
	}
	return s[i-1 : j]
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
