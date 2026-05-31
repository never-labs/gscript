package gscript_test

import (
	"fmt"
	"strings"
	"testing"

	gs "github.com/never-labs/gscript"
)

type hostModuleService struct {
	Prefix string
	count  int64
}

func (s *hostModuleService) Label(id int64) string {
	return fmt.Sprintf("%s-%03d", s.Prefix, id)
}

func (s *hostModuleService) Bump() int64 {
	s.count++
	return s.count
}

func TestRegisterModuleRequire(t *testing.T) {
	vm := gs.New(gs.WithSandbox())
	err := vm.RegisterModule("go/strings", gs.Module{
		"upper": strings.ToUpper,
		"join":  strings.Join,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := vm.Exec(`
strings := require("go/strings")
result := strings.upper("hello")
`); err != nil {
		t.Fatal(err)
	}

	val, err := vm.Get("result")
	if err != nil {
		t.Fatal(err)
	}
	if val != "HELLO" {
		t.Fatalf("result = %v, want HELLO", val)
	}
}

func TestStdlibUUIDRequirePackageLoadedIdentity(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			vm := gs.New(tc.opts...)
			if err := vm.Exec(`
mod := require("uuid")
sameGlobal := mod == uuid
sameLoaded := package.loaded["uuid"] == mod
result := type(mod.v4())
`); err != nil {
				t.Fatal(err)
			}
			if got, err := vm.Get("sameGlobal"); err != nil || got != true {
				t.Fatalf("sameGlobal = %v, %v; want true, nil", got, err)
			}
			if got, err := vm.Get("sameLoaded"); err != nil || got != true {
				t.Fatalf("sameLoaded = %v, %v; want true, nil", got, err)
			}
			if got, err := vm.Get("result"); err != nil || got != "string" {
				t.Fatalf("result = %v, %v; want string, nil", got, err)
			}
		})
	}
}

func TestWithGoImportsRequireAllowlist(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := append([]gs.Option{
				gs.WithSandbox(),
				gs.WithGoImports(map[string]any{
					"strings": gs.Module{
						"upper": strings.ToUpper,
					},
				}),
			}, tc.opts...)
			vm := gs.New(opts...)
			if err := vm.Exec(`
strings := require("go:strings")
same := package.loaded["go:strings"] == strings
result := strings.upper("go-host")
`); err != nil {
				t.Fatal(err)
			}
			if got, err := vm.Get("result"); err != nil || got != "GO-HOST" {
				t.Fatalf("result = %v, %v; want GO-HOST, nil", got, err)
			}
			if got, err := vm.Get("same"); err != nil || got != true {
				t.Fatalf("same = %v, %v; want true, nil", got, err)
			}
		})
	}
}

func TestWithGoImportsImportSyntax(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := append([]gs.Option{
				gs.WithSandbox(),
				gs.WithGoImports(map[string]any{
					"go:strings": gs.Module{
						"upper": strings.ToUpper,
					},
				}),
			}, tc.opts...)
			vm := gs.New(opts...)
			if err := vm.Exec(`
import "go:strings" as strings
same := package.loaded["go:strings"] == strings
result := strings.upper("imported")
`); err != nil {
				t.Fatal(err)
			}
			if got, err := vm.Get("result"); err != nil || got != "IMPORTED" {
				t.Fatalf("result = %v, %v; want IMPORTED, nil", got, err)
			}
			if got, err := vm.Get("same"); err != nil || got != true {
				t.Fatalf("same = %v, %v; want true, nil", got, err)
			}
		})
	}
}

func TestWithGoImportsRejectsUnauthorizedGoImport(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := append([]gs.Option{
				gs.WithGoImports(map[string]any{
					"go:strings": gs.Module{"upper": strings.ToUpper},
				}),
			}, tc.opts...)
			vm := gs.New(opts...)
			err := vm.Exec(`require("go:os")`)
			if err == nil {
				t.Fatal("expected unauthorized go import to fail")
			}
			if !strings.Contains(err.Error(), `go import "go:os" is not allowed`) {
				t.Fatalf("error = %v; want unauthorized go import error", err)
			}
		})
	}
}

func TestRegisterModuleFromService(t *testing.T) {
	vm := gs.New(gs.WithSandbox(), gs.WithModuleLoading(false))
	service := &hostModuleService{Prefix: "job"}
	if err := vm.RegisterModuleFrom("go/host", service); err != nil {
		t.Fatal(err)
	}

	if err := vm.Exec(`
host := require("go/host")
label := host.label(7)
prefix := host.prefix
first := host.bump()
second := host.bump()
`); err != nil {
		t.Fatal(err)
	}

	if got, err := vm.Get("label"); err != nil || got != "job-007" {
		t.Fatalf("label = %v, %v; want job-007, nil", got, err)
	}
	if got, err := vm.Get("prefix"); err != nil || got != "job" {
		t.Fatalf("prefix = %v, %v; want job, nil", got, err)
	}
	if got, err := vm.Get("second"); err != nil || got != int64(2) {
		t.Fatalf("second = %v, %v; want 2, nil", got, err)
	}
}

func TestRegisterModuleFromExactNames(t *testing.T) {
	vm := gs.New()
	service := &hostModuleService{Prefix: "task"}
	if err := vm.RegisterModuleFrom("go/exact", service, gs.WithModuleExactNames()); err != nil {
		t.Fatal(err)
	}

	if err := vm.Exec(`
host := require("go/exact")
result := host.Label(3)
`); err != nil {
		t.Fatal(err)
	}
	if got, err := vm.Get("result"); err != nil || got != "task-003" {
		t.Fatalf("result = %v, %v; want task-003, nil", got, err)
	}
}

func TestRegisterModuleRequireBytecodeVM(t *testing.T) {
	vm := gs.New(gs.WithVM())
	err := vm.RegisterModule("go/strings", gs.Module{
		"upper": strings.ToUpper,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := vm.Exec(`
strings := require("go/strings")
result := strings.upper("vm")
`); err != nil {
		t.Fatal(err)
	}

	val, err := vm.Get("result")
	if err != nil {
		t.Fatal(err)
	}
	if val != "VM" {
		t.Fatalf("result = %v, want VM", val)
	}
}

func TestRegisterModuleRequireBytecodeVMAfterInit(t *testing.T) {
	vm := gs.New(gs.WithVM(), gs.WithModuleLoading(false))
	if err := vm.Exec(`warmup := true`); err != nil {
		t.Fatal(err)
	}
	err := vm.RegisterModule("go/strings", gs.Module{
		"upper": strings.ToUpper,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := vm.Exec(`
strings := require("go/strings")
same := package.loaded["go/strings"] == strings
result := strings.upper("late")
`); err != nil {
		t.Fatal(err)
	}

	val, err := vm.Get("result")
	if err != nil {
		t.Fatal(err)
	}
	if val != "LATE" {
		t.Fatalf("result = %v, want LATE", val)
	}
	same, err := vm.Get("same")
	if err != nil {
		t.Fatal(err)
	}
	if same != true {
		t.Fatalf("same = %v, want true", same)
	}
}

func TestRegisterModuleRequireBytecodeVMNativeRequireAfterInit(t *testing.T) {
	vm := gs.New(gs.WithVM())
	if err := vm.Exec(`warmup := true`); err != nil {
		t.Fatal(err)
	}
	err := vm.RegisterModule("go/strings", gs.Module{
		"upper": strings.ToUpper,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := vm.Exec(`
strings := require("go/strings")
same := package.loaded["go/strings"] == strings
result := strings.upper("native")
`); err != nil {
		t.Fatal(err)
	}

	val, err := vm.Get("result")
	if err != nil {
		t.Fatal(err)
	}
	if val != "NATIVE" {
		t.Fatalf("result = %v, want NATIVE", val)
	}
	same, err := vm.Get("same")
	if err != nil {
		t.Fatal(err)
	}
	if same != true {
		t.Fatalf("same = %v, want true", same)
	}
}
