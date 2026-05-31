package hash

import "testing"

func TestDigests(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "md5", got: MD5("hello"), want: "5d41402abc4b2a76b9719d911017c592"},
		{name: "sha1", got: SHA1("hello"), want: "aaf4c61ddcc5e8a2dabede0f3b482cd9aea9434d"},
		{name: "sha256", got: SHA256("hello"), want: "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"},
		{name: "sha512", got: SHA512("hello"), want: "9b71d224bd62f3785d96d46ad3ea3d73319bfbc2890caadae2dff72519673ca72323c3d99ba5c11d7c7acc6e14b8c5da0c4663475c2e5c3adef46f73bcdec043"},
		{name: "hmacSHA256", got: HMACSHA256("key", "hello"), want: "9307b3b915efb5171ff14d8cb55fbcc798c6c0ef1456d66ded1a6aa723a58b7b"},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Fatalf("%s = %q, want %q", tt.name, tt.got, tt.want)
		}
	}
}

func TestCRC32(t *testing.T) {
	if got := CRC32("hello"); got != 907060870 {
		t.Fatalf("CRC32(\"hello\") = %d, want 907060870", got)
	}
	if got := CRC32(""); got != 0 {
		t.Fatalf("CRC32(\"\") = %d, want 0", got)
	}
}
