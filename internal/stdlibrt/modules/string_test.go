package modules

import (
	"testing"

	"github.com/never-labs/leia/internal/runtime"
)

func TestStringModuleAndMethodMetatable(t *testing.T) {
	interp := runtime.NewCore()
	stringLib := BuildString(interp.CallFunction, interp.MaxHostResultBytes)
	interp.SetStringLibrary(stringLib)
	installTestModule(interp, "string", runtime.TableValue(stringLib))

	execOnInterp(t, interp, `
direct := string.upper("go")
method := "script":upper()
packed := string.pack("u16 string", 7, "ok")
a, b := string.unpack("u16 string", packed)
`)

	if got := interp.GetGlobal("direct"); !got.IsString() || got.Str() != "GO" {
		t.Fatalf("direct = %v, want GO", got)
	}
	if got := interp.GetGlobal("method"); !got.IsString() || got.Str() != "SCRIPT" {
		t.Fatalf("method = %v, want SCRIPT", got)
	}
	if got := interp.GetGlobal("a"); !got.IsInt() || got.Int() != 7 {
		t.Fatalf("a = %v, want 7", got)
	}
	if got := interp.GetGlobal("b"); !got.IsString() || got.Str() != "ok" {
		t.Fatalf("b = %v, want ok", got)
	}
}
