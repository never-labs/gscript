package bind

import (
	"fmt"
	"strings"

	base64lib "github.com/never-labs/leia/internal/stdlib/lib/base64"
	encodinglib "github.com/never-labs/leia/internal/stdlib/lib/encoding"
	hashlib "github.com/never-labs/leia/internal/stdlib/lib/hash"
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
