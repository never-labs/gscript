package modfile

import (
	"strings"
	"testing"
)

func TestParseFormatModuleFile(t *testing.T) {
	f, diags := Parse("gscript.mod", strings.NewReader(`
module example.com/app
gs 0.1

require example.com/lib v0.2.0
replace example.com/lib => ../lib
collection vendor ./vendor
`))
	if len(diags) != 0 {
		t.Fatalf("Parse diagnostics = %#v", diags)
	}
	if f.Module != "example.com/app" || f.GS != "0.1" {
		t.Fatalf("file = %#v", f)
	}
	if len(f.Require) != 1 || f.Require[0].Path != "example.com/lib" || f.Require[0].Version != "v0.2.0" {
		t.Fatalf("requires = %#v", f.Require)
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
		"gs 0.1\n",
		"require example.com/lib v0.2.0\n",
		"replace example.com/lib => ../lib\n",
		"collection vendor ./vendor\n",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("Format missing %q in:\n%s", want, out)
		}
	}
}

func TestParseRejectsUnknownAndDuplicateDirectives(t *testing.T) {
	_, diags := Parse("gscript.mod", strings.NewReader(`
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
	_, diags := Parse("gscript.mod", strings.NewReader(`module example.com/app
gs 0.1
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
	_, diags := Parse("gscript.mod", strings.NewReader(`module example.com/app
gs 0.1
gs 0.2
replace example.com/lib => ./lib
replace example.com/lib => ./other-lib
replace example.com/lib v1.0.0 => ./lib-v1
replace example.com/lib v1.0.0 => ./other-lib-v1
`))
	for _, want := range []string{
		"gs declared more than once",
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
