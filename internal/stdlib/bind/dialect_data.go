package bind

import (
	"fmt"
	"io"
	"strings"

	base64lib "github.com/never-labs/leia/internal/stdlib/lib/base64"
	compresslib "github.com/never-labs/leia/internal/stdlib/lib/compress"
	encodinglib "github.com/never-labs/leia/internal/stdlib/lib/encoding"
	hashlib "github.com/never-labs/leia/internal/stdlib/lib/hash"
	uuidlib "github.com/never-labs/leia/internal/stdlib/lib/uuid"
	binfmt "github.com/never-labs/leia/internal/support/binaryfmt"
)

func registerDialectData(register dialectRegisterFunc, maxHostResult func() int64) {
	register([]string{"base64"}, dialectHandler{
		eval: func(body Value, options *Table) ([]Value, error) {
			return dialectBase64(body.Str(), options, maxHostResult)
		},
	})
	register([]string{"hash"}, dialectHandler{
		eval: func(body Value, options *Table) ([]Value, error) {
			return dialectHash(body.Str(), options)
		},
	})
	register([]string{"hex"}, dialectHandler{
		eval: func(body Value, options *Table) ([]Value, error) {
			return dialectHex(body.Str(), options, maxHostResult)
		},
	})
	register([]string{"base32"}, dialectHandler{
		eval: func(body Value, options *Table) ([]Value, error) {
			return dialectBase32(body.Str(), options, maxHostResult)
		},
	})
	register([]string{"uuid"}, dialectHandler{
		eval: func(body Value, options *Table) ([]Value, error) {
			return dialectUUID(body, options)
		},
	})
	register([]string{"gzip"}, dialectHandler{
		eval: func(body Value, options *Table) ([]Value, error) {
			return dialectCompress("gzip", body.Str(), options, maxHostResult)
		},
	})
	register([]string{"zlib"}, dialectHandler{
		eval: func(body Value, options *Table) ([]Value, error) {
			return dialectCompress("zlib", body.Str(), options, maxHostResult)
		},
	})
	register([]string{"deflate"}, dialectHandler{
		eval: func(body Value, options *Table) ([]Value, error) {
			return dialectCompress("deflate", body.Str(), options, maxHostResult)
		},
	})
	register([]string{"binary"}, dialectHandler{
		eval: func(body Value, options *Table) ([]Value, error) {
			return dialectBinary(body, options, maxHostResult)
		},
	})
}

func dialectBase64(src string, opts *Table, maxHostResult func() int64) ([]Value, error) {
	mode := "encode"
	if opts != nil && opts.RawGetString("mode").IsString() {
		mode = opts.RawGetString("mode").Str()
	}
	switch mode {
	case "encode", "":
		if err := CheckProjectedHostStringBytes(hostResultLimit(maxHostResult), base64lib.EncodedLen(len(src))); err != nil {
			return nil, err
		}
		return []Value{StringValue(base64lib.Encode(src))}, nil
	case "decode":
		if err := CheckProjectedHostStringBytes(hostResultLimit(maxHostResult), base64lib.DecodedLen(len(src))); err != nil {
			return nil, err
		}
		decoded, err := base64lib.Decode(src)
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		return []Value{StringValue(decoded)}, nil
	case "url_encode":
		if err := CheckProjectedHostStringBytes(hostResultLimit(maxHostResult), base64lib.URLEncodedLen(len(src))); err != nil {
			return nil, err
		}
		return []Value{StringValue(base64lib.URLEncode(src))}, nil
	case "url_decode":
		if err := CheckProjectedHostStringBytes(hostResultLimit(maxHostResult), base64lib.URLDecodedLen(len(src))); err != nil {
			return nil, err
		}
		decoded, err := base64lib.URLDecode(src)
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		return []Value{StringValue(decoded)}, nil
	default:
		return nil, fmt.Errorf("base64 dialect: unknown mode %q", mode)
	}
}

func dialectHash(src string, opts *Table) ([]Value, error) {
	algo := "sha256"
	if opts != nil && opts.RawGetString("algo").IsString() {
		algo = opts.RawGetString("algo").Str()
	}
	switch strings.ToLower(algo) {
	case "md5":
		return []Value{StringValue(hashlib.MD5(src))}, nil
	case "sha1":
		return []Value{StringValue(hashlib.SHA1(src))}, nil
	case "sha256", "":
		return []Value{StringValue(hashlib.SHA256(src))}, nil
	case "sha512":
		return []Value{StringValue(hashlib.SHA512(src))}, nil
	case "crc32":
		return []Value{IntValue(int64(hashlib.CRC32(src)))}, nil
	default:
		return nil, fmt.Errorf("hash dialect: unknown algorithm %q", algo)
	}
}

func dialectHex(src string, opts *Table, maxHostResult func() int64) ([]Value, error) {
	mode := "encode"
	if opts != nil && opts.RawGetString("mode").IsString() {
		mode = opts.RawGetString("mode").Str()
	}
	switch mode {
	case "encode", "":
		if err := CheckProjectedHostStringBytes(hostResultLimit(maxHostResult), encodinglib.HexEncodedLen(len(src))); err != nil {
			return nil, err
		}
		return []Value{StringValue(encodinglib.HexEncode(src))}, nil
	case "decode":
		if err := CheckProjectedHostStringBytes(hostResultLimit(maxHostResult), encodinglib.HexDecodedLen(len(src))); err != nil {
			return nil, err
		}
		decoded, err := encodinglib.HexDecode(src)
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		return []Value{StringValue(decoded)}, nil
	default:
		return nil, fmt.Errorf("hex dialect: unknown mode %q", mode)
	}
}

func dialectBase32(src string, opts *Table, maxHostResult func() int64) ([]Value, error) {
	mode := "encode"
	if opts != nil && opts.RawGetString("mode").IsString() {
		mode = opts.RawGetString("mode").Str()
	}
	switch mode {
	case "encode", "":
		if err := CheckProjectedHostStringBytes(hostResultLimit(maxHostResult), encodinglib.Base32EncodedLen(len(src))); err != nil {
			return nil, err
		}
		return []Value{StringValue(encodinglib.Base32Encode(src))}, nil
	case "decode":
		if err := CheckProjectedHostStringBytes(hostResultLimit(maxHostResult), encodinglib.Base32DecodedLen(len(src))); err != nil {
			return nil, err
		}
		decoded, err := encodinglib.Base32Decode(src)
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		return []Value{StringValue(decoded)}, nil
	case "hex_encode":
		if err := CheckProjectedHostStringBytes(hostResultLimit(maxHostResult), encodinglib.Base32HexEncodedLen(len(src))); err != nil {
			return nil, err
		}
		return []Value{StringValue(encodinglib.Base32HexEncode(src))}, nil
	case "hex_decode":
		if err := CheckProjectedHostStringBytes(hostResultLimit(maxHostResult), encodinglib.Base32HexDecodedLen(len(src))); err != nil {
			return nil, err
		}
		decoded, err := encodinglib.Base32HexDecode(src)
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		return []Value{StringValue(decoded)}, nil
	default:
		return nil, fmt.Errorf("base32 dialect: unknown mode %q", mode)
	}
}

func dialectUUID(body Value, opts *Table) ([]Value, error) {
	mode := "parse"
	if opts != nil && opts.RawGetString("mode").IsString() {
		mode = opts.RawGetString("mode").Str()
	}
	switch mode {
	case "", "parse", "validate":
		parsed, ok := uuidlib.Parse(body.String())
		if !ok {
			return []Value{NilValue(), StringValue("invalid UUID format")}, nil
		}
		out := NewTable()
		out.RawSetString("text", StringValue(body.String()))
		out.RawSetString("valid", BoolValue(true))
		out.RawSetString("version", IntValue(parsed.Version))
		out.RawSetString("variant", StringValue(parsed.Variant))
		out.RawSetString("bytes", StringValue(parsed.Bytes))
		out.RawSetString("nil", BoolValue(body.String() == uuidlib.Nil()))
		return []Value{TableValue(out)}, nil
	case "is_valid", "valid":
		return []Value{BoolValue(uuidlib.IsValid(body.String()))}, nil
	case "nil":
		return []Value{StringValue(uuidlib.Nil())}, nil
	default:
		return nil, fmt.Errorf("uuid dialect: unknown mode %q", mode)
	}
}

func dialectCompress(kind, src string, opts *Table, maxHostResult func() int64) ([]Value, error) {
	mode := "encode"
	if opts != nil && opts.RawGetString("mode").IsString() {
		mode = opts.RawGetString("mode").Str()
	}
	level := compressDefaultLevel(kind)
	if opts != nil && opts.RawGetString("level").IsNumber() {
		level = compresslib.NormalizeLevel(int(toInt(opts.RawGetString("level"))), level)
	}
	limit := hostResultLimit(maxHostResult)
	switch mode {
	case "", "encode", "compress":
		out, err := compressEncode(kind, src, level)
		if err != nil {
			return nil, fmt.Errorf("%s dialect: %v", kind, err)
		}
		if err := CheckProjectedHostStringBytes(limit, len(out)); err != nil {
			return nil, err
		}
		return []Value{StringValue(out)}, nil
	case "decode", "decompress":
		out, err := compressDecode(kind, src, limit)
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		return []Value{StringValue(out)}, nil
	default:
		return nil, fmt.Errorf("%s dialect: unknown mode %q", kind, mode)
	}
}

func compressDefaultLevel(kind string) int {
	switch kind {
	case "gzip":
		return compresslib.GzipDefaultLevel()
	case "zlib":
		return compresslib.ZlibDefaultLevel()
	default:
		return compresslib.DeflateDefaultLevel()
	}
}

func compressEncode(kind, src string, level int) (string, error) {
	switch kind {
	case "gzip":
		return compresslib.GzipEncode(src, level)
	case "zlib":
		return compresslib.ZlibEncode(src, level)
	default:
		return compresslib.DeflateEncode(src, level)
	}
}

func compressDecode(kind, src string, limit int64) (string, error) {
	readAll := func(r io.Reader) ([]byte, error) {
		return ReadAllWithHostResultLimit(r, limit)
	}
	switch kind {
	case "gzip":
		return compresslib.GzipDecode(src, readAll)
	case "zlib":
		return compresslib.ZlibDecode(src, readAll)
	default:
		return compresslib.DeflateDecode(src, readAll)
	}
}

func dialectBinary(body Value, opts *Table, maxHostResult func() int64) ([]Value, error) {
	mode := "unpack"
	if body.IsTable() {
		mode = "pack"
	}
	if opts != nil && opts.RawGetString("mode").IsString() {
		mode = opts.RawGetString("mode").Str()
	}
	format := ""
	if opts != nil && opts.RawGetString("format").IsString() {
		format = opts.RawGetString("format").Str()
	}
	if format == "" {
		return nil, fmt.Errorf("binary dialect: format option required")
	}
	switch mode {
	case "pack", "encode":
		if !body.IsTable() {
			return nil, fmt.Errorf("binary dialect: pack expects table body")
		}
		packed, err := binfmt.Pack("binary dialect", format, binaryDialectPackValues(body.Table()), hostResultLimit(maxHostResult))
		if err != nil {
			return nil, err
		}
		return []Value{StringValue(packed)}, nil
	case "", "unpack", "decode":
		offset := 1
		if opts != nil && opts.RawGetString("offset").IsNumber() {
			offset = int(toInt(opts.RawGetString("offset")))
		}
		values, next, err := binfmt.Unpack("binary dialect", format, body.String(), offset)
		if err != nil {
			if resultErr, ok := err.(binfmt.ResultError); ok {
				return []Value{NilValue(), StringValue(resultErr.Error())}, nil
			}
			return nil, err
		}
		out := NewTable()
		arr := NewAppendArrayTable(len(values))
		for i, value := range values {
			arr.RawSetInt(int64(i+1), binaryUnpackedValue(value))
		}
		out.RawSetString("values", TableValue(arr))
		out.RawSetString("next", IntValue(int64(next)))
		return []Value{TableValue(out)}, nil
	case "size":
		size, fixed, err := binfmt.Size("binary dialect", format)
		if err != nil {
			return nil, err
		}
		if !fixed {
			return []Value{NilValue(), StringValue("binary dialect: variable-size field in format")}, nil
		}
		return []Value{IntValue(int64(size))}, nil
	default:
		return nil, fmt.Errorf("binary dialect: unknown mode %q", mode)
	}
}

func binaryDialectPackValues(tbl *Table) []binfmt.PackValue {
	values := make([]binfmt.PackValue, 0, tbl.Length())
	for i := 1; i <= tbl.Length(); i++ {
		values = append(values, binaryPackValue(tbl.RawGetInt(int64(i))))
	}
	return values
}
