package runtime

import (
	"fmt"
	"unsafe"
)

// stdlib_identity_ptrs.go holds the non-string stdlib identity-token accessors
// (tonumber/select/pairs/ipairs/rawget/rawset/rawlen/type/next/getmetatable)
// and the select range/result helpers. These are misplaced relative to the
// string library but moved here verbatim.
//
// Pure code movement from stdlib_string.go; no behavior change.

func StdToNumberIdentityPtr() unsafe.Pointer {
	return unsafe.Pointer(&stdToNumberIdentity)
}

func StdSelectIdentityPtr() unsafe.Pointer {
	return unsafe.Pointer(&stdSelectIdentity)
}

func StdPairsIdentityPtr() unsafe.Pointer {
	return unsafe.Pointer(&stdPairsIdentity)
}

func StdIPairsIdentityPtr() unsafe.Pointer {
	return unsafe.Pointer(&stdIPairsIdentity)
}

func StdRawGetIdentityPtr() unsafe.Pointer {
	return unsafe.Pointer(&stdRawGetIdentity)
}

func StdRawSetIdentityPtr() unsafe.Pointer {
	return unsafe.Pointer(&stdRawSetIdentity)
}

func StdRawLenIdentityPtr() unsafe.Pointer {
	return unsafe.Pointer(&stdRawLenIdentity)
}

func StdTypeIdentityPtr() unsafe.Pointer {
	return unsafe.Pointer(&stdTypeIdentity)
}

func StdNextIdentityPtr() unsafe.Pointer {
	return unsafe.Pointer(&stdNextIdentity)
}

func StdGetMetatableIdentityPtr() unsafe.Pointer {
	return unsafe.Pointer(&stdGetMetatableIdentity)
}

func IsStdToNumberFunction(v Value) bool {
	gf := v.GoFunction()
	return gf != nil &&
		gf.NativeKind == NativeKindStdToNumber &&
		gf.NativeData == StdToNumberIdentityPtr() &&
		gf.FastArg1 != nil
}

func IsStdSelectFunction(v Value) bool {
	gf := v.GoFunction()
	return gf != nil &&
		gf.NativeKind == NativeKindStdSelect &&
		gf.NativeData == StdSelectIdentityPtr()
}

func IsStdPairsFunction(v Value) bool {
	gf := v.GoFunction()
	return gf != nil &&
		gf.NativeKind == NativeKindStdPairs &&
		gf.NativeData == StdPairsIdentityPtr()
}

func IsStdIPairsFunction(v Value) bool {
	gf := v.GoFunction()
	return gf != nil &&
		gf.NativeKind == NativeKindStdIPairs &&
		gf.NativeData == StdIPairsIdentityPtr()
}

func SelectReturnRange(selector Value, argCount int) (start int, countOnly bool, err error) {
	if argCount == 0 {
		return 0, false, fmt.Errorf("bad argument #1 to 'select'")
	}
	if selector.IsString() && selector.Str() == "#" {
		return argCount - 1, true, nil
	}
	n, ok := selector.ToNumber()
	if !ok {
		return 0, false, fmt.Errorf("bad argument #1 to 'select' (number or string expected)")
	}
	idx := int(n.Number())
	if idx < 0 {
		idx = argCount + idx
	}
	if idx < 1 {
		return 0, false, fmt.Errorf("bad argument #1 to 'select' (index out of range)")
	}
	if idx >= argCount {
		return argCount, false, nil
	}
	return idx, false, nil
}

func SelectResults(args []Value) ([]Value, error) {
	start, countOnly, err := SelectReturnRange(args[0], len(args))
	if err != nil {
		return nil, err
	}
	if countOnly {
		return []Value{IntValue(int64(start))}, nil
	}
	if start >= len(args) {
		return nil, nil
	}
	return args[start:], nil
}
