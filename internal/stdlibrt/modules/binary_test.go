package modules

import (
	"testing"

	"github.com/never-labs/gscript/internal/runtime"
)

func newBinaryModulesInterp() *runtime.Interpreter {
	interp := runtime.NewCore()
	installTestModules(interp)
	return interp
}

func newStringPackAliasInterp() *runtime.Interpreter {
	interp := runtime.New()
	installTestModule(interp, "bytes", runtime.TableValue(BuildBytes(interp.MaxHostResultBytes)))
	return interp
}

func TestBinaryPackUnpackMixedFields(t *testing.T) {
	interp := newBinaryModulesInterp()

	execOnInterp(t, interp, `
		packed := binary.pack("be:u16 i32 f32 string bytes:3", 258, -7, 1.5, "go", "abc")
		hex := bytes.toHex(packed)
		a, b, c, s, raw, next := binary.unpack("be:u16 i32 f32 string bytes:3", packed)
		fixedSize := binary.size("be:u16 i32 f32 bytes:3")
		varSize, varErr := binary.size("string")
	`)

	if got := interp.GetGlobal("hex").Str(); got != "0102fffffff93fc0000000000002676f616263" {
		t.Fatalf("hex = %q", got)
	}
	if got := interp.GetGlobal("a").Int(); got != 258 {
		t.Fatalf("a = %d, want 258", got)
	}
	if got := interp.GetGlobal("b").Int(); got != -7 {
		t.Fatalf("b = %d, want -7", got)
	}
	if got := interp.GetGlobal("c").Float(); got != 1.5 {
		t.Fatalf("c = %v, want 1.5", got)
	}
	if got := interp.GetGlobal("s").Str(); got != "go" {
		t.Fatalf("s = %q, want go", got)
	}
	if got := interp.GetGlobal("raw").Str(); got != "abc" {
		t.Fatalf("raw = %q, want abc", got)
	}
	if got := interp.GetGlobal("next").Int(); got != 20 {
		t.Fatalf("next = %d, want 20", got)
	}
	if got := interp.GetGlobal("fixedSize").Int(); got != 13 {
		t.Fatalf("fixedSize = %d, want 13", got)
	}
	if !interp.GetGlobal("varSize").IsNil() {
		t.Fatalf("varSize = %v, want nil", interp.GetGlobal("varSize"))
	}
	if !interp.GetGlobal("varErr").IsString() {
		t.Fatalf("varErr = %v, want string", interp.GetGlobal("varErr"))
	}
}

func TestBinaryLittleEndianOffsetAndErrors(t *testing.T) {
	interp := newBinaryModulesInterp()

	execOnInterp(t, interp, `
		packed := binary.pack("le u16 u32", 513, 16909060)
		a, b, next := binary.unpack("le u16 u32", "xx" .. packed, 3)
		bad, badErr := binary.unpack("u32", "a")
	`)

	if got := interp.GetGlobal("a").Int(); got != 513 {
		t.Fatalf("a = %d, want 513", got)
	}
	if got := interp.GetGlobal("b").Int(); got != 16909060 {
		t.Fatalf("b = %d, want 16909060", got)
	}
	if got := interp.GetGlobal("next").Int(); got != 9 {
		t.Fatalf("next = %d, want 9", got)
	}
	if !interp.GetGlobal("bad").IsNil() {
		t.Fatalf("bad = %v, want nil", interp.GetGlobal("bad"))
	}
	if !interp.GetGlobal("badErr").IsString() {
		t.Fatalf("badErr = %v, want string", interp.GetGlobal("badErr"))
	}
}

func TestStringPackAliasesUseGoStyleBinaryFormats(t *testing.T) {
	interp := newStringPackAliasInterp()

	execOnInterp(t, interp, `
		packed := string.pack("be:u16 bytes:2", 258, "go")
		hex := bytes.toHex(packed)
		a, raw, next := string.unpack("be:u16 bytes:2", packed)
		fixedSize := string.packsize("be:u16 bytes:2")
		varSize, varErr := string.packsize("string")
	`)

	if got := interp.GetGlobal("hex").Str(); got != "0102676f" {
		t.Fatalf("hex = %q, want 0102676f", got)
	}
	if got := interp.GetGlobal("a").Int(); got != 258 {
		t.Fatalf("a = %d, want 258", got)
	}
	if got := interp.GetGlobal("raw").Str(); got != "go" {
		t.Fatalf("raw = %q, want go", got)
	}
	if got := interp.GetGlobal("next").Int(); got != 5 {
		t.Fatalf("next = %d, want 5", got)
	}
	if got := interp.GetGlobal("fixedSize").Int(); got != 4 {
		t.Fatalf("fixedSize = %d, want 4", got)
	}
	if !interp.GetGlobal("varSize").IsNil() {
		t.Fatalf("varSize = %v, want nil", interp.GetGlobal("varSize"))
	}
	if !interp.GetGlobal("varErr").IsString() {
		t.Fatalf("varErr = %v, want string", interp.GetGlobal("varErr"))
	}
}
