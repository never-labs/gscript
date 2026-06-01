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
