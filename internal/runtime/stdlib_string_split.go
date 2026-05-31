package runtime

import (
	"fmt"
	"unsafe"

	basestring "github.com/never-labs/gscript/internal/stdlib/base/string"
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
	capacity := 8
	if sep == "" {
		capacity = len(s)
	}
	tbl := NewAppendArrayTable(capacity)
	basestring.SplitEach(s, sep, func(part string) {
		arenaAppendValue(DefaultHeap, &tbl.array, StringValue(part))
	})
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
	return basestring.SplitProject(s, sep, index)
}

func stringSplitProjectStrings(s, sep string, index int64) Value {
	token, ok := stringSplitProjectSlice(s, sep, index)
	if !ok {
		return NilValue()
	}
	return StringValue(token)
}
