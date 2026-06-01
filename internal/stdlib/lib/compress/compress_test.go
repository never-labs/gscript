package compress

import (
	"errors"
	"io"
	"strings"
	"testing"
)

func TestGzipRoundTrip(t *testing.T) {
	input := strings.Repeat("gzip data ", 100)
	encoded, err := GzipEncode(input, NormalizeLevel(6, GzipDefaultLevel()))
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) >= len(input) {
		t.Fatalf("GzipEncode() length = %d, want less than %d", len(encoded), len(input))
	}
	decoded, err := GzipDecode(encoded, nil)
	if err != nil {
		t.Fatal(err)
	}
	if decoded != input {
		t.Fatalf("GzipDecode() = %q, want %q", decoded, input)
	}
}

func TestZlibRoundTrip(t *testing.T) {
	input := strings.Repeat("zlib data ", 100)
	encoded, err := ZlibEncode(input, NormalizeLevel(9, ZlibDefaultLevel()))
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) >= len(input) {
		t.Fatalf("ZlibEncode() length = %d, want less than %d", len(encoded), len(input))
	}
	decoded, err := ZlibDecode(encoded, nil)
	if err != nil {
		t.Fatal(err)
	}
	if decoded != input {
		t.Fatalf("ZlibDecode() = %q, want %q", decoded, input)
	}
}

func TestDeflateRoundTrip(t *testing.T) {
	input := strings.Repeat("deflate data ", 100)
	encoded, err := DeflateEncode(input, NormalizeLevel(1, DeflateDefaultLevel()))
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) >= len(input) {
		t.Fatalf("DeflateEncode() length = %d, want less than %d", len(encoded), len(input))
	}
	decoded, err := DeflateDecode(encoded, nil)
	if err != nil {
		t.Fatal(err)
	}
	if decoded != input {
		t.Fatalf("DeflateDecode() = %q, want %q", decoded, input)
	}
}

func TestEmptyRoundTrips(t *testing.T) {
	tests := []struct {
		name   string
		encode func(string, int) (string, error)
		decode func(string, ReadAllFunc) (string, error)
		level  int
	}{
		{name: "gzip", encode: GzipEncode, decode: GzipDecode, level: GzipDefaultLevel()},
		{name: "zlib", encode: ZlibEncode, decode: ZlibDecode, level: ZlibDefaultLevel()},
		{name: "deflate", encode: DeflateEncode, decode: DeflateDecode, level: DeflateDefaultLevel()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded, err := tt.encode("", tt.level)
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := tt.decode(encoded, nil)
			if err != nil {
				t.Fatal(err)
			}
			if decoded != "" {
				t.Fatalf("decode() = %q, want empty string", decoded)
			}
		})
	}
}

func TestDecodeRejectsInvalidData(t *testing.T) {
	tests := []struct {
		name   string
		decode func(string, ReadAllFunc) (string, error)
	}{
		{name: "gzip", decode: GzipDecode},
		{name: "zlib", decode: ZlibDecode},
		{name: "deflate", decode: DeflateDecode},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := tt.decode("not compressed data", nil); err == nil {
				t.Fatal("decode() accepted invalid data")
			}
		})
	}
}

func TestDecodeUsesReadAllAdapter(t *testing.T) {
	wantErr := errors.New("budget exceeded")
	encoded, err := GzipEncode("hello", GzipDefaultLevel())
	if err != nil {
		t.Fatal(err)
	}
	_, err = GzipDecode(encoded, func(_ io.Reader) ([]byte, error) {
		return nil, wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("GzipDecode() error = %v, want %v", err, wantErr)
	}
}

func TestNormalizeLevel(t *testing.T) {
	if got := NormalizeLevel(5, 6); got != 5 {
		t.Fatalf("NormalizeLevel(5, 6) = %d, want 5", got)
	}
	if got := NormalizeLevel(0, 6); got != 6 {
		t.Fatalf("NormalizeLevel(0, 6) = %d, want 6", got)
	}
	if got := NormalizeLevel(10, 6); got != 6 {
		t.Fatalf("NormalizeLevel(10, 6) = %d, want 6", got)
	}
}
