package bind

import (
	"testing"
)

// base64Interp creates an interpreter with the base64 library manually registered.
func base64Interp(t *testing.T, src string) *Interpreter {
	t.Helper()
	return runWithLib(t, src, "base64", BuildBase64(func() int64 { return 0 }))
}

// ==================================================================
// base64 encode/decode tests
// ==================================================================

func TestBase64Encode(t *testing.T) {
	interp := base64Interp(t, `result := base64.encode("hello world")`)
	v := interp.GetGlobal("result")
	if v.Str() != "aGVsbG8gd29ybGQ=" {
		t.Errorf("expected 'aGVsbG8gd29ybGQ=', got %v", v.Str())
	}
}

func TestBase64EncodeEmpty(t *testing.T) {
	interp := base64Interp(t, `result := base64.encode("")`)
	v := interp.GetGlobal("result")
	if v.Str() != "" {
		t.Errorf("expected empty string, got %v", v.Str())
	}
}

func TestBase64Decode(t *testing.T) {
	interp := base64Interp(t, `result := base64.decode("aGVsbG8gd29ybGQ=")`)
	v := interp.GetGlobal("result")
	if v.Str() != "hello world" {
		t.Errorf("expected 'hello world', got %v", v.Str())
	}
}

func TestBase64DecodeError(t *testing.T) {
	interp := base64Interp(t, `result, err := base64.decode("!!!invalid!!!")`)
	result := interp.GetGlobal("result")
	errMsg := interp.GetGlobal("err")
	if !result.IsNil() {
		t.Errorf("expected nil on decode error, got %v", result)
	}
	if !errMsg.IsString() || errMsg.Str() == "" {
		t.Errorf("expected error message, got %v", errMsg)
	}
}

func TestBase64Roundtrip(t *testing.T) {
	interp := base64Interp(t, `
		original := "Hello, Leia!"
		encoded := base64.encode(original)
		decoded := base64.decode(encoded)
		result := decoded == original
	`)
	v := interp.GetGlobal("result")
	if !v.IsBool() || !v.Bool() {
		t.Errorf("expected roundtrip to match, got %v", v)
	}
}

// ==================================================================
// base64 URL encode/decode tests
// ==================================================================

func TestBase64URLEncode(t *testing.T) {
	// URL-safe base64 uses - and _ instead of + and /
	interp := base64Interp(t, `result := base64.urlEncode("subjects?_d")`)
	v := interp.GetGlobal("result")
	s := v.Str()
	// URL-safe should not contain + or / or =
	for _, c := range s {
		if c == '+' || c == '/' || c == '=' {
			t.Errorf("URL-safe base64 should not contain '+', '/' or '=', got %v", s)
			break
		}
	}
}

func TestBase64URLDecode(t *testing.T) {
	interp := base64Interp(t, `
		encoded := base64.urlEncode("hello world")
		result := base64.urlDecode(encoded)
	`)
	v := interp.GetGlobal("result")
	if v.Str() != "hello world" {
		t.Errorf("expected 'hello world', got %v", v.Str())
	}
}

func TestBase64URLDecodeError(t *testing.T) {
	interp := base64Interp(t, `result, err := base64.urlDecode("!!!invalid!!!")`)
	result := interp.GetGlobal("result")
	errMsg := interp.GetGlobal("err")
	if !result.IsNil() {
		t.Errorf("expected nil on URL decode error, got %v", result)
	}
	if !errMsg.IsString() || errMsg.Str() == "" {
		t.Errorf("expected error message, got %v", errMsg)
	}
}

func TestBase64URLRoundtrip(t *testing.T) {
	interp := base64Interp(t, `
		original := "Hello+World/Test==Foo"
		encoded := base64.urlEncode(original)
		decoded := base64.urlDecode(encoded)
		result := decoded == original
	`)
	v := interp.GetGlobal("result")
	if !v.IsBool() || !v.Bool() {
		t.Errorf("expected URL roundtrip to match, got %v", v)
	}
}

func TestBase64GeneratedBindingsExposeFastPathsAndFallbacks(t *testing.T) {
	lib := BuildBase64(func() int64 { return 0 })

	encode := lib.RawGetString("encode").GoFunction()
	if encode == nil || encode.FastArg1 == nil {
		t.Fatalf("base64.encode missing FastArg1: %#v", encode)
	}
	fastEncoded, err := encode.FastArg1(StringValue("hello"))
	if err != nil {
		t.Fatalf("FastArg1 encode failed: %v", err)
	}
	fallbackEncoded, err := encode.Fn([]Value{StringValue("hello")})
	if err != nil {
		t.Fatalf("Fn encode failed: %v", err)
	}
	if len(fallbackEncoded) != 1 || fallbackEncoded[0].Str() != fastEncoded.Str() {
		t.Fatalf("encode fallback mismatch: fast=%v fallback=%v", fastEncoded, fallbackEncoded)
	}

	decode := lib.RawGetString("decode").GoFunction()
	if decode == nil || decode.FastArg1Ret2 == nil {
		t.Fatalf("base64.decode missing FastArg1Ret2: %#v", decode)
	}
	r0, _, n, err := decode.FastArg1Ret2(StringValue("aGVsbG8="))
	if err != nil {
		t.Fatalf("FastArg1Ret2 decode failed: %v", err)
	}
	if n != 1 || r0.Str() != "hello" {
		t.Fatalf("unexpected fast decode result: r0=%v n=%d", r0, n)
	}
	fallbackDecoded, err := decode.Fn([]Value{StringValue("aGVsbG8=")})
	if err != nil {
		t.Fatalf("Fn decode failed: %v", err)
	}
	if len(fallbackDecoded) != 1 || fallbackDecoded[0].Str() != "hello" {
		t.Fatalf("decode fallback mismatch: %v", fallbackDecoded)
	}
	fallbackDecoded, err = decode.Fn([]Value{StringValue("!!!invalid!!!")})
	if err != nil {
		t.Fatalf("Fn decode invalid returned hard error: %v", err)
	}
	if len(fallbackDecoded) != 2 || !fallbackDecoded[0].IsNil() || !fallbackDecoded[1].IsString() {
		t.Fatalf("decode fallback should return nil,error values, got %v", fallbackDecoded)
	}
}
