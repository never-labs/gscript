package modresolve

import (
	"path/filepath"
	"testing"
)

func TestResolveUsesCollectionReplaceAndModuleRoot(t *testing.T) {
	collections := []Collection{{Name: "vendor", Root: "/project/vendor"}}
	replaces := []Replace{{Path: "example.com/lib", Root: "/project/local/lib"}}

	tests := []struct {
		module   string
		wantKind string
		wantFile string
	}{
		{module: "vendor:pkg.util", wantKind: "collection", wantFile: filepath.Join("/project/vendor", "pkg", "util.gs")},
		{module: "example.com/lib/foo", wantKind: "replace", wantFile: filepath.Join("/project/local/lib", "foo.gs")},
		{module: "pkg.util", wantKind: "module", wantFile: filepath.Join("/project/root", "pkg", "util.gs")},
		{module: "../outside", wantKind: "module", wantFile: filepath.Join("/project", "outside.gs")},
	}
	for _, tt := range tests {
		got := Resolve(tt.module, collections, replaces, "/project/root")
		if got.Kind != tt.wantKind {
			t.Fatalf("Resolve(%q) kind = %q, want %q", tt.module, got.Kind, tt.wantKind)
		}
		if filepath.Clean(got.File) != filepath.Clean(tt.wantFile) {
			t.Fatalf("Resolve(%q) file = %q, want %q", tt.module, got.File, tt.wantFile)
		}
	}
}

func TestResolveReplaceUsesLongestPrefix(t *testing.T) {
	replaces := []Replace{
		{Path: "example.com/lib", Root: "/project/local/lib"},
		{Path: "example.com/lib/sub", Root: "/project/local/sub"},
	}

	got := Resolve("example.com/lib/sub/pkg.util", nil, replaces, "/project/root")

	if got.Kind != "replace" {
		t.Fatalf("Resolve kind = %q, want replace", got.Kind)
	}
	if got.Path != "example.com/lib/sub" {
		t.Fatalf("Resolve replace path = %q, want example.com/lib/sub", got.Path)
	}
	if got.Root != "/project/local/sub" {
		t.Fatalf("Resolve root = %q, want /project/local/sub", got.Root)
	}
	if got.Rel != filepath.Join("pkg", "util.gs") {
		t.Fatalf("Resolve rel = %q, want %q", got.Rel, filepath.Join("pkg", "util.gs"))
	}
	if got.File != filepath.Join("/project/local/sub", "pkg", "util.gs") {
		t.Fatalf("Resolve file = %q, want %q", got.File, filepath.Join("/project/local/sub", "pkg", "util.gs"))
	}
}

func TestResolveExactReplaceToGSFile(t *testing.T) {
	replaces := []Replace{{Path: "example.com/tool", Root: "/project/tools/tool.gs"}}

	got := Resolve("example.com/tool", nil, replaces, "/project/root")

	if got.Kind != "replace" {
		t.Fatalf("Resolve kind = %q, want replace", got.Kind)
	}
	if got.Path != "example.com/tool" {
		t.Fatalf("Resolve replace path = %q, want example.com/tool", got.Path)
	}
	if got.Root != "/project/tools" {
		t.Fatalf("Resolve root = %q, want /project/tools", got.Root)
	}
	if got.Rel != "tool.gs" {
		t.Fatalf("Resolve rel = %q, want tool.gs", got.Rel)
	}
	if got.File != "/project/tools/tool.gs" {
		t.Fatalf("Resolve file = %q, want /project/tools/tool.gs", got.File)
	}
}

func TestResolveCollectionMissFallsBackToModuleRoot(t *testing.T) {
	collections := []Collection{{Name: "vendor", Root: "/project/vendor"}}

	got := Resolve("missing:pkg.util", collections, nil, "/project/root")

	if got.Kind != "module" {
		t.Fatalf("Resolve kind = %q, want module", got.Kind)
	}
	if got.Root != "/project/root" {
		t.Fatalf("Resolve root = %q, want /project/root", got.Root)
	}
	if got.Rel != filepath.Join("missing:pkg", "util.gs") {
		t.Fatalf("Resolve rel = %q, want %q", got.Rel, filepath.Join("missing:pkg", "util.gs"))
	}
	if got.File != filepath.Join("/project/root", "missing:pkg", "util.gs") {
		t.Fatalf("Resolve file = %q, want %q", got.File, filepath.Join("/project/root", "missing:pkg", "util.gs"))
	}
}

func TestResolveRelativeRequireOutsideKeepsPathSemantics(t *testing.T) {
	got := Resolve("../outside", nil, nil, "/project/root")

	if got.Kind != "module" {
		t.Fatalf("Resolve kind = %q, want module", got.Kind)
	}
	if got.Root != "/project/root" {
		t.Fatalf("Resolve root = %q, want /project/root", got.Root)
	}
	if got.Rel != "../outside.gs" {
		t.Fatalf("Resolve rel = %q, want ../outside.gs", got.Rel)
	}
	if filepath.Clean(got.File) != filepath.Clean("/project/outside.gs") {
		t.Fatalf("Resolve file = %q, want %q", got.File, filepath.Join("/project", "outside.gs"))
	}
}
