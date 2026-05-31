// Code generated during stdlib binding refresh; DO NOT EDIT.

package runtime

import "fmt"

func installBase64GeneratedBindings(t *Table, maxHostResult func() int64) {
	stdlibBindBase64Decode(t, maxHostResult)
	stdlibBindBase64Encode(t, maxHostResult)
	stdlibBindBase64URLDecode(t, maxHostResult)
	stdlibBindBase64URLEncode(t, maxHostResult)
}

func installHashGeneratedBindings(t *Table) {
	stdlibBindHashCRC32(t)
	stdlibBindHashHMACSHA256(t)
	stdlibBindHashMD5(t)
	stdlibBindHashSHA1(t)
	stdlibBindHashSHA256(t)
	stdlibBindHashSHA512(t)
}

func stdlibBindBase64Decode(t *Table, maxHostResult func() int64) {
	fast := func(a Value) (Value, Value, int, error) { return base64DecodeValue(maxHostResult, a) }
	t.RawSetString("decode", FunctionValue(&GoFunction{
		Name: "base64.decode",
		Fn: func(args []Value) ([]Value, error) {
			if len(args) < 1 {
				return nil, fmt.Errorf("bad argument #1 to '%s'", "base64.decode")
			}
			return stdlibBindRet2Values(fast(args[0]))
		},
		FastArg1Ret2: fast,
	}))
}

func stdlibBindBase64Encode(t *Table, maxHostResult func() int64) {
	fast := func(a Value) (Value, error) { return base64EncodeValue(maxHostResult, a) }
	t.RawSetString("encode", FunctionValue(&GoFunction{
		Name: "base64.encode",
		Fn: func(args []Value) ([]Value, error) {
			if len(args) < 1 {
				return nil, fmt.Errorf("bad argument #1 to '%s'", "base64.encode")
			}
			v, err := fast(args[0])
			if err != nil {
				return nil, err
			}
			return []Value{v}, nil
		},
		FastArg1: fast,
	}))
}

func stdlibBindBase64URLDecode(t *Table, maxHostResult func() int64) {
	fast := func(a Value) (Value, Value, int, error) { return base64URLDecodeValue(maxHostResult, a) }
	t.RawSetString("urlDecode", FunctionValue(&GoFunction{
		Name: "base64.urlDecode",
		Fn: func(args []Value) ([]Value, error) {
			if len(args) < 1 {
				return nil, fmt.Errorf("bad argument #1 to '%s'", "base64.urlDecode")
			}
			return stdlibBindRet2Values(fast(args[0]))
		},
		FastArg1Ret2: fast,
	}))
}

func stdlibBindBase64URLEncode(t *Table, maxHostResult func() int64) {
	fast := func(a Value) (Value, error) { return base64URLEncodeValue(maxHostResult, a) }
	t.RawSetString("urlEncode", FunctionValue(&GoFunction{
		Name: "base64.urlEncode",
		Fn: func(args []Value) ([]Value, error) {
			if len(args) < 1 {
				return nil, fmt.Errorf("bad argument #1 to '%s'", "base64.urlEncode")
			}
			v, err := fast(args[0])
			if err != nil {
				return nil, err
			}
			return []Value{v}, nil
		},
		FastArg1: fast,
	}))
}

func stdlibBindHashCRC32(t *Table) {
	fast := hashCRC32Value
	t.RawSetString("crc32", FunctionValue(&GoFunction{
		Name: "hash.crc32",
		Fn: func(args []Value) ([]Value, error) {
			if len(args) < 1 {
				return nil, fmt.Errorf("bad argument #1 to '%s'", "hash.crc32")
			}
			v, err := fast(args[0])
			if err != nil {
				return nil, err
			}
			return []Value{v}, nil
		},
		FastArg1: fast,
	}))
}

func stdlibBindHashHMACSHA256(t *Table) {
	t.RawSetString("hmacSHA256", FunctionValue(&GoFunction{
		Name: "hash.hmacSHA256",
		Fn: func(args []Value) ([]Value, error) {
			if len(args) < 2 {
				return nil, fmt.Errorf("bad arguments to 'hash.hmacSHA256' (key and message expected)")
			}
			v, err := hashHMACSHA256Value(args[0], args[1])
			if err != nil {
				return nil, err
			}
			return []Value{v}, nil
		},
	}))
}

func stdlibBindHashMD5(t *Table) {
	fast := hashMD5Value
	t.RawSetString("md5", FunctionValue(&GoFunction{
		Name: "hash.md5",
		Fn: func(args []Value) ([]Value, error) {
			if len(args) < 1 {
				return nil, fmt.Errorf("bad argument #1 to '%s'", "hash.md5")
			}
			v, err := fast(args[0])
			if err != nil {
				return nil, err
			}
			return []Value{v}, nil
		},
		FastArg1: fast,
	}))
}

func stdlibBindHashSHA1(t *Table) {
	fast := hashSHA1Value
	t.RawSetString("sha1", FunctionValue(&GoFunction{
		Name: "hash.sha1",
		Fn: func(args []Value) ([]Value, error) {
			if len(args) < 1 {
				return nil, fmt.Errorf("bad argument #1 to '%s'", "hash.sha1")
			}
			v, err := fast(args[0])
			if err != nil {
				return nil, err
			}
			return []Value{v}, nil
		},
		FastArg1: fast,
	}))
}

func stdlibBindHashSHA256(t *Table) {
	fast := hashSHA256Value
	t.RawSetString("sha256", FunctionValue(&GoFunction{
		Name: "hash.sha256",
		Fn: func(args []Value) ([]Value, error) {
			if len(args) < 1 {
				return nil, fmt.Errorf("bad argument #1 to '%s'", "hash.sha256")
			}
			v, err := fast(args[0])
			if err != nil {
				return nil, err
			}
			return []Value{v}, nil
		},
		FastArg1: fast,
	}))
}

func stdlibBindHashSHA512(t *Table) {
	fast := hashSHA512Value
	t.RawSetString("sha512", FunctionValue(&GoFunction{
		Name: "hash.sha512",
		Fn: func(args []Value) ([]Value, error) {
			if len(args) < 1 {
				return nil, fmt.Errorf("bad argument #1 to '%s'", "hash.sha512")
			}
			v, err := fast(args[0])
			if err != nil {
				return nil, err
			}
			return []Value{v}, nil
		},
		FastArg1: fast,
	}))
}

func stdlibBindRet2Values(r0, r1 Value, n int, err error) ([]Value, error) {
	if err != nil {
		return nil, err
	}
	if n <= 1 {
		return []Value{r0}, nil
	}
	return []Value{r0, r1}, nil
}
