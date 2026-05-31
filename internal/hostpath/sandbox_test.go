package hostpath

import (
	"path/filepath"
	"testing"
)

func TestResolveSandboxPath(t *testing.T) {
	root := t.TempDir()
	got, err := ResolveSandboxPath(root, "sub/file.txt")
	if err != nil {
		t.Fatalf("ResolveSandboxPath returned error: %v", err)
	}
	want := filepath.Join(root, "sub", "file.txt")
	if got != want {
		t.Fatalf("ResolveSandboxPath = %q, want %q", got, want)
	}
}

func TestResolveSandboxPathAllowsRoot(t *testing.T) {
	root := t.TempDir()
	got, err := ResolveSandboxPath(root, ".")
	if err != nil {
		t.Fatalf("ResolveSandboxPath root returned error: %v", err)
	}
	if got != root {
		t.Fatalf("ResolveSandboxPath root = %q, want %q", got, root)
	}
}

func TestResolveSandboxPathRejectsEscape(t *testing.T) {
	root := t.TempDir()
	if _, err := ResolveSandboxPath(root, "../outside.txt"); err == nil {
		t.Fatalf("ResolveSandboxPath accepted path escape")
	}
}

func TestResolveSandboxPathUnrestricted(t *testing.T) {
	got, err := ResolveSandboxPath("", "relative.txt")
	if err != nil {
		t.Fatalf("unrestricted ResolveSandboxPath returned error: %v", err)
	}
	if got != "relative.txt" {
		t.Fatalf("unrestricted ResolveSandboxPath = %q", got)
	}
}
