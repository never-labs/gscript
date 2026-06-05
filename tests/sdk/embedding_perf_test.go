package leia_test

import (
	"fmt"
	"testing"

	leia "github.com/never-labs/leia"
)

func BenchmarkRegisterFuncCallFromScript(b *testing.B) {
	src := `
func run(n) {
    sum := 0
    for i := 1; i <= n; i++ {
        sum = inc(sum)
    }
    return sum
}
`

	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		b.Run(tc.name, func(b *testing.B) {
			vm := leia.New(tc.opts...)
			if err := vm.RegisterFunc("inc", func(v int64) int64 { return v + 1 }); err != nil {
				b.Fatal(err)
			}
			prog, err := leia.Compile(src, leia.WithSourceName("benchmark/register_func_call.leia"))
			if err != nil {
				b.Fatal(err)
			}
			if err := vm.Run(prog); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				results, err := vm.Call("run", int64(512))
				if err != nil {
					b.Fatal(err)
				}
				if len(results) != 1 || results[0] != int64(512) {
					b.Fatalf("run result = %v, want [512]", results)
				}
			}
		})
	}
}

func BenchmarkRegisterModuleCallFromScript(b *testing.B) {
	src := `
host := require("go/host")

func run(n) {
    sum := 0
    for i := 1; i <= n; i++ {
        sum = host.bump(sum)
    }
    return host.label(sum)
}
`

	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		b.Run(tc.name, func(b *testing.B) {
			vm := leia.New(tc.opts...)
			if err := vm.RegisterModule("go/host", leia.Module{
				"bump": func(v int64) int64 { return v + 1 },
				"label": func(v int64) string {
					return fmt.Sprintf("host-%03d", v)
				},
			}); err != nil {
				b.Fatal(err)
			}
			prog, err := leia.Compile(src, leia.WithSourceName("benchmark/register_module_call.leia"))
			if err != nil {
				b.Fatal(err)
			}
			if err := vm.Run(prog); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				results, err := vm.Call("run", int64(256))
				if err != nil {
					b.Fatal(err)
				}
				if len(results) != 1 || results[0] != "host-256" {
					b.Fatalf("run result = %v, want [host-256]", results)
				}
			}
		})
	}
}
