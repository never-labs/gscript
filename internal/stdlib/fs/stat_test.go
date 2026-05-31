package fs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProjectFileInfo(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(path, []byte("hello"), 0640); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	got := ProjectFileInfo(info)
	if got.Name != "file.txt" {
		t.Fatalf("Name = %q, want file.txt", got.Name)
	}
	if got.Size != 5 {
		t.Fatalf("Size = %d, want 5", got.Size)
	}
	if got.MTime != info.ModTime().Unix() {
		t.Fatalf("MTime = %d, want %d", got.MTime, info.ModTime().Unix())
	}
	if got.IsDir {
		t.Fatalf("IsDir = true, want false")
	}
	if !got.IsFile() {
		t.Fatalf("IsFile = false, want true")
	}
	if got.Mode != FormatPerm(info.Mode()) {
		t.Fatalf("Mode = %q, want %q", got.Mode, FormatPerm(info.Mode()))
	}
}

func TestFormatPerm(t *testing.T) {
	if got := FormatPerm(0644); got != "0644" {
		t.Fatalf("FormatPerm(0644) = %q, want 0644", got)
	}
	if got := FormatPerm(os.ModeDir | 0755); got != "0755" {
		t.Fatalf("FormatPerm(dir 0755) = %q, want 0755", got)
	}
}
