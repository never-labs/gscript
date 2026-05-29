package runtime

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"fmt"
	"io"
	"strings"
	"sync"
)

var (
	stdlibGzipWriterPools    [10]sync.Pool
	stdlibZlibWriterPools    [10]sync.Pool
	stdlibDeflateWriterPools [10]sync.Pool
	stdlibGzipReaderPool     sync.Pool
	stdlibZlibReaderPool     sync.Pool
	stdlibDeflateReaderPool  sync.Pool
)

type resetReadCloser interface {
	io.ReadCloser
	Reset(io.Reader, []byte) error
}

func normalizeCompressLevel(level, def int) int {
	if level < 1 || level > 9 {
		return def
	}
	return level
}

func compressLevelIndex(level int) (int, bool) {
	if level < 1 || level > 9 {
		return 0, false
	}
	return level, true
}

// buildCompressLib creates the "compress" standard library table.
// Provides gzip, zlib, and deflate compression/decompression.
// Inspired by Odin's compress package (gzip, zlib).
func buildCompressLib(interps ...*Interpreter) *Table {
	t := NewTable()
	var interp *Interpreter
	if len(interps) > 0 {
		interp = interps[0]
	}
	maxHostResult := func() int64 {
		if interp == nil {
			return 0
		}
		return interp.maxHostResult
	}

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
		level := gzip.DefaultCompression
		if levelVal.IsNumber() {
			level = normalizeCompressLevel(int(toInt(levelVal)), gzip.DefaultCompression)
		}
		var buf bytes.Buffer
		var w *gzip.Writer
		poolIdx, poolOK := compressLevelIndex(level)
		if idx, ok := compressLevelIndex(level); ok {
			if cached := stdlibGzipWriterPools[idx].Get(); cached != nil {
				w = cached.(*gzip.Writer)
				w.Reset(&buf)
			}
		}
		if w == nil {
			var err error
			w, err = gzip.NewWriterLevel(&buf, level)
			if err != nil {
				return NilValue(), fmt.Errorf("compress.gzipEncode: %v", err)
			}
		}
		if _, err := w.Write([]byte(dataVal.Str())); err != nil {
			return NilValue(), fmt.Errorf("compress.gzipEncode: %v", err)
		}
		if err := w.Close(); err != nil {
			return NilValue(), fmt.Errorf("compress.gzipEncode: %v", err)
		}
		out := StringValue(buf.String())
		if poolOK {
			w.Reset(io.Discard)
			stdlibGzipWriterPools[poolIdx].Put(w)
		}
		return out, nil
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
		src := strings.NewReader(arg.Str())
		var r *gzip.Reader
		if cached := stdlibGzipReaderPool.Get(); cached != nil {
			r = cached.(*gzip.Reader)
			if err := r.Reset(src); err != nil {
				return NilValue(), StringValue(err.Error()), 2, nil
			}
		} else {
			var err error
			r, err = gzip.NewReader(src)
			if err != nil {
				return NilValue(), StringValue(err.Error()), 2, nil
			}
		}
		decoded, err := ReadAllWithHostResultLimit(r, maxHostResult())
		closeErr := r.Close()
		stdlibGzipReaderPool.Put(r)
		if err != nil {
			return NilValue(), StringValue(err.Error()), 2, nil
		}
		if closeErr != nil {
			return NilValue(), StringValue(closeErr.Error()), 2, nil
		}
		return StringValue(string(decoded)), NilValue(), 1, nil
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
		level := zlib.DefaultCompression
		if levelVal.IsNumber() {
			level = normalizeCompressLevel(int(toInt(levelVal)), zlib.DefaultCompression)
		}
		var buf bytes.Buffer
		var w *zlib.Writer
		poolIdx, poolOK := compressLevelIndex(level)
		if idx, ok := compressLevelIndex(level); ok {
			if cached := stdlibZlibWriterPools[idx].Get(); cached != nil {
				w = cached.(*zlib.Writer)
				w.Reset(&buf)
			}
		}
		if w == nil {
			var err error
			w, err = zlib.NewWriterLevel(&buf, level)
			if err != nil {
				return NilValue(), fmt.Errorf("compress.zlibEncode: %v", err)
			}
		}
		if _, err := w.Write([]byte(dataVal.Str())); err != nil {
			return NilValue(), fmt.Errorf("compress.zlibEncode: %v", err)
		}
		if err := w.Close(); err != nil {
			return NilValue(), fmt.Errorf("compress.zlibEncode: %v", err)
		}
		out := StringValue(buf.String())
		if poolOK {
			w.Reset(io.Discard)
			stdlibZlibWriterPools[poolIdx].Put(w)
		}
		return out, nil
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
		src := strings.NewReader(arg.Str())
		var r resetReadCloser
		if cached := stdlibZlibReaderPool.Get(); cached != nil {
			r = cached.(resetReadCloser)
			if err := r.Reset(src, nil); err != nil {
				return NilValue(), StringValue(err.Error()), 2, nil
			}
		} else {
			newReader, err := zlib.NewReader(src)
			if err != nil {
				return NilValue(), StringValue(err.Error()), 2, nil
			}
			resetter, ok := newReader.(resetReadCloser)
			if !ok {
				defer newReader.Close()
				decoded, err := ReadAllWithHostResultLimit(newReader, maxHostResult())
				if err != nil {
					return NilValue(), StringValue(err.Error()), 2, nil
				}
				return StringValue(string(decoded)), NilValue(), 1, nil
			}
			r = resetter
		}
		decoded, err := ReadAllWithHostResultLimit(r, maxHostResult())
		closeErr := r.Close()
		stdlibZlibReaderPool.Put(r)
		if err != nil {
			return NilValue(), StringValue(err.Error()), 2, nil
		}
		if closeErr != nil {
			return NilValue(), StringValue(closeErr.Error()), 2, nil
		}
		return StringValue(string(decoded)), NilValue(), 1, nil
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
		level := flate.DefaultCompression
		if levelVal.IsNumber() {
			level = normalizeCompressLevel(int(toInt(levelVal)), flate.DefaultCompression)
		}
		var buf bytes.Buffer
		var w *flate.Writer
		poolIdx, poolOK := compressLevelIndex(level)
		if idx, ok := compressLevelIndex(level); ok {
			if cached := stdlibDeflateWriterPools[idx].Get(); cached != nil {
				w = cached.(*flate.Writer)
				w.Reset(&buf)
			}
		}
		if w == nil {
			var err error
			w, err = flate.NewWriter(&buf, level)
			if err != nil {
				return NilValue(), fmt.Errorf("compress.deflateEncode: %v", err)
			}
		}
		if _, err := w.Write([]byte(dataVal.Str())); err != nil {
			return NilValue(), fmt.Errorf("compress.deflateEncode: %v", err)
		}
		if err := w.Close(); err != nil {
			return NilValue(), fmt.Errorf("compress.deflateEncode: %v", err)
		}
		out := StringValue(buf.String())
		if poolOK {
			w.Reset(io.Discard)
			stdlibDeflateWriterPools[poolIdx].Put(w)
		}
		return out, nil
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
		src := strings.NewReader(arg.Str())
		var r resetReadCloser
		if cached := stdlibDeflateReaderPool.Get(); cached != nil {
			r = cached.(resetReadCloser)
			if err := r.Reset(src, nil); err != nil {
				return NilValue(), StringValue(err.Error()), 2, nil
			}
		} else {
			r = flate.NewReader(src).(resetReadCloser)
		}
		decoded, err := ReadAllWithHostResultLimit(r, maxHostResult())
		closeErr := r.Close()
		stdlibDeflateReaderPool.Put(r)
		if err != nil {
			return NilValue(), StringValue(err.Error()), 2, nil
		}
		if closeErr != nil {
			return NilValue(), StringValue(closeErr.Error()), 2, nil
		}
		return StringValue(string(decoded)), NilValue(), 1, nil
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
