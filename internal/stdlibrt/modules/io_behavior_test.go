package modules

import (
	"path/filepath"
	"testing"

	"github.com/never-labs/gscript/internal/lexer"
	"github.com/never-labs/gscript/internal/parser"
	"github.com/never-labs/gscript/internal/stdlibrt/host"
)

func execBinaryIOTest(t *testing.T, interp *Interpreter, src string) {
	t.Helper()
	tokens, err := lexer.New(src).Tokenize()
	if err != nil {
		t.Fatalf("lexer error: %v", err)
	}
	prog, err := parser.New(tokens).Parse()
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if err := interp.Exec(prog); err != nil {
		t.Fatalf("exec error: %v", err)
	}
}

func TestIOFileHandleSeekFlushTypeAndClose(t *testing.T) {
	interp := New()
	ioModule := TableValue(BuildIO(host.Options{}))
	interp.SetGlobal("io", ioModule)
	interp.SetModule("io", ioModule)
	path := filepath.Join(t.TempDir(), "seek.txt")
	interp.SetGlobal("file", StringValue(path))

	execBinaryIOTest(t, interp, `
		f := io.open(file, "w+")
		beforeType := io.type(f)
		assert(f:write("abcdef"))
		assert(f:flush())
		pos := f:seek("set", 2)
		chunk := f:read(3)
		endPos := f:seek("end", 0)
		assert(f:close())
		afterType := io.type(f)
		again, againErr := f:close()
	`)

	if got := interp.GetGlobal("beforeType").Str(); got != "file" {
		t.Fatalf("beforeType = %q, want file", got)
	}
	if got := interp.GetGlobal("pos").Int(); got != 2 {
		t.Fatalf("pos = %d, want 2", got)
	}
	if got := interp.GetGlobal("chunk").Str(); got != "cde" {
		t.Fatalf("chunk = %q, want cde", got)
	}
	if got := interp.GetGlobal("endPos").Int(); got != 6 {
		t.Fatalf("endPos = %d, want 6", got)
	}
	if got := interp.GetGlobal("afterType").Str(); got != "closed file" {
		t.Fatalf("afterType = %q, want closed file", got)
	}
	if !interp.GetGlobal("again").IsNil() {
		t.Fatalf("again = %v, want nil", interp.GetGlobal("again"))
	}
	if !interp.GetGlobal("againErr").IsString() {
		t.Fatalf("againErr = %v, want string", interp.GetGlobal("againErr"))
	}
}

func TestIOInputOutputAndTmpfile(t *testing.T) {
	interp := New()
	ioModule := TableValue(BuildIO(host.Options{}))
	interp.SetGlobal("io", ioModule)
	interp.SetModule("io", ioModule)
	path := filepath.Join(t.TempDir(), "redirect.txt")
	interp.SetGlobal("file", StringValue(path))

	execBinaryIOTest(t, interp, `
		oldOut := io.output()
		out := io.output(file)
		io.write("hello", "\n", "world")
		assert(io.flush())
		io.output(oldOut)
		assert(out:close())

		oldIn := io.input()
		inp := io.input(file)
		all := io.read("a")
		io.input(oldIn)
		assert(inp:close())

		tmp := io.tmpfile()
		assert(io.type(tmp) == "file")
		assert(tmp:write("xy"))
		assert(tmp:seek("set", 0) == 0)
		tmpData := tmp:read(2)
		assert(tmp:close())
		tmpType := io.type(tmp)
	`)

	if got := interp.GetGlobal("all").Str(); got != "hello\nworld" {
		t.Fatalf("all = %q, want redirected file contents", got)
	}
	if got := interp.GetGlobal("tmpData").Str(); got != "xy" {
		t.Fatalf("tmpData = %q, want xy", got)
	}
	if got := interp.GetGlobal("tmpType").Str(); got != "closed file" {
		t.Fatalf("tmpType = %q, want closed file", got)
	}
}

func TestIOReadFormatsLineWithNewlineBytesAndMultipleResults(t *testing.T) {
	interp := New()
	ioModule := TableValue(BuildIO(host.Options{}))
	interp.SetGlobal("io", ioModule)
	interp.SetModule("io", ioModule)
	path := filepath.Join(t.TempDir(), "read-formats.txt")
	interp.SetGlobal("file", StringValue(path))

	execBinaryIOTest(t, interp, `
		f := io.open(file, "w")
		assert(f:write("first\nsecond\nrest!"))
		assert(f:close())

		f = io.open(file, "r")
		lineWithEnd, lineWithoutEnd, chunk := f:read("L", "l", 4)
		empty := f:read(0)
		tail := f:read(100)
		atEOF := f:read(1)
		assert(f:close())
	`)

	if got := interp.GetGlobal("lineWithEnd").Str(); got != "first\n" {
		t.Fatalf("lineWithEnd = %q, want first with newline", got)
	}
	if got := interp.GetGlobal("lineWithoutEnd").Str(); got != "second" {
		t.Fatalf("lineWithoutEnd = %q, want second", got)
	}
	if got := interp.GetGlobal("chunk").Str(); got != "rest" {
		t.Fatalf("chunk = %q, want rest", got)
	}
	if got := interp.GetGlobal("empty").Str(); got != "" {
		t.Fatalf("empty = %q, want empty string", got)
	}
	if got := interp.GetGlobal("tail").Str(); got != "!" {
		t.Fatalf("tail = %q, want !", got)
	}
	if !interp.GetGlobal("atEOF").IsNil() {
		t.Fatalf("atEOF = %v, want nil", interp.GetGlobal("atEOF"))
	}
}
