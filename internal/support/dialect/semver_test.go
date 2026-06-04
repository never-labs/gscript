package dialect

import "testing"

func TestParseSemVer(t *testing.T) {
	got, err := ParseSemVer("1.2.3-rc.1+build.7")
	if err != nil {
		t.Fatalf("ParseSemVer: %v", err)
	}
	if got.Major != 1 || got.Minor != 2 || got.Patch != 3 {
		t.Fatalf("core = %d.%d.%d, want 1.2.3", got.Major, got.Minor, got.Patch)
	}
	if len(got.Prerelease) != 2 || got.Prerelease[0] != "rc" || got.Prerelease[1] != "1" {
		t.Fatalf("prerelease = %#v, want rc.1", got.Prerelease)
	}
	if len(got.Build) != 2 || got.Build[0] != "build" || got.Build[1] != "7" {
		t.Fatalf("build = %#v, want build.7", got.Build)
	}
}

func TestFormatSemVer(t *testing.T) {
	got, err := FormatSemVer(SemVer{
		Major:      2,
		Minor:      0,
		Patch:      1,
		Prerelease: []string{"beta", "2"},
		Build:      []string{"ci", "0042"},
	})
	if err != nil {
		t.Fatalf("FormatSemVer: %v", err)
	}
	if got != "2.0.1-beta.2+ci.0042" {
		t.Fatalf("FormatSemVer = %q, want 2.0.1-beta.2+ci.0042", got)
	}
}

func TestParseSemVerRejectsInvalidVersions(t *testing.T) {
	for _, src := range []string{
		"",
		"1.2",
		"01.2.3",
		"1.02.3",
		"1.2.03",
		"1.2.3-01",
		"1.2.3-",
		"1.2.3+",
		"1.2.3-alpha..1",
		"1.2.3+build!",
	} {
		if _, err := ParseSemVer(src); err == nil {
			t.Fatalf("ParseSemVer(%q) succeeded, want error", src)
		}
	}
}
