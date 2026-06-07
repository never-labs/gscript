package runtime

import (
	"fmt"
	"unsafe"
)

// builtin_identity_ptrs.go holds the identity-token accessors for native
// builtins such as tonumber/select/pairs/ipairs/rawget/rawset/rawlen/type/next.
//
// The JIT uses these stable pointers to recognize runtime-owned functions
// without depending on Go function pointer equality.

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

func StdSoAAffineManyIdentityPtr() unsafe.Pointer {
	return unsafe.Pointer(&stdSoAAffineManyIdentity)
}

func StdQSQLIdentityPtr() unsafe.Pointer {
	return unsafe.Pointer(&stdQSQLIdentity)
}

func StdQSelectIdentityPtr() unsafe.Pointer {
	return unsafe.Pointer(&stdQSelectIdentity)
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

func IsStdSoAAffineManyFunction(v Value) bool {
	gf := v.GoFunction()
	return gf != nil &&
		gf.NativeKind == NativeKindStdSoAAffineMany &&
		gf.NativeData == StdSoAAffineManyIdentityPtr() &&
		gf.FastArg2 != nil
}

func IsStdQSQLFunction(v Value) bool {
	gf := v.GoFunction()
	return gf != nil &&
		gf.NativeKind == NativeKindStdQSQL &&
		gf.NativeData == StdQSQLIdentityPtr() &&
		gf.FastArg2 != nil
}

func IsStdQSelectFunction(v Value) bool {
	gf := v.GoFunction()
	return gf != nil &&
		gf.NativeKind == NativeKindStdQSelect &&
		gf.NativeData == StdQSelectIdentityPtr() &&
		gf.FastArg2 != nil
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
