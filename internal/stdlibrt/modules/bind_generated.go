// Code generated during stdlib binding refresh; DO NOT EDIT.

package modules

import (
	"fmt"

	"github.com/never-labs/gscript/internal/runtime"
)

func installBase64GeneratedBindings(t *runtime.Table, maxHostResult func() int64) {
	stdlibBindBase64Decode(t, maxHostResult)
	stdlibBindBase64Encode(t, maxHostResult)
	stdlibBindBase64URLDecode(t, maxHostResult)
	stdlibBindBase64URLEncode(t, maxHostResult)
}

func installHashGeneratedBindings(t *runtime.Table) {
	stdlibBindHashCRC32(t)
	stdlibBindHashHMACSHA256(t)
	stdlibBindHashMD5(t)
	stdlibBindHashSHA1(t)
	stdlibBindHashSHA256(t)
	stdlibBindHashSHA512(t)
}

func stdlibBindBase64Decode(t *runtime.Table, maxHostResult func() int64) {
	fast := func(a runtime.Value) (runtime.Value, runtime.Value, int, error) {
		return base64DecodeValue(maxHostResult, a)
	}
	t.RawSetString("decode", runtime.FunctionValue(&runtime.GoFunction{
		Name: "base64.decode",
		Fn: func(args []runtime.Value) ([]runtime.Value, error) {
			if len(args) < 1 {
				return nil, fmt.Errorf("bad argument #1 to '%s'", "base64.decode")
			}
			return stdlibBindRet2Values(fast(args[0]))
		},
		FastArg1Ret2: fast,
	}))
}

func stdlibBindBase64Encode(t *runtime.Table, maxHostResult func() int64) {
	fast := func(a runtime.Value) (runtime.Value, error) { return base64EncodeValue(maxHostResult, a) }
	t.RawSetString("encode", runtime.FunctionValue(&runtime.GoFunction{
		Name: "base64.encode",
		Fn: func(args []runtime.Value) ([]runtime.Value, error) {
			if len(args) < 1 {
				return nil, fmt.Errorf("bad argument #1 to '%s'", "base64.encode")
			}
			v, err := fast(args[0])
			if err != nil {
				return nil, err
			}
			return []runtime.Value{v}, nil
		},
		FastArg1: fast,
	}))
}

func stdlibBindBase64URLDecode(t *runtime.Table, maxHostResult func() int64) {
	fast := func(a runtime.Value) (runtime.Value, runtime.Value, int, error) {
		return base64URLDecodeValue(maxHostResult, a)
	}
	t.RawSetString("urlDecode", runtime.FunctionValue(&runtime.GoFunction{
		Name: "base64.urlDecode",
		Fn: func(args []runtime.Value) ([]runtime.Value, error) {
			if len(args) < 1 {
				return nil, fmt.Errorf("bad argument #1 to '%s'", "base64.urlDecode")
			}
			return stdlibBindRet2Values(fast(args[0]))
		},
		FastArg1Ret2: fast,
	}))
}

func stdlibBindBase64URLEncode(t *runtime.Table, maxHostResult func() int64) {
	fast := func(a runtime.Value) (runtime.Value, error) { return base64URLEncodeValue(maxHostResult, a) }
	t.RawSetString("urlEncode", runtime.FunctionValue(&runtime.GoFunction{
		Name: "base64.urlEncode",
		Fn: func(args []runtime.Value) ([]runtime.Value, error) {
			if len(args) < 1 {
				return nil, fmt.Errorf("bad argument #1 to '%s'", "base64.urlEncode")
			}
			v, err := fast(args[0])
			if err != nil {
				return nil, err
			}
			return []runtime.Value{v}, nil
		},
		FastArg1: fast,
	}))
}

func stdlibBindHashCRC32(t *runtime.Table) {
	fast := hashCRC32Value
	t.RawSetString("crc32", runtime.FunctionValue(&runtime.GoFunction{
		Name: "hash.crc32",
		Fn: func(args []runtime.Value) ([]runtime.Value, error) {
			if len(args) < 1 {
				return nil, fmt.Errorf("bad argument #1 to '%s'", "hash.crc32")
			}
			v, err := fast(args[0])
			if err != nil {
				return nil, err
			}
			return []runtime.Value{v}, nil
		},
		FastArg1: fast,
	}))
}

func stdlibBindHashHMACSHA256(t *runtime.Table) {
	t.RawSetString("hmacSHA256", runtime.FunctionValue(&runtime.GoFunction{
		Name: "hash.hmacSHA256",
		Fn: func(args []runtime.Value) ([]runtime.Value, error) {
			if len(args) < 2 {
				return nil, fmt.Errorf("bad arguments to 'hash.hmacSHA256' (key and message expected)")
			}
			v, err := hashHMACSHA256Value(args[0], args[1])
			if err != nil {
				return nil, err
			}
			return []runtime.Value{v}, nil
		},
	}))
}

func stdlibBindHashMD5(t *runtime.Table) {
	fast := hashMD5Value
	t.RawSetString("md5", runtime.FunctionValue(&runtime.GoFunction{
		Name: "hash.md5",
		Fn: func(args []runtime.Value) ([]runtime.Value, error) {
			if len(args) < 1 {
				return nil, fmt.Errorf("bad argument #1 to '%s'", "hash.md5")
			}
			v, err := fast(args[0])
			if err != nil {
				return nil, err
			}
			return []runtime.Value{v}, nil
		},
		FastArg1: fast,
	}))
}

func stdlibBindHashSHA1(t *runtime.Table) {
	fast := hashSHA1Value
	t.RawSetString("sha1", runtime.FunctionValue(&runtime.GoFunction{
		Name: "hash.sha1",
		Fn: func(args []runtime.Value) ([]runtime.Value, error) {
			if len(args) < 1 {
				return nil, fmt.Errorf("bad argument #1 to '%s'", "hash.sha1")
			}
			v, err := fast(args[0])
			if err != nil {
				return nil, err
			}
			return []runtime.Value{v}, nil
		},
		FastArg1: fast,
	}))
}

func stdlibBindHashSHA256(t *runtime.Table) {
	fast := hashSHA256Value
	t.RawSetString("sha256", runtime.FunctionValue(&runtime.GoFunction{
		Name: "hash.sha256",
		Fn: func(args []runtime.Value) ([]runtime.Value, error) {
			if len(args) < 1 {
				return nil, fmt.Errorf("bad argument #1 to '%s'", "hash.sha256")
			}
			v, err := fast(args[0])
			if err != nil {
				return nil, err
			}
			return []runtime.Value{v}, nil
		},
		FastArg1: fast,
	}))
}

func stdlibBindHashSHA512(t *runtime.Table) {
	fast := hashSHA512Value
	t.RawSetString("sha512", runtime.FunctionValue(&runtime.GoFunction{
		Name: "hash.sha512",
		Fn: func(args []runtime.Value) ([]runtime.Value, error) {
			if len(args) < 1 {
				return nil, fmt.Errorf("bad argument #1 to '%s'", "hash.sha512")
			}
			v, err := fast(args[0])
			if err != nil {
				return nil, err
			}
			return []runtime.Value{v}, nil
		},
		FastArg1: fast,
	}))
}

func stdlibBindRet2Values(r0, r1 runtime.Value, n int, err error) ([]runtime.Value, error) {
	if err != nil {
		return nil, err
	}
	if n <= 1 {
		return []runtime.Value{r0}, nil
	}
	return []runtime.Value{r0, r1}, nil
}
