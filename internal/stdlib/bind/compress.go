package bind

import (
	"fmt"
	"io"

	basecompress "github.com/never-labs/leia/internal/stdlib/lib/compress"
)

// BuildCompress creates the "compress" standard library table.
// Provides gzip, zlib, and deflate compression/decompression.
// Inspired by Odin's compress package (gzip, zlib).
func BuildCompress(maxHostResult func() int64) *Table {
	t := NewTable()

	setFastArg2 := func(name string, fn func([]Value) ([]Value, error), fast func(Value, Value) (Value, error)) {
		t.RawSet(StringValue(name), FunctionValue(&GoFunction{
			Name:     "compress." + name,
			Fn:       fn,
			FastArg2: fast,
		}))
	}
	setFastArg1Ret2 := func(name string, fn func([]Value) ([]Value, error), fast func(Value) (Value, Value, int, error)) {
		t.RawSet(StringValue(name), FunctionValue(&GoFunction{
			Name:         "compress." + name,
			Fn:           fn,
			FastArg1Ret2: fast,
		}))
	}

	// ---------------------------------------------------------------
	// Gzip
	// ---------------------------------------------------------------

	// compress.gzipEncode(str [, level]) -> compressed string
	// level: 1-9 (1=fastest, 9=best compression), default=6
	gzipEncode := func(dataVal, levelVal Value) (Value, error) {
		if !dataVal.IsString() {
			return NilValue(), fmt.Errorf("bad argument #1 to 'compress.gzipEncode' (string expected)")
		}
		level := basecompress.GzipDefaultLevel()
		if levelVal.IsNumber() {
			level = basecompress.NormalizeLevel(int(toInt(levelVal)), basecompress.GzipDefaultLevel())
		}
		out, err := basecompress.GzipEncode(dataVal.Str(), level)
		if err != nil {
			return NilValue(), fmt.Errorf("compress.gzipEncode: %v", err)
		}
		return StringValue(out), nil
	}
	setFastArg2("gzipEncode", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'compress.gzipEncode' (string expected)")
		}
		level := NilValue()
		if len(args) >= 2 {
			level = args[1]
		}
		v, err := gzipEncode(args[0], level)
		return []Value{v}, err
	}, gzipEncode)

	// compress.gzipDecode(str) -> decompressed string, or nil, error
	gzipDecode := func(arg Value) (Value, Value, int, error) {
		if !arg.IsString() {
			return NilValue(), NilValue(), 0, fmt.Errorf("bad argument #1 to 'compress.gzipDecode' (string expected)")
		}
		decoded, err := basecompress.GzipDecode(arg.Str(), func(r io.Reader) ([]byte, error) {
			return ReadAllWithHostResultLimit(r, hostResultLimit(maxHostResult))
		})
		if err != nil {
			return NilValue(), StringValue(err.Error()), 2, nil
		}
		return StringValue(decoded), NilValue(), 1, nil
	}
	setFastArg1Ret2("gzipDecode", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'compress.gzipDecode' (string expected)")
		}
		r0, r1, n, err := gzipDecode(args[0])
		if err != nil {
			return nil, err
		}
		if n == 1 {
			return []Value{r0}, nil
		}
		return []Value{r0, r1}, nil
	}, gzipDecode)

	// ---------------------------------------------------------------
	// Zlib
	// ---------------------------------------------------------------

	// compress.zlibEncode(str [, level]) -> compressed string
	zlibEncode := func(dataVal, levelVal Value) (Value, error) {
		if !dataVal.IsString() {
			return NilValue(), fmt.Errorf("bad argument #1 to 'compress.zlibEncode' (string expected)")
		}
		level := basecompress.ZlibDefaultLevel()
		if levelVal.IsNumber() {
			level = basecompress.NormalizeLevel(int(toInt(levelVal)), basecompress.ZlibDefaultLevel())
		}
		out, err := basecompress.ZlibEncode(dataVal.Str(), level)
		if err != nil {
			return NilValue(), fmt.Errorf("compress.zlibEncode: %v", err)
		}
		return StringValue(out), nil
	}
	setFastArg2("zlibEncode", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'compress.zlibEncode' (string expected)")
		}
		level := NilValue()
		if len(args) >= 2 {
			level = args[1]
		}
		v, err := zlibEncode(args[0], level)
		return []Value{v}, err
	}, zlibEncode)

	// compress.zlibDecode(str) -> decompressed string, or nil, error
	zlibDecode := func(arg Value) (Value, Value, int, error) {
		if !arg.IsString() {
			return NilValue(), NilValue(), 0, fmt.Errorf("bad argument #1 to 'compress.zlibDecode' (string expected)")
		}
		decoded, err := basecompress.ZlibDecode(arg.Str(), func(r io.Reader) ([]byte, error) {
			return ReadAllWithHostResultLimit(r, hostResultLimit(maxHostResult))
		})
		if err != nil {
			return NilValue(), StringValue(err.Error()), 2, nil
		}
		return StringValue(decoded), NilValue(), 1, nil
	}
	setFastArg1Ret2("zlibDecode", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'compress.zlibDecode' (string expected)")
		}
		r0, r1, n, err := zlibDecode(args[0])
		if err != nil {
			return nil, err
		}
		if n == 1 {
			return []Value{r0}, nil
		}
		return []Value{r0, r1}, nil
	}, zlibDecode)

	// ---------------------------------------------------------------
	// Deflate (raw, no header)
	// ---------------------------------------------------------------

	// compress.deflateEncode(str [, level]) -> compressed string
	deflateEncode := func(dataVal, levelVal Value) (Value, error) {
		if !dataVal.IsString() {
			return NilValue(), fmt.Errorf("bad argument #1 to 'compress.deflateEncode' (string expected)")
		}
		level := basecompress.DeflateDefaultLevel()
		if levelVal.IsNumber() {
			level = basecompress.NormalizeLevel(int(toInt(levelVal)), basecompress.DeflateDefaultLevel())
		}
		out, err := basecompress.DeflateEncode(dataVal.Str(), level)
		if err != nil {
			return NilValue(), fmt.Errorf("compress.deflateEncode: %v", err)
		}
		return StringValue(out), nil
	}
	setFastArg2("deflateEncode", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'compress.deflateEncode' (string expected)")
		}
		level := NilValue()
		if len(args) >= 2 {
			level = args[1]
		}
		v, err := deflateEncode(args[0], level)
		return []Value{v}, err
	}, deflateEncode)

	// compress.deflateDecode(str) -> decompressed string, or nil, error
	deflateDecode := func(arg Value) (Value, Value, int, error) {
		if !arg.IsString() {
			return NilValue(), NilValue(), 0, fmt.Errorf("bad argument #1 to 'compress.deflateDecode' (string expected)")
		}
		decoded, err := basecompress.DeflateDecode(arg.Str(), func(r io.Reader) ([]byte, error) {
			return ReadAllWithHostResultLimit(r, hostResultLimit(maxHostResult))
		})
		if err != nil {
			return NilValue(), StringValue(err.Error()), 2, nil
		}
		return StringValue(decoded), NilValue(), 1, nil
	}
	setFastArg1Ret2("deflateDecode", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'compress.deflateDecode' (string expected)")
		}
		r0, r1, n, err := deflateDecode(args[0])
		if err != nil {
			return nil, err
		}
		if n == 1 {
			return []Value{r0}, nil
		}
		return []Value{r0, r1}, nil
	}, deflateDecode)

	return t
}
