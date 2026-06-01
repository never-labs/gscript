package source

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestFilesDiscoversGScriptFilesSorted(t *testing.T) {
	dir := t.TempDir()
	writeSourceTestFile(t, filepath.Join(dir, "b.gs"), "")
	writeSourceTestFile(t, filepath.Join(dir, "nested", "a.gs"), "")
	writeSourceTestFile(t, filepath.Join(dir, "ignore.txt"), "")

	got, err := Files(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join(dir, "b.gs"), filepath.Join(dir, "nested", "a.gs")}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Files = %#v, want %#v", got, want)
	}
}

func TestParseReportsSyntaxErrors(t *testing.T) {
	if err := Parse("bad.gs", []byte("x := ")); err == nil {
		t.Fatal("Parse error = nil, want syntax error")
	}
}

func writeSourceTestFile(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
}
