package runtime

import "testing"

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

	execBinaryIOTest(t, interp, `
		closed := false
		obj := {}
		obj.close = func(self) {
			closed = true
		}
		func writeThenFail() {
			defer obj:close()
			error("boom")
		}
		ok, err := pcall(writeThenFail)
	`)

	if interp.GetGlobal("ok").Truthy() {
		t.Fatalf("writeThenFail should fail")
	}
	if got := interp.GetGlobal("err").Str(); got != "boom" {
		t.Fatalf("err = %q, want boom", got)
	}
	if !interp.GetGlobal("closed").Truthy() {
		t.Fatalf("deferred method did not run")
	}
}

func TestDeferRunsAtTopLevelScriptExit(t *testing.T) {
	interp := NewCore()

	execBinaryIOTest(t, interp, `
		top := ""
		func record(s) {
			top = top .. s
		}
		defer record("done")
	`)

	if got := interp.GetGlobal("top").Str(); got != "done" {
		t.Fatalf("top = %q, want done", got)
	}
}
