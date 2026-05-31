package path

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPathOperations(t *testing.T) {
	if got, want := Join("a", "b", "c"), filepath.Join("a", "b", "c"); got != want {
		t.Fatalf("Join = %q, want %q", got, want)
	}
	if got, want := Dir("/usr/local/bin/go"), filepath.Dir("/usr/local/bin/go"); got != want {
		t.Fatalf("Dir = %q, want %q", got, want)
	}
	if got, want := Base("/usr/local/bin/go"), filepath.Base("/usr/local/bin/go"); got != want {
		t.Fatalf("Base = %q, want %q", got, want)
	}
	if got, want := Ext("archive.tar.gz"), filepath.Ext("archive.tar.gz"); got != want {
		t.Fatalf("Ext = %q, want %q", got, want)
	}
	if got, want := Clean("/usr//local/../local/bin/./go"), filepath.Clean("/usr//local/../local/bin/./go"); got != want {
		t.Fatalf("Clean = %q, want %q", got, want)
	}
}

func TestPathQueries(t *testing.T) {
	abs, err := Abs(".")
	if err != nil {
		t.Fatalf("Abs returned error: %v", err)
	}
	if !IsAbs(abs) {
		t.Fatalf("IsAbs(%q) = false, want true", abs)
	}

	dir, file := Split("/usr/local/bin/go")
	wantDir, wantFile := filepath.Split("/usr/local/bin/go")
	if dir != wantDir || file != wantFile {
		t.Fatalf("Split = %q, %q; want %q, %q", dir, file, wantDir, wantFile)
	}

	matched, err := Match("*.go", "main.go")
	if err != nil || !matched {
		t.Fatalf("Match(\"*.go\", \"main.go\") = %v, %v; want true, nil", matched, err)
	}
	if _, err := Match("[", "main.go"); err == nil {
		t.Fatal("Match with invalid pattern returned nil error")
	}

	rel, err := Rel("/usr/local", "/usr/local/bin/go")
	if err != nil {
		t.Fatalf("Rel returned error: %v", err)
	}
	if want, _ := filepath.Rel("/usr/local", "/usr/local/bin/go"); rel != want {
		t.Fatalf("Rel = %q, want %q", rel, want)
	}
}

func TestSeparators(t *testing.T) {
	if got, want := Separator(), string(os.PathSeparator); got != want {
		t.Fatalf("Separator = %q, want %q", got, want)
	}
	if got, want := ListSeparator(), string(os.PathListSeparator); got != want {
		t.Fatalf("ListSeparator = %q, want %q", got, want)
	}
}
