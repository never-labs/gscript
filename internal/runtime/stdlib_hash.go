package runtime

import (
	"fmt"

	basehash "github.com/never-labs/gscript/internal/stdlib/hash"
)

// buildHashLib creates the "hash" standard library table.
func buildHashLib() *Table {
	t := NewTable()
	installHashGeneratedBindings(t)
	return t
}

func hashMD5Value(arg Value) (Value, error) {
	if !arg.IsString() {
		return NilValue(), fmt.Errorf("bad argument #1 to 'hash.md5' (string expected)")
	}
	return StringValue(basehash.MD5(arg.Str())), nil
}

func hashSHA1Value(arg Value) (Value, error) {
	if !arg.IsString() {
		return NilValue(), fmt.Errorf("bad argument #1 to 'hash.sha1' (string expected)")
	}
	return StringValue(basehash.SHA1(arg.Str())), nil
}

func hashSHA256Value(arg Value) (Value, error) {
	if !arg.IsString() {
		return NilValue(), fmt.Errorf("bad argument #1 to 'hash.sha256' (string expected)")
	}
	return StringValue(basehash.SHA256(arg.Str())), nil
}

func hashSHA512Value(arg Value) (Value, error) {
	if !arg.IsString() {
		return NilValue(), fmt.Errorf("bad argument #1 to 'hash.sha512' (string expected)")
	}
	return StringValue(basehash.SHA512(arg.Str())), nil
}

func hashCRC32Value(arg Value) (Value, error) {
	if !arg.IsString() {
		return NilValue(), fmt.Errorf("bad argument #1 to 'hash.crc32' (string expected)")
	}
	return IntValue(int64(basehash.CRC32(arg.Str()))), nil
}

func hashHMACSHA256Value(key, message Value) (Value, error) {
	if !key.IsString() {
		return NilValue(), fmt.Errorf("bad argument #1 to 'hash.hmacSHA256' (string expected)")
	}
	if !message.IsString() {
		return NilValue(), fmt.Errorf("bad argument #2 to 'hash.hmacSHA256' (string expected)")
	}
	return StringValue(basehash.HMACSHA256(key.Str(), message.Str())), nil
}
