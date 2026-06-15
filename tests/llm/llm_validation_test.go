package leia_test

import (
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
)

func TestLLMStdlibRuntimeValidation(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
		want string
	}{
		{
			name: "tool caps missing requirement",
			src: `
refund := llm.tool("refund", func(id) {
    return id, nil
}, {params: ["id"], requires: ["payments.refund"]})
ok, err := llm.check_tools({refund}, {})
got := err.kind .. "|" .. err.capability .. "|" .. err.tool
`,
			want: "capability|payments.refund|refund",
		},
		{
			name: "validate output rejects non json",
			src: `
ok, msg := llm.validate_output("not json", {name: "Ada"})
got := tostring(ok) .. "|" .. msg
`,
			want: "false|value is not valid JSON",
		},
		{
			name: "validate output reports missing field",
			src: `
ok, msg := llm.validate_output({name: "Ada"}, {name: "Ada", email: "ada@example.com"})
got := tostring(ok) .. "|" .. msg
`,
			want: `false|structured output missing field "email"`,
		},
		{
			name: "agent config must return table",
			src: `
answer := llm.agent("answer", func(q) {
    return "bad", nil
})
result, err := answer("hello")
got := err.kind .. "|" .. err.message
`,
			want: "validation|agent config function must return a table",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, mode := range []struct {
				name string
				opts []leia.Option
			}{
				{name: "interpreter"},
				{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
			} {
				t.Run(mode.name, func(t *testing.T) {
					opts := append([]leia.Option{leia.WithLibs(leia.LibString | leia.LibLLM)}, mode.opts...)
					vm := leia.New(opts...)
					if err := vm.Exec(tc.src); err != nil {
						t.Fatalf("Exec: %v", err)
					}
					got, err := vm.Get("got")
					if err != nil {
						t.Fatalf("Get got: %v", err)
					}
					if !strings.Contains(got.(string), tc.want) {
						t.Fatalf("got = %#v, want substring %q", got, tc.want)
					}
				})
			}
		})
	}
}

func TestLLMStdlibArgumentValidationErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
		want string
	}{
		{
			name: "tool requires function",
			src:  `lookup := llm.tool("lookup", "not a function")`,
			want: "bad argument to 'llm.tool'",
		},
		{
			name: "agent flow must be function",
			src: `
answer := llm.agent("answer", func(q) {
    return {user: q}, nil
}, "not a function")
`,
			want: "bad argument #3 to 'llm.agent'",
		},
		{
			name: "turn options must be table",
			src:  `result, err := llm.turn(nil)`,
			want: "bad argument #1 to 'llm.turn'",
		},
		{
			name: "run agent options must be table",
			src:  `result, err := llm.run_agent("bad")`,
			want: "bad argument #1 to 'llm.run_agent'",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, mode := range []struct {
				name string
				opts []leia.Option
			}{
				{name: "interpreter"},
				{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
			} {
				t.Run(mode.name, func(t *testing.T) {
					opts := append([]leia.Option{leia.WithLibs(leia.LibString | leia.LibLLM)}, mode.opts...)
					vm := leia.New(opts...)
					err := vm.Exec(tc.src)
					if err == nil || !strings.Contains(err.Error(), tc.want) {
						t.Fatalf("Exec error = %v, want substring %q", err, tc.want)
					}
				})
			}
		})
	}
}

func TestLLMStdlibValidationAllowsAliasOnlyModels(t *testing.T) {
	for _, mode := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(mode.name, func(t *testing.T) {
			opts := append([]leia.Option{leia.WithLibs(leia.LibString | leia.LibLLM)}, mode.opts...)
			vm := leia.New(opts...)
			err := vm.Exec(`
llm.register_models({
    default: "fast"
    fast: "host-fast"
    host: {provider_model: "host-only"}
})
`)
			if err != nil {
				t.Fatalf("Exec: %v", err)
			}
		})
	}
}

func TestLLMStdlibValidationRejectsModelAliasCycles(t *testing.T) {
	for _, mode := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(mode.name, func(t *testing.T) {
			opts := append([]leia.Option{leia.WithLibs(leia.LibString | leia.LibLLM)}, mode.opts...)
			vm := leia.New(opts...)
			err := vm.Exec(`
llm.register_models({
    default: "fast"
    fast: "cheap"
    cheap: "fast"
})
`)
			if err == nil || !strings.Contains(err.Error(), "llm model alias cycle: fast -> cheap -> fast") {
				t.Fatalf("Exec error = %v, want model alias cycle", err)
			}
		})
	}
}
