package io

import (
	"bufio"
	stdio "io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolvePathChecksCapabilities(t *testing.T) {
	if _, err := ResolvePath("", "in.txt", false, true, true, false); err == nil {
		t.Fatalf("ResolvePath allowed read without read capability")
	}
	if _, err := ResolvePath("", "out.txt", true, false, false, true); err == nil {
		t.Fatalf("ResolvePath allowed write without write capability")
	}
}

func TestResolvePathUsesSandboxRoot(t *testing.T) {
	root := t.TempDir()
	got, err := ResolvePath(root, "sub/file.txt", true, true, true, false)
	if err != nil {
		t.Fatalf("ResolvePath returned error: %v", err)
	}
	want := filepath.Join(root, "sub", "file.txt")
	if got != want {
		t.Fatalf("ResolvePath = %q, want %q", got, want)
	}
	if _, err := ResolvePath(root, "../outside.txt", true, true, true, false); err == nil {
		t.Fatalf("ResolvePath accepted path escape")
	}
}

func TestSeekWhence(t *testing.T) {
	tests := map[string]int{
		"set": stdio.SeekStart,
		"cur": stdio.SeekCurrent,
		"end": stdio.SeekEnd,
	}
	for input, want := range tests {
		got, err := SeekWhence(input)
		if err != nil {
			t.Fatalf("SeekWhence(%q) returned error: %v", input, err)
		}
		if got != want {
			t.Fatalf("SeekWhence(%q) = %d, want %d", input, got, want)
		}
	}
	if _, err := SeekWhence("bad"); err == nil {
		t.Fatalf("SeekWhence accepted invalid whence")
	}
}

func TestReadOneFormats(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("line\r\n42\nrest"))
	got, err := ReadOne(reader, StringFormat("*l"))
	if err != nil {
		t.Fatalf("ReadOne line returned error: %v", err)
	}
	if got.Kind != ReadString || got.String != "line" {
		t.Fatalf("ReadOne line = %#v", got)
	}
	got, err = ReadOne(reader, StringFormat("*n"))
	if err != nil {
		t.Fatalf("ReadOne number returned error: %v", err)
	}
	if got.Kind != ReadInt || got.Int != 42 {
		t.Fatalf("ReadOne number = %#v", got)
	}
	got, err = ReadOne(reader, StringFormat("*a"))
	if err != nil {
		t.Fatalf("ReadOne all returned error: %v", err)
	}
	if got.Kind != ReadString || got.String != "rest" {
		t.Fatalf("ReadOne all = %#v", got)
	}
}

func TestReadOneCount(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("abc"))
	got, err := ReadOne(reader, CountFormat(2))
	if err != nil {
		t.Fatalf("ReadOne count returned error: %v", err)
	}
	if got.Kind != ReadString || got.String != "ab" {
		t.Fatalf("ReadOne count = %#v", got)
	}
	got, err = ReadOne(reader, CountFormat(0))
	if err != nil {
		t.Fatalf("ReadOne zero count returned error: %v", err)
	}
	if got.Kind != ReadString || got.String != "" {
		t.Fatalf("ReadOne zero count = %#v", got)
	}
	got, err = ReadOne(reader, CountFormat(5))
	if err != nil {
		t.Fatalf("ReadOne trailing count returned error: %v", err)
	}
	if got.Kind != ReadString || got.String != "c" {
		t.Fatalf("ReadOne trailing count = %#v", got)
	}
	got, err = ReadOne(reader, CountFormat(0))
	if err != nil {
		t.Fatalf("ReadOne eof zero count returned error: %v", err)
	}
	if got.Kind != ReadNil {
		t.Fatalf("ReadOne eof zero count = %#v", got)
	}
}

func TestReadOneErrors(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader(""))
	if _, err := ReadOne(reader, CountFormat(-1)); err == nil {
		t.Fatalf("ReadOne accepted negative count")
	}
	if _, err := ReadOne(reader, StringFormat("*x")); err == nil {
		t.Fatalf("ReadOne accepted invalid string format")
	}
}

func TestResolvePathUnrestricted(t *testing.T) {
	path := string(os.PathSeparator) + filepath.Join("tmp", "file.txt")
	got, err := ResolvePath("", path, true, true, true, true)
	if err != nil {
		t.Fatalf("ResolvePath unrestricted returned error: %v", err)
	}
	if got != path {
		t.Fatalf("ResolvePath unrestricted = %q, want %q", got, path)
	}
}
