package modules

import (
	"fmt"
	"github.com/never-labs/leia/internal/runtime"

	basehash "github.com/never-labs/leia/internal/stdlib/hash"
)

// buildHashLib creates the "hash" standard library table.
func BuildHash() *runtime.Table {
	t := runtime.NewTable()
	installHashGeneratedBindings(t)
	return t
}

func hashMD5Value(arg runtime.Value) (runtime.Value, error) {
	if !arg.IsString() {
		return runtime.NilValue(), fmt.Errorf("bad argument #1 to 'hash.md5' (string expected)")
	}
	return runtime.StringValue(basehash.MD5(arg.Str())), nil
}

func hashSHA1Value(arg runtime.Value) (runtime.Value, error) {
	if !arg.IsString() {
		return runtime.NilValue(), fmt.Errorf("bad argument #1 to 'hash.sha1' (string expected)")
	}
	return runtime.StringValue(basehash.SHA1(arg.Str())), nil
}

func hashSHA256Value(arg runtime.Value) (runtime.Value, error) {
	if !arg.IsString() {
		return runtime.NilValue(), fmt.Errorf("bad argument #1 to 'hash.sha256' (string expected)")
	}
	return runtime.StringValue(basehash.SHA256(arg.Str())), nil
}

func hashSHA512Value(arg runtime.Value) (runtime.Value, error) {
	if !arg.IsString() {
		return runtime.NilValue(), fmt.Errorf("bad argument #1 to 'hash.sha512' (string expected)")
	}
	return runtime.StringValue(basehash.SHA512(arg.Str())), nil
}

func hashCRC32Value(arg runtime.Value) (runtime.Value, error) {
	if !arg.IsString() {
		return runtime.NilValue(), fmt.Errorf("bad argument #1 to 'hash.crc32' (string expected)")
	}
	return runtime.IntValue(int64(basehash.CRC32(arg.Str()))), nil
}

func hashHMACSHA256Value(key, message runtime.Value) (runtime.Value, error) {
	if !key.IsString() {
		return runtime.NilValue(), fmt.Errorf("bad argument #1 to 'hash.hmacSHA256' (string expected)")
	}
	if !message.IsString() {
		return runtime.NilValue(), fmt.Errorf("bad argument #2 to 'hash.hmacSHA256' (string expected)")
	}
	return runtime.StringValue(basehash.HMACSHA256(key.Str(), message.Str())), nil
}
