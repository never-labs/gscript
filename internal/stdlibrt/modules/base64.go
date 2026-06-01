package modules

import (
	"fmt"
	"github.com/never-labs/leia/internal/runtime"

	base64lib "github.com/never-labs/leia/internal/stdlib/base64"
)

// buildBase64Lib creates the "base64" standard library table.
func BuildBase64(maxHostResult func() int64) *runtime.Table {
	t := runtime.NewTable()
	installBase64GeneratedBindings(t, maxHostResult)
	return t
}

// base64.encode(str) -> standard base64 encoded string
func base64EncodeValue(maxHostResult func() int64, arg runtime.Value) (runtime.Value, error) {
	if !arg.IsString() {
		return runtime.NilValue(), fmt.Errorf("bad argument #1 to 'base64.encode' (string expected)")
	}
	if err := runtime.CheckProjectedHostStringBytes(maxHostResult(), base64lib.EncodedLen(runtime.StringLen(arg))); err != nil {
		return runtime.NilValue(), err
	}
	return runtime.StringValue(base64lib.Encode(arg.Str())), nil
}

// base64.decode(str) -> decoded string, or nil, "error message"
func base64DecodeValue(maxHostResult func() int64, arg runtime.Value) (runtime.Value, runtime.Value, int, error) {
	if !arg.IsString() {
		return runtime.NilValue(), runtime.NilValue(), 0, fmt.Errorf("bad argument #1 to 'base64.decode' (string expected)")
	}
	if err := runtime.CheckProjectedHostStringBytes(maxHostResult(), base64lib.DecodedLen(runtime.StringLen(arg))); err != nil {
		return runtime.NilValue(), runtime.NilValue(), 0, err
	}
	decoded, err := base64lib.Decode(arg.Str())
	if err != nil {
		return runtime.NilValue(), runtime.StringValue(err.Error()), 2, nil
	}
	return runtime.StringValue(decoded), runtime.NilValue(), 1, nil
}

// base64.urlEncode(str) -> URL-safe base64 encoded string (no padding)
func base64URLEncodeValue(maxHostResult func() int64, arg runtime.Value) (runtime.Value, error) {
	if !arg.IsString() {
		return runtime.NilValue(), fmt.Errorf("bad argument #1 to 'base64.urlEncode' (string expected)")
	}
	if err := runtime.CheckProjectedHostStringBytes(maxHostResult(), base64lib.URLEncodedLen(runtime.StringLen(arg))); err != nil {
		return runtime.NilValue(), err
	}
	return runtime.StringValue(base64lib.URLEncode(arg.Str())), nil
}

// base64.urlDecode(str) -> decoded string, or nil, "error message"
func base64URLDecodeValue(maxHostResult func() int64, arg runtime.Value) (runtime.Value, runtime.Value, int, error) {
	if !arg.IsString() {
		return runtime.NilValue(), runtime.NilValue(), 0, fmt.Errorf("bad argument #1 to 'base64.urlDecode' (string expected)")
	}
	if err := runtime.CheckProjectedHostStringBytes(maxHostResult(), base64lib.URLDecodedLen(runtime.StringLen(arg))); err != nil {
		return runtime.NilValue(), runtime.NilValue(), 0, err
	}
	decoded, err := base64lib.URLDecode(arg.Str())
	if err != nil {
		return runtime.NilValue(), runtime.StringValue(err.Error()), 2, nil
	}
	return runtime.StringValue(decoded), runtime.NilValue(), 1, nil
}
