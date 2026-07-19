package runtime

import (
	"fmt"
	"math"
	"unsafe"

	stdlibstring "github.com/never-labs/leia/internal/support/stringlib"
)

// native_string_sub.go holds runtime-owned native string.sub / string.byte
// fast-path value helpers and the raw substring / string-to-number primitives
// shared by VM/JIT paths.

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
		j = stringSubIndex(args[2])
		hasEnd = true
	}
	return StringValue(stdlibstring.LuaSub(args[0].Str(), stringSubIndex(args[1]), j, hasEnd)), nil
}

func stringSub2Value(sv, iv Value) (Value, error) {
	if !sv.IsString() {
		return NilValue(), fmt.Errorf("bad argument #1 to 'string.sub' (string expected)")
	}
	return StringValue(stdlibstring.LuaSub(sv.Str(), stringSubIndex(iv), 0, false)), nil
}

func stringSub3Value(sv, iv, jv Value) (Value, error) {
	if !sv.IsString() {
		return NilValue(), fmt.Errorf("bad argument #1 to 'string.sub' (string expected)")
	}
	return StringValue(stdlibstring.LuaSub(sv.Str(), stringSubIndex(iv), stringSubIndex(jv), true)), nil
}

func stringSubIndex(v Value) int64 {
	switch v.Type() {
	case TypeFloat:
		f := v.Float()
		if f >= float64(math.MaxInt64) {
			return math.MaxInt64
		}
		if f <= float64(math.MinInt64) {
			return math.MinInt64
		}
		return int64(f)
	case TypeString:
		n, ok := v.ToNumber()
		if ok {
			return stringSubIndex(n)
		}
		return 0
	default:
		return toInt(v)
	}
}

func stringByte1Value(sv Value) (Value, error) {
	if !sv.IsString() {
		return NilValue(), fmt.Errorf("bad argument #1 to 'string.byte' (string expected)")
	}
	s := sv.Str()
	b, ok := stdlibstring.LuaByteAt(s, 1)
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
	b, ok := stdlibstring.LuaByteAt(s, toInt(iv))
	if !ok {
		return NilValue(), nil
	}
	return IntValue(int64(b)), nil
}

func StdStringSubIdentityPtr() unsafe.Pointer {
	return unsafe.Pointer(&stdStringSubIdentity)
}

func stringSubRaw(s string, start, end int64, hasEnd bool) string {
	return stdlibstring.LuaSub(s, start, end, hasEnd)
}

func stringToNumberRaw(raw string) (Value, bool) {
	n, ok := stdlibstring.ParseDecimalNumber(raw)
	if !ok {
		return NilValue(), false
	}
	if n.Kind == stdlibstring.DecimalNumberInt {
		return IntValue(n.Int), true
	}
	return FloatValue(n.Float), true
}
