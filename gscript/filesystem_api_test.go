package gscript_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	gs "github.com/never-labs/gscript/gscript"
	"github.com/never-labs/gscript/internal/runtime"
)

func TestWithFilesystemFalseRemovesFilesystemGlobals(t *testing.T) {
	vm := gs.New(gs.WithFilesystem(false))
	for _, name := range []string{"fs", "dofile", "loadfile"} {
		got, err := vm.Get(name)
		if err != nil {
			t.Fatal(err)
		}
		if got != nil {
			t.Fatalf("%s = %v, want nil", name, got)
		}
	}
}

func TestWithFilesystemFalseClearsRootEnabledFilesystem(t *testing.T) {
	root := t.TempDir()
	vm := gs.New(gs.WithFilesystemRoot(root), gs.WithFilesystem(false))
	for _, name := range []string{"fs", "dofile", "loadfile"} {
		got, err := vm.Get(name)
		if err != nil {
			t.Fatal(err)
		}
		if got != nil {
			t.Fatalf("%s = %v, want nil", name, got)
		}
	}
}

func TestWithFilesystemWriteFalseBlocksOSFileMutation(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := append([]gs.Option{
				gs.WithLibs(gs.LibString | gs.LibOS),
				gs.WithFilesystemWrite(false),
			}, tc.opts...)
			vm := gs.New(opts...)
			for _, src := range []string{
				`ok := os.remove("blocked.txt")`,
				`ok := os.rename("old.txt", "new.txt")`,
				`name := os.tmpname()`,
			} {
				err := vm.Exec(src)
				if err == nil || !strings.Contains(err.Error(), "filesystem write access disabled") {
					t.Fatalf("%s err = %v, want filesystem write access disabled", src, err)
				}
			}
		})
	}
}

func TestWithFilesystemCapabilitiesGateIOLibFiles(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "in.txt"), []byte("hello"), 0644); err != nil {
				t.Fatal(err)
			}
			readOnly := append([]gs.Option{
				gs.WithLibs(gs.LibString | gs.LibIO),
				gs.WithFilesystemRoot(root),
				gs.WithFilesystemWrite(false),
			}, tc.opts...)
			vm := gs.New(readOnly...)
			if err := vm.Exec(`f := io.open("in.txt", "r"); data := f:read("a"); f:close()`); err != nil {
				t.Fatalf("io.open read in read-only filesystem failed: %v", err)
			}
			got, err := vm.Get("data")
			if err != nil {
				t.Fatal(err)
			}
			if got != "hello" {
				t.Fatalf("data = %v, want hello", got)
			}
			for _, src := range []string{
				`f := io.open("out.txt", "w")`,
				`io.output("out.txt")`,
				`tmp := io.tmpfile()`,
			} {
				err := vm.Exec(src)
				if err == nil || !strings.Contains(err.Error(), "filesystem write access disabled") {
					t.Fatalf("%s err = %v, want filesystem write access disabled", src, err)
				}
			}

			writeOnly := append([]gs.Option{
				gs.WithLibs(gs.LibString | gs.LibIO),
				gs.WithFilesystemRoot(root),
				gs.WithFilesystemRead(false),
			}, tc.opts...)
			vm = gs.New(writeOnly...)
			if err := vm.Exec(`f := io.open("out.txt", "w"); f:write("ok"); f:close()`); err != nil {
				t.Fatalf("io.open write in write-only filesystem failed: %v", err)
			}
			for _, src := range []string{
				`f := io.open("in.txt", "r")`,
				`iter := io.lines("in.txt")`,
				`io.input("in.txt")`,
			} {
				err := vm.Exec(src)
				if err == nil || !strings.Contains(err.Error(), "filesystem read access disabled") {
					t.Fatalf("%s err = %v, want filesystem read access disabled", src, err)
				}
			}
		})
	}
}

func TestWithFilesystemRootConfinesIOLibFiles(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			opts := append([]gs.Option{
				gs.WithLibs(gs.LibString | gs.LibIO),
				gs.WithFilesystemRoot(root),
			}, tc.opts...)
			vm := gs.New(opts...)
			if err := vm.Exec(`
				f := io.open("inside.txt", "w")
				assert(f:write("ok"))
				assert(f:close())
			`); err != nil {
				t.Fatalf("io.open inside root failed: %v", err)
			}
			if got, err := os.ReadFile(filepath.Join(root, "inside.txt")); err != nil || string(got) != "ok" {
				t.Fatalf("inside file = %q err=%v, want ok", got, err)
			}
			err := vm.Exec(`f := io.open("../escape.txt", "w")`)
			if err == nil || !strings.Contains(err.Error(), "filesystem access denied") {
				t.Fatalf("io.open escape err = %v, want filesystem access denied", err)
			}
			if err := vm.Exec(`
				tmp := io.tmpfile()
				assert(tmp:write("x"))
				assert(tmp:close())
			`); err != nil {
				t.Fatalf("io.tmpfile in root failed: %v", err)
			}
		})
	}
}

func TestWithFilesystemRootConfinesOSFileMutation(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "old.txt"), []byte("ok"), 0644); err != nil {
				t.Fatal(err)
			}
			opts := append([]gs.Option{
				gs.WithLibs(gs.LibString | gs.LibOS),
				gs.WithFilesystemRoot(root),
			}, tc.opts...)
			vm := gs.New(opts...)
			if err := vm.Exec(`ok := os.rename("old.txt", "new.txt")`); err != nil {
				t.Fatalf("os.rename inside root failed: %v", err)
			}
			if _, err := os.Stat(filepath.Join(root, "new.txt")); err != nil {
				t.Fatalf("renamed file missing: %v", err)
			}
			err := vm.Exec(`ok := os.remove("../escape.txt")`)
			if err == nil || !strings.Contains(err.Error(), "filesystem access denied") {
				t.Fatalf("os.remove escape err = %v, want filesystem access denied", err)
			}
			if err := vm.Exec(`name := os.tmpname()`); err != nil {
				t.Fatalf("os.tmpname in root failed: %v", err)
			}
			got, err := vm.Get("name")
			if err != nil {
				t.Fatal(err)
			}
			name, ok := got.(string)
			if !ok || !strings.HasPrefix(name, root+string(os.PathSeparator)) {
				t.Fatalf("tmpname = %v, want path inside %s", got, root)
			}
			_ = os.Remove(name)
		})
	}
}

func TestWithFilesystemReadOnlyAllowsReadAndBlocksWrite(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "inside.txt"), []byte("inside"), 0644); err != nil {
		t.Fatal(err)
	}

	vm := gs.New(gs.WithLibs(gs.LibString|gs.LibFS), gs.WithFilesystemRoot(root), gs.WithFilesystemWrite(false))
	if err := vm.Exec(`content := fs.readfile("inside.txt")`); err != nil {
		t.Fatalf("readfile with read-only filesystem failed: %v", err)
	}
	content, err := vm.Get("content")
	if err != nil {
		t.Fatal(err)
	}
	if content != "inside" {
		t.Fatalf("content = %v, want inside", content)
	}
	err = vm.Exec(`fs.writefile("new.txt", "new")`)
	if err == nil || !strings.Contains(err.Error(), "filesystem write access disabled") {
		t.Fatalf("writefile error = %v, want write access disabled", err)
	}
}

func TestWithFilesystemRootReadOnlyAllowsFileLoadsAndConfinesReads(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "root")
	if err := os.Mkdir(root, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "inside.gs"), []byte(`loaded := "inside"`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "inside.txt"), []byte("inside"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "outside.txt"), []byte("outside"), 0644); err != nil {
		t.Fatal(err)
	}

	vm := gs.New(gs.WithLibs(gs.LibString|gs.LibFS), gs.WithFilesystemRoot(root), gs.WithFilesystemWrite(false))
	if err := vm.Exec(`
		dofile("inside.gs")
		inside, insideErr := fs.readfile("inside.txt")
		outside, outsideErr := fs.readfile("../outside.txt")
	`); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]interface{}{
		"loaded":    "inside",
		"inside":    "inside",
		"insideErr": nil,
		"outside":   nil,
	} {
		got, err := vm.Get(name)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("%s = %v (%T), want %v", name, got, got, want)
		}
	}
	outsideErr, err := vm.Get("outsideErr")
	if err != nil {
		t.Fatal(err)
	}
	if msg, ok := outsideErr.(string); !ok || !strings.Contains(msg, "escapes root") {
		t.Fatalf("outsideErr = %v, want escapes root string", outsideErr)
	}
	err = vm.Exec(`fs.writefile("blocked.txt", "blocked")`)
	if err == nil || !strings.Contains(err.Error(), "filesystem write access disabled") {
		t.Fatalf("writefile error = %v, want write access disabled", err)
	}
	loadfile, err := vm.Get("loadfile")
	if err != nil {
		t.Fatal(err)
	}
	if got := publicAPIType(loadfile); got != "function" {
		t.Fatalf("loadfile type = %v, want function", got)
	}
}

func TestWithFilesystemWriteOnlyAllowsWriteAndBlocksRead(t *testing.T) {
	root := t.TempDir()
	vm := gs.New(gs.WithLibs(gs.LibString|gs.LibFS), gs.WithFilesystemRoot(root), gs.WithFilesystemRead(false))

	if err := vm.Exec(`ok := fs.writefile("out.txt", "out")`); err != nil {
		t.Fatalf("writefile with write-only filesystem failed: %v", err)
	}
	if got, err := os.ReadFile(filepath.Join(root, "out.txt")); err != nil || string(got) != "out" {
		t.Fatalf("host file = %q, %v; want out, nil", string(got), err)
	}
	for _, name := range []string{"dofile", "loadfile"} {
		got, err := vm.Get(name)
		if err != nil {
			t.Fatal(err)
		}
		if got != nil {
			t.Fatalf("%s = %v, want nil", name, got)
		}
	}
	err := vm.Exec(`fs.readfile("out.txt")`)
	if err == nil || !strings.Contains(err.Error(), "filesystem read access disabled") {
		t.Fatalf("readfile error = %v, want read access disabled", err)
	}
}

func TestWithFilesystemRootWriteOnlyConfinesWritesAndRemovesFileLoads(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "root")
	if err := os.Mkdir(root, 0755); err != nil {
		t.Fatal(err)
	}

	vm := gs.New(gs.WithLibs(gs.LibString|gs.LibFS), gs.WithFilesystemRoot(root), gs.WithFilesystemRead(false))
	if err := vm.Exec(`
		insideOK, insideErr := fs.writefile("inside.txt", "inside")
		outsideOK, outsideErr := fs.writefile("../outside.txt", "outside")
	`); err != nil {
		t.Fatal(err)
	}
	insideOK, err := vm.Get("insideOK")
	if err != nil {
		t.Fatal(err)
	}
	if insideOK != true {
		t.Fatalf("insideOK = %v, want true", insideOK)
	}
	insideErr, err := vm.Get("insideErr")
	if err != nil {
		t.Fatal(err)
	}
	if insideErr != nil {
		t.Fatalf("insideErr = %v, want nil", insideErr)
	}
	if got, err := os.ReadFile(filepath.Join(root, "inside.txt")); err != nil || string(got) != "inside" {
		t.Fatalf("host file = %q, %v; want inside, nil", string(got), err)
	}
	if _, err := os.Stat(filepath.Join(base, "outside.txt")); !os.IsNotExist(err) {
		t.Fatalf("outside file stat err = %v, want not exist", err)
	}
	outsideOK, err := vm.Get("outsideOK")
	if err != nil {
		t.Fatal(err)
	}
	if outsideOK != nil {
		t.Fatalf("outsideOK = %v, want nil", outsideOK)
	}
	outsideErr, err := vm.Get("outsideErr")
	if err != nil {
		t.Fatal(err)
	}
	if msg, ok := outsideErr.(string); !ok || !strings.Contains(msg, "escapes root") {
		t.Fatalf("outsideErr = %v, want escapes root string", outsideErr)
	}
	for _, name := range []string{"dofile", "loadfile"} {
		got, err := vm.Get(name)
		if err != nil {
			t.Fatal(err)
		}
		if got != nil {
			t.Fatalf("%s = %v, want nil", name, got)
		}
	}
}

func TestFilesystemReadCapabilityControlsBytecodeFileLoadGlobals(t *testing.T) {
	tests := []struct {
		name      string
		opts      []gs.Option
		wantFS    string
		wantFiles string
	}{
		{
			name:      "filesystem disabled",
			opts:      []gs.Option{gs.WithFilesystem(false)},
			wantFS:    "nil",
			wantFiles: "nil",
		},
		{
			name:      "read only",
			opts:      []gs.Option{gs.WithFilesystemWrite(false)},
			wantFS:    "table",
			wantFiles: "function",
		},
		{
			name:      "write only",
			opts:      []gs.Option{gs.WithFilesystemRead(false)},
			wantFS:    "table",
			wantFiles: "nil",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			opts := append([]gs.Option{gs.WithVM()}, tc.opts...)
			vm := gs.New(opts...)
			if err := vm.Exec(`probe := true`); err != nil {
				t.Fatal(err)
			}
			gotFS, err := vm.Get("fs")
			if err != nil {
				t.Fatal(err)
			}
			if got := publicAPIType(gotFS); got != tc.wantFS {
				t.Fatalf("fs type = %v, want %s", got, tc.wantFS)
			}
			for _, name := range []string{"dofile", "loadfile"} {
				got, err := vm.Get(name)
				if err != nil {
					t.Fatal(err)
				}
				if gotType := publicAPIType(got); gotType != tc.wantFiles {
					t.Fatalf("%s type = %v, want %s", name, gotType, tc.wantFiles)
				}
			}
		})
	}
}

func TestMaxFilesystemReadBytesLimitsFSReadFile(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "big.txt"), []byte("12345"), 0644); err != nil {
				t.Fatal(err)
			}
			opts := append([]gs.Option{
				gs.WithLibs(gs.LibString | gs.LibFS),
				gs.WithFilesystemRoot(root),
				gs.WithMaxFilesystemReadBytes(4),
			}, tc.opts...)
			vm := gs.New(opts...)
			if err := vm.Exec(`content, readErr := fs.readfile("big.txt")`); err != nil {
				t.Fatal(err)
			}
			content, err := vm.Get("content")
			if err != nil {
				t.Fatal(err)
			}
			if content != nil {
				t.Fatalf("content = %v, want nil", content)
			}
			readErr, err := vm.Get("readErr")
			if err != nil {
				t.Fatal(err)
			}
			if msg, ok := readErr.(string); !ok || !strings.Contains(msg, "filesystem read byte limit exceeded (4)") {
				t.Fatalf("readErr = %v, want read byte budget string", readErr)
			}
		})
	}
}

func TestMaxFilesystemWriteBytesLimitsFSWriteFile(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			opts := append([]gs.Option{
				gs.WithLibs(gs.LibString | gs.LibFS),
				gs.WithFilesystemRoot(root),
				gs.WithMaxFilesystemWriteBytes(4),
			}, tc.opts...)
			vm := gs.New(opts...)
			if err := vm.Exec(`ok, writeErr := fs.writefile("big.txt", "12345")`); err != nil {
				t.Fatal(err)
			}
			ok, err := vm.Get("ok")
			if err != nil {
				t.Fatal(err)
			}
			if ok != nil {
				t.Fatalf("ok = %v, want nil", ok)
			}
			writeErr, err := vm.Get("writeErr")
			if err != nil {
				t.Fatal(err)
			}
			if msg, ok := writeErr.(string); !ok || !strings.Contains(msg, "filesystem write byte limit exceeded (4)") {
				t.Fatalf("writeErr = %v, want write byte budget string", writeErr)
			}
			if _, err := os.Stat(filepath.Join(root, "big.txt")); !os.IsNotExist(err) {
				t.Fatalf("big.txt stat err = %v, want not exist", err)
			}
		})
	}
}

func publicAPIType(v interface{}) string {
	if v == nil {
		return "nil"
	}
	if val, ok := v.(runtime.Value); ok && val.IsFunction() {
		return "function"
	}
	if _, ok := v.(map[string]interface{}); ok {
		return "table"
	}
	return fmt.Sprintf("%T", v)
}

func TestWithFilesystemRootConfinesFSModule(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "root")
	if err := os.Mkdir(root, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "inside.txt"), []byte("inside"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "outside.txt"), []byte("outside"), 0644); err != nil {
		t.Fatal(err)
	}

	vm := gs.New(gs.WithLibs(gs.LibString|gs.LibFS), gs.WithFilesystemRoot(root))
	if err := vm.Exec(`
		inside, insideErr := fs.readfile("inside.txt")
		outside, outsideErr := fs.readfile("../outside.txt")
	`); err != nil {
		t.Fatal(err)
	}
	inside, err := vm.Get("inside")
	if err != nil {
		t.Fatal(err)
	}
	if inside != "inside" {
		t.Fatalf("inside = %v, want inside", inside)
	}
	outside, err := vm.Get("outside")
	if err != nil {
		t.Fatal(err)
	}
	if outside != nil {
		t.Fatalf("outside = %v, want nil", outside)
	}
	outsideErr, err := vm.Get("outsideErr")
	if err != nil {
		t.Fatal(err)
	}
	if msg, ok := outsideErr.(string); !ok || !strings.Contains(msg, "escapes root") {
		t.Fatalf("outsideErr = %v, want escapes root string", outsideErr)
	}
}

func TestWithFilesystemRootConfinesBytecodeRequire(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "root")
	if err := os.Mkdir(root, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "outside.gs"), []byte(`return { value: 99 }`), 0644); err != nil {
		t.Fatal(err)
	}
	vm := gs.New(gs.WithRequirePath(root), gs.WithFilesystemRoot(root), gs.WithVM())
	err := vm.Exec(`require("../outside")`)
	if err == nil || !strings.Contains(err.Error(), "escapes root") {
		t.Fatalf("require outside error = %v, want escapes root", err)
	}
}
