package support

import (
	"os"
	"testing"
)

func TestParseFileMode(t *testing.T) {
	tests := []struct {
		mode string
		flag int
		ok   bool
	}{
		{mode: "r", flag: os.O_RDONLY, ok: true},
		{mode: "rb", flag: os.O_RDONLY, ok: true},
		{mode: "w", flag: os.O_WRONLY | os.O_CREATE | os.O_TRUNC, ok: true},
		{mode: "a", flag: os.O_WRONLY | os.O_CREATE | os.O_APPEND, ok: true},
		{mode: "r+", flag: os.O_RDWR, ok: true},
		{mode: "w+b", flag: os.O_RDWR | os.O_CREATE | os.O_TRUNC, ok: true},
		{mode: "a+", flag: os.O_RDWR | os.O_CREATE | os.O_APPEND, ok: true},
		{mode: "x", ok: false},
	}
	for _, tt := range tests {
		flag, ok := ParseFileMode(tt.mode)
		if ok != tt.ok || flag != tt.flag {
			t.Fatalf("ParseFileMode(%q) = (%d, %v), want (%d, %v)", tt.mode, flag, ok, tt.flag, tt.ok)
		}
	}
}

func TestFileModeAccess(t *testing.T) {
	tests := []struct {
		name      string
		flag      int
		wantRead  bool
		wantWrite bool
	}{
		{name: "read only", flag: os.O_RDONLY, wantRead: true, wantWrite: false},
		{name: "write only", flag: os.O_WRONLY | os.O_CREATE, wantRead: false, wantWrite: true},
		{name: "read write", flag: os.O_RDWR, wantRead: true, wantWrite: true},
	}
	for _, tt := range tests {
		read, write := FileModeAccess(tt.flag)
		if read != tt.wantRead || write != tt.wantWrite {
			t.Fatalf("%s: FileModeAccess(%d) = (%v, %v), want (%v, %v)", tt.name, tt.flag, read, write, tt.wantRead, tt.wantWrite)
		}
	}
}
