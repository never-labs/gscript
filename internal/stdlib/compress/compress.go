package compress

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"io"
	"strings"
	"sync"
)

var (
	gzipWriterPools    [10]sync.Pool
	zlibWriterPools    [10]sync.Pool
	deflateWriterPools [10]sync.Pool
	gzipReaderPool     sync.Pool
	zlibReaderPool     sync.Pool
	deflateReaderPool  sync.Pool
)

type resetReadCloser interface {
	io.ReadCloser
	Reset(io.Reader, []byte) error
}

type ReadAllFunc func(io.Reader) ([]byte, error)

func NormalizeLevel(level, def int) int {
	if level < 1 || level > 9 {
		return def
	}
	return level
}

func levelIndex(level int) (int, bool) {
	if level < 1 || level > 9 {
		return 0, false
	}
	return level, true
}

func GzipDefaultLevel() int {
	return gzip.DefaultCompression
}

func ZlibDefaultLevel() int {
	return zlib.DefaultCompression
}

func DeflateDefaultLevel() int {
	return flate.DefaultCompression
}

func GzipEncode(data string, level int) (string, error) {
	var buf bytes.Buffer
	var w *gzip.Writer
	poolIdx, poolOK := levelIndex(level)
	if poolOK {
		if cached := gzipWriterPools[poolIdx].Get(); cached != nil {
			w = cached.(*gzip.Writer)
			w.Reset(&buf)
		}
	}
	if w == nil {
		var err error
		w, err = gzip.NewWriterLevel(&buf, level)
		if err != nil {
			return "", err
		}
	}
	if _, err := w.Write([]byte(data)); err != nil {
		return "", err
	}
	if err := w.Close(); err != nil {
		return "", err
	}
	out := buf.String()
	if poolOK {
		w.Reset(io.Discard)
		gzipWriterPools[poolIdx].Put(w)
	}
	return out, nil
}

func GzipDecode(data string, readAll ReadAllFunc) (string, error) {
	if readAll == nil {
		readAll = io.ReadAll
	}
	src := strings.NewReader(data)
	var r *gzip.Reader
	if cached := gzipReaderPool.Get(); cached != nil {
		r = cached.(*gzip.Reader)
		if err := r.Reset(src); err != nil {
			return "", err
		}
	} else {
		var err error
		r, err = gzip.NewReader(src)
		if err != nil {
			return "", err
		}
	}
	decoded, err := readAll(r)
	closeErr := r.Close()
	gzipReaderPool.Put(r)
	if err != nil {
		return "", err
	}
	if closeErr != nil {
		return "", closeErr
	}
	return string(decoded), nil
}

func ZlibEncode(data string, level int) (string, error) {
	var buf bytes.Buffer
	var w *zlib.Writer
	poolIdx, poolOK := levelIndex(level)
	if poolOK {
		if cached := zlibWriterPools[poolIdx].Get(); cached != nil {
			w = cached.(*zlib.Writer)
			w.Reset(&buf)
		}
	}
	if w == nil {
		var err error
		w, err = zlib.NewWriterLevel(&buf, level)
		if err != nil {
			return "", err
		}
	}
	if _, err := w.Write([]byte(data)); err != nil {
		return "", err
	}
	if err := w.Close(); err != nil {
		return "", err
	}
	out := buf.String()
	if poolOK {
		w.Reset(io.Discard)
		zlibWriterPools[poolIdx].Put(w)
	}
	return out, nil
}

func ZlibDecode(data string, readAll ReadAllFunc) (string, error) {
	if readAll == nil {
		readAll = io.ReadAll
	}
	src := strings.NewReader(data)
	var r resetReadCloser
	if cached := zlibReaderPool.Get(); cached != nil {
		r = cached.(resetReadCloser)
		if err := r.Reset(src, nil); err != nil {
			return "", err
		}
	} else {
		newReader, err := zlib.NewReader(src)
		if err != nil {
			return "", err
		}
		resetter, ok := newReader.(resetReadCloser)
		if !ok {
			defer newReader.Close()
			decoded, err := readAll(newReader)
			if err != nil {
				return "", err
			}
			return string(decoded), nil
		}
		r = resetter
	}
	decoded, err := readAll(r)
	closeErr := r.Close()
	zlibReaderPool.Put(r)
	if err != nil {
		return "", err
	}
	if closeErr != nil {
		return "", closeErr
	}
	return string(decoded), nil
}

func DeflateEncode(data string, level int) (string, error) {
	var buf bytes.Buffer
	var w *flate.Writer
	poolIdx, poolOK := levelIndex(level)
	if poolOK {
		if cached := deflateWriterPools[poolIdx].Get(); cached != nil {
			w = cached.(*flate.Writer)
			w.Reset(&buf)
		}
	}
	if w == nil {
		var err error
		w, err = flate.NewWriter(&buf, level)
		if err != nil {
			return "", err
		}
	}
	if _, err := w.Write([]byte(data)); err != nil {
		return "", err
	}
	if err := w.Close(); err != nil {
		return "", err
	}
	out := buf.String()
	if poolOK {
		w.Reset(io.Discard)
		deflateWriterPools[poolIdx].Put(w)
	}
	return out, nil
}

func DeflateDecode(data string, readAll ReadAllFunc) (string, error) {
	if readAll == nil {
		readAll = io.ReadAll
	}
	src := strings.NewReader(data)
	var r resetReadCloser
	if cached := deflateReaderPool.Get(); cached != nil {
		r = cached.(resetReadCloser)
		if err := r.Reset(src, nil); err != nil {
			return "", err
		}
	} else {
		r = flate.NewReader(src).(resetReadCloser)
	}
	decoded, err := readAll(r)
	closeErr := r.Close()
	deflateReaderPool.Put(r)
	if err != nil {
		return "", err
	}
	if closeErr != nil {
		return "", closeErr
	}
	return string(decoded), nil
}
