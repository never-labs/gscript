package gscript_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	gs "github.com/never-labs/gscript/gscript"
)

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
