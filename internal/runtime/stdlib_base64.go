package runtime

import (
	"fmt"

	base64lib "github.com/never-labs/gscript/internal/stdlib/base64"
)

// buildBase64Lib creates the "base64" standard library table.
func buildBase64Lib(interps ...*Interpreter) *Table {
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

	installBase64GeneratedBindings(t, maxHostResult)
	return t
}

// base64.encode(str) -> standard base64 encoded string
func base64EncodeValue(maxHostResult func() int64, arg Value) (Value, error) {
	if !arg.IsString() {
		return NilValue(), fmt.Errorf("bad argument #1 to 'base64.encode' (string expected)")
	}
	if err := CheckProjectedHostStringBytes(maxHostResult(), base64lib.EncodedLen(StringLen(arg))); err != nil {
		return NilValue(), err
	}
	return StringValue(base64lib.Encode(arg.Str())), nil
}

// base64.decode(str) -> decoded string, or nil, "error message"
func base64DecodeValue(maxHostResult func() int64, arg Value) (Value, Value, int, error) {
	if !arg.IsString() {
		return NilValue(), NilValue(), 0, fmt.Errorf("bad argument #1 to 'base64.decode' (string expected)")
	}
	if err := CheckProjectedHostStringBytes(maxHostResult(), base64lib.DecodedLen(StringLen(arg))); err != nil {
		return NilValue(), NilValue(), 0, err
	}
	decoded, err := base64lib.Decode(arg.Str())
	if err != nil {
		return NilValue(), StringValue(err.Error()), 2, nil
	}
	return StringValue(decoded), NilValue(), 1, nil
}

// base64.urlEncode(str) -> URL-safe base64 encoded string (no padding)
func base64URLEncodeValue(maxHostResult func() int64, arg Value) (Value, error) {
	if !arg.IsString() {
		return NilValue(), fmt.Errorf("bad argument #1 to 'base64.urlEncode' (string expected)")
	}
	if err := CheckProjectedHostStringBytes(maxHostResult(), base64lib.URLEncodedLen(StringLen(arg))); err != nil {
		return NilValue(), err
	}
	return StringValue(base64lib.URLEncode(arg.Str())), nil
}

// base64.urlDecode(str) -> decoded string, or nil, "error message"
func base64URLDecodeValue(maxHostResult func() int64, arg Value) (Value, Value, int, error) {
	if !arg.IsString() {
		return NilValue(), NilValue(), 0, fmt.Errorf("bad argument #1 to 'base64.urlDecode' (string expected)")
	}
	if err := CheckProjectedHostStringBytes(maxHostResult(), base64lib.URLDecodedLen(StringLen(arg))); err != nil {
		return NilValue(), NilValue(), 0, err
	}
	decoded, err := base64lib.URLDecode(arg.Str())
	if err != nil {
		return NilValue(), StringValue(err.Error()), 2, nil
	}
	return StringValue(decoded), NilValue(), 1, nil
}
