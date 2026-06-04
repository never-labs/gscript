package leia_test

import (
	"fmt"
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
)

func TestWithDialectRegistersHostDialectAcrossExecutionEngines(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := append([]leia.Option{
				leia.WithDialect("wrap", func(body leia.Value, options leia.Value) ([]leia.Value, error) {
					prefix := "<"
					suffix := ">"
					if encoded, err := options.Encode(); err == nil {
						if table, ok := encoded.(map[string]any); ok {
							if v, ok := table["prefix"].(string); ok {
								prefix = v
							}
							if v, ok := table["suffix"].(string); ok {
								suffix = v
							}
						}
					}
					return []leia.Value{leia.String(prefix + body.String() + suffix)}, nil
				}, leia.DialectOptions{
					Aliases:      []string{"bracket"},
					Category:     "text",
					Capabilities: []string{"text.wrap"},
					Block: func(body leia.Value, options leia.Value) ([]leia.Value, error) {
						encoded, err := body.Encode()
						if err != nil {
							return nil, err
						}
						table, ok := encoded.(map[string]any)
						if !ok {
							return nil, fmt.Errorf("block body is %T, want table", encoded)
						}
						return []leia.Value{leia.MustDecode(map[string]any{
							"kind":  "block",
							"title": table["title"],
						})}, nil
					},
				}),
			}, tc.opts...)
			vm := leia.New(opts...)
			err := vm.Exec(`
literal := wrap` + "`ok`" + `
alias := bracket` + "`ok`" + `
explicit := dialect.eval("wrap", "ok", {prefix: "[", suffix: "]"})
block := wrap { title: "Plan" }
info := dialect.info("wrap")
alias_info := dialect.info("bracket")
tags := dialect.tags()
block_kind := block.kind
block_title := block.title
info_category := info.category
info_builtin := info.builtin
info_capability := info.capabilities[1]
info_alias := info.aliases[1]
alias_info_alias := alias_info.aliases[1]
`)
			if err != nil {
				t.Fatal(err)
			}
			assertSDKGlobal(t, vm, "literal", "<ok>")
			assertSDKGlobal(t, vm, "alias", "<ok>")
			assertSDKGlobal(t, vm, "explicit", "[ok]")
			assertSDKGlobal(t, vm, "block_kind", "block")
			assertSDKGlobal(t, vm, "block_title", "Plan")
			assertSDKGlobal(t, vm, "info_category", "text")
			assertSDKGlobal(t, vm, "info_builtin", false)
			assertSDKGlobal(t, vm, "info_capability", "text.wrap")
			assertSDKGlobal(t, vm, "info_alias", "bracket")
			assertSDKGlobal(t, vm, "alias_info_alias", "wrap")
		})
	}
}

func TestWithDialectRejectsBuiltinOverride(t *testing.T) {
	for _, opts := range [][]leia.Option{
		nil,
		{leia.WithVM()},
	} {
		func() {
			defer func() {
				recovered := recover()
				if recovered == nil {
					t.Fatalf("WithDialect json override did not panic for opts %#v", opts)
				}
				if got := fmt.Sprint(recovered); !strings.Contains(got, `duplicate dialect "json"`) {
					t.Fatalf("panic = %q, want duplicate json", got)
				}
			}()
			vm := leia.New(append([]leia.Option{
				leia.WithDialect("json", func(body leia.Value, options leia.Value) ([]leia.Value, error) {
					return []leia.Value{body}, nil
				}),
			}, opts...)...)
			_ = vm.Exec(`_ = dialect.tags()`)
		}()
	}
}

func assertSDKGlobal(t *testing.T, vm *leia.VM, name string, want any) {
	t.Helper()
	got, err := vm.Get(name)
	if err != nil {
		t.Fatalf("Get(%q): %v", name, err)
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("Get(%q) = %v (%T), want %v (%T)", name, got, got, want, want)
	}
}
