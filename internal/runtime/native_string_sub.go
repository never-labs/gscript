package runtime

import (
	"fmt"
	"unsafe"

	stdlibstring "github.com/never-labs/gscript/internal/stringlib"
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
		j = toInt(args[2])
		hasEnd = true
	}
	return StringValue(stdlibstring.LuaSub(args[0].Str(), toInt(args[1]), j, hasEnd)), nil
}

func stringSub2Value(sv, iv Value) (Value, error) {
	if !sv.IsString() {
		return NilValue(), fmt.Errorf("bad argument #1 to 'string.sub' (string expected)")
	}
	return StringValue(stdlibstring.LuaSub(sv.Str(), toInt(iv), 0, false)), nil
}

func stringSub3Value(sv, iv, jv Value) (Value, error) {
	if !sv.IsString() {
		return NilValue(), fmt.Errorf("bad argument #1 to 'string.sub' (string expected)")
	}
	return StringValue(stdlibstring.LuaSub(sv.Str(), toInt(iv), toInt(jv), true)), nil
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
