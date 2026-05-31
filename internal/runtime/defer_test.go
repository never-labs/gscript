package runtime

import (
	"path/filepath"
	"testing"
)

func TestDeferRunsFunctionScopeLIFO(t *testing.T) {
	interp := NewCore()

	execBinaryIOTest(t, interp, `
		order := ""
		func record(s) {
			order = order .. s
		}
		func work() {
			x := "one"
			defer record(x)
			x = "two"
			defer record(x)
			order = order .. "body"
			return 7
		}
		result := work()
	`)

	if got := interp.GetGlobal("result").Int(); got != 7 {
		t.Fatalf("result = %d, want 7", got)
	}
	if got := interp.GetGlobal("order").Str(); got != "bodytwoone" {
		t.Fatalf("order = %q, want bodytwoone", got)
	}
}

func TestDeferRunsOnErrorAndSupportsMethods(t *testing.T) {
	interp := NewCore()
	ioModule := TableValue(buildIOLib(interp))
	interp.SetGlobal("io", ioModule)
	interp.SetModule("io", ioModule)
	path := filepath.Join(t.TempDir(), "defer.txt")
	interp.SetGlobal("file", StringValue(path))

	execBinaryIOTest(t, interp, `
		func writeThenFail() {
			f := io.open(file, "w+")
			defer f:close()
			assert(f:write("closed"))
			error("boom")
		}
		ok, err := pcall(writeThenFail)
		f := io.open(file, "r")
		text := f:read("a")
		f:close()
	`)

	if interp.GetGlobal("ok").Truthy() {
		t.Fatalf("writeThenFail should fail")
	}
	if got := interp.GetGlobal("err").Str(); got != "boom" {
		t.Fatalf("err = %q, want boom", got)
	}
	if got := interp.GetGlobal("text").Str(); got != "closed" {
		t.Fatalf("text = %q, want closed", got)
	}
}

func TestDeferRunsAtTopLevelScriptExit(t *testing.T) {
	interp := NewCore()
	ioModule := TableValue(buildIOLib(interp))
	interp.SetGlobal("io", ioModule)
	interp.SetModule("io", ioModule)
	path := filepath.Join(t.TempDir(), "top.txt")
	interp.SetGlobal("file", StringValue(path))

	execBinaryIOTest(t, interp, `
		f := io.open(file, "w+")
		defer f:close()
		assert(f:write("top"))
	`)

	check := NewCore()
	checkIOModule := TableValue(buildIOLib(check))
	check.SetGlobal("io", checkIOModule)
	check.SetModule("io", checkIOModule)
	check.SetGlobal("file", StringValue(path))
	execBinaryIOTest(t, check, `
		f := io.open(file, "r")
		text := f:read("a")
		f:close()
	`)
	if got := check.GetGlobal("text").Str(); got != "top" {
		t.Fatalf("text = %q, want top", got)
	}
}
