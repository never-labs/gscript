package modfile

import (
	"strings"
	"testing"
)

func TestParseFormatModuleFile(t *testing.T) {
	f, diags := Parse("leia.mod", strings.NewReader(`
module example.com/app
leia 0.1
go 1.25.7
go require github.com/gen2brain/raylib-go/raylib v0.55.1
go replace github.com/never-labs/leia => ../leia

capability net.client
cap fs.read,tool.exec
require example.com/lib v0.2.0
replace example.com/lib => ../lib
collection vendor ./vendor
`))
	if len(diags) != 0 {
		t.Fatalf("Parse diagnostics = %#v", diags)
	}
	if f.Module != "example.com/app" || f.Leia != "0.1" {
		t.Fatalf("file = %#v", f)
	}
	if f.Go != "1.25.7" || len(f.GoRequire) != 1 || f.GoRequire[0].Path != "github.com/gen2brain/raylib-go/raylib" {
		t.Fatalf("go metadata = %#v", f)
	}
	if len(f.GoReplace) != 1 || f.GoReplace[0].NewPath != "../leia" {
		t.Fatalf("go replaces = %#v", f.GoReplace)
	}
	if len(f.Require) != 1 || f.Require[0].Path != "example.com/lib" || f.Require[0].Version != "v0.2.0" {
		t.Fatalf("requires = %#v", f.Require)
	}
	if !containsCapability(f.Capability, "fs.read") || !containsCapability(f.Capability, "net.client") || !containsCapability(f.Capability, "tool.exec") {
		t.Fatalf("capabilities = %#v", f.Capability)
	}
	if len(f.Replace) != 1 || f.Replace[0].NewPath != "../lib" {
		t.Fatalf("replace = %#v", f.Replace)
	}
	if len(f.Collections) != 1 || f.Collections[0].Name != "vendor" {
		t.Fatalf("collections = %#v", f.Collections)
	}
	out := string(Format(f))
	for _, want := range []string{
		"module example.com/app\n",
		"leia 0.1\n",
		"go 1.25.7\n",
		"go require github.com/gen2brain/raylib-go/raylib v0.55.1\n",
		"go replace github.com/never-labs/leia => ../leia\n",
		"capability fs.read\n",
		"capability net.client\n",
		"capability tool.exec\n",
		"require example.com/lib v0.2.0\n",
		"replace example.com/lib => ../lib\n",
		"collection vendor ./vendor\n",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("Format missing %q in:\n%s", want, out)
		}
	}
}

func TestParseRejectsDuplicateGoNativeDirectives(t *testing.T) {
	_, diags := Parse("leia.mod", strings.NewReader(`module example.com/app
go 1.25
go 1.26
go require example.com/native v1.0.0
go require example.com/native v1.1.0
go replace example.com/native => ./native
go replace example.com/native => ./other
`))
	for _, want := range []string{
		"go declared more than once",
		"duplicate go require for example.com/native",
		"duplicate go replace for example.com/native",
	} {
		found := false
		for _, diag := range diags {
			if strings.Contains(diag.Message, want) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("Parse diagnostics = %#v, want %q", diags, want)
		}
	}
}

func TestParseCapabilityDirectiveRejectsInvalidNames(t *testing.T) {
	_, diags := Parse("leia.mod", strings.NewReader(`module example.com/app
capability fs.read "bad"
cap
`))
	if len(diags) != 2 {
		t.Fatalf("diagnostics = %#v, want 2", diags)
	}
}

func TestParseRejectsUnknownAndDuplicateDirectives(t *testing.T) {
	_, diags := Parse("leia.mod", strings.NewReader(`
module example.com/app
module example.com/again
require example.com/lib v1.0.0
require example.com/lib v1.0.0
collection bad/name ./x
unknown thing
`))
	if len(diags) != 4 {
		t.Fatalf("diagnostics = %#v, want 4", diags)
	}
}

func TestParseRejectsDuplicateRequirePathWithDifferentVersion(t *testing.T) {
	_, diags := Parse("leia.mod", strings.NewReader(`module example.com/app
leia 0.1
require example.com/lib v1.0.0
require example.com/lib v2.0.0
`))
	if len(diags) == 0 {
		t.Fatal("Parse diagnostics = nil, want duplicate require diagnostic")
	}
	found := false
	for _, diag := range diags {
		if strings.Contains(diag.Message, "duplicate require for example.com/lib") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("Parse diagnostics = %#v, want duplicate require path diagnostic", diags)
	}
}

func TestParseRejectsDuplicateGSAndReplace(t *testing.T) {
	_, diags := Parse("leia.mod", strings.NewReader(`module example.com/app
leia 0.1
leia 0.2
replace example.com/lib => ./lib
replace example.com/lib => ./other-lib
replace example.com/lib v1.0.0 => ./lib-v1
replace example.com/lib v1.0.0 => ./other-lib-v1
`))
	for _, want := range []string{
		"leia declared more than once",
		"duplicate replace for example.com/lib",
	} {
		found := false
		for _, diag := range diags {
			if strings.Contains(diag.Message, want) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("Parse diagnostics = %#v, want %q", diags, want)
		}
	}
}

func TestValidModulePath(t *testing.T) {
	for _, path := range []string{"example.com/app", "github.com/acme/toolkit:demo"} {
		if !ValidModulePath(path) {
			t.Fatalf("ValidModulePath(%q) = false, want true", path)
		}
	}
	for _, path := range []string{"", "bad module", "../outside", "example.com\\app", "example.com/app\nrequire bad v1.0.0"} {
		if ValidModulePath(path) {
			t.Fatalf("ValidModulePath(%q) = true, want false", path)
		}
	}
}

func TestAddRequireUpdatesExistingPath(t *testing.T) {
	f := File{Module: "example.com/app"}
	var err error
	f, err = AddRequire(f, Require{Path: "example.com/lib", Version: "v0.1.0"})
	if err != nil {
		t.Fatal(err)
	}
	f, err = AddRequire(f, Require{Path: "example.com/lib", Version: "v0.2.0"})
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Require) != 1 || f.Require[0].Version != "v0.2.0" {
		t.Fatalf("requires = %#v", f.Require)
	}
}

func containsCapability(caps []string, want string) bool {
	for _, cap := range caps {
		if cap == want {
			return true
		}
	}
	return false
}
