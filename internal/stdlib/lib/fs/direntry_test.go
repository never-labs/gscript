package fs

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestProjectDirEntries(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0755); err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := ProjectDirEntries(entries)

	byName := map[string]DirEntry{}
	for _, entry := range got {
		byName[entry.Name] = entry
	}
	if byName["file.txt"].IsDir {
		t.Fatalf("file entry marked as directory: %#v", byName["file.txt"])
	}
	if byName["file.txt"].Size != 5 {
		t.Fatalf("file entry size = %d, want 5", byName["file.txt"].Size)
	}
	if !byName["subdir"].IsDir {
		t.Fatalf("directory entry not marked as directory: %#v", byName["subdir"])
	}
}

func TestProjectDirEntryUsesZeroSizeWhenInfoFails(t *testing.T) {
	got := ProjectDirEntry(brokenDirEntry{name: "missing"})
	if got.Name != "missing" || got.IsDir || got.Size != 0 {
		t.Fatalf("ProjectDirEntry(broken) = %#v", got)
	}
}

type brokenDirEntry struct {
	name string
}

func (e brokenDirEntry) Name() string               { return e.name }
func (e brokenDirEntry) IsDir() bool                { return false }
func (e brokenDirEntry) Type() fs.FileMode          { return 0 }
func (e brokenDirEntry) Info() (fs.FileInfo, error) { return nil, errors.New("stat failed") }
