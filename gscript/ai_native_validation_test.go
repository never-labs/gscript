package gscript_test

import (
	"strings"
	"testing"

	gs "github.com/never-labs/gscript/gscript"
)

func TestAINativeSyntaxValidation(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
		want string
	}{
		{
			name: "duplicate defaults",
			src: `
agent defaults { model: "a" }
agent defaults { model: "b" }
`,
			want: "duplicate agent defaults",
		},
		{
			name: "nested defaults",
			src: `
func f() {
    agent defaults { model: "a" }
}
`,
			want: "module scope",
		},
		{
			name: "literal api key",
			src: `
models {
    default: "m"
    m: {provider_model: "m", api_key: "secret"}
}
`,
			want: "api_key",
		},
		{
			name: "provider protocol must be string",
			src: `
models {
    default: "m"
    m: {protocol: ("openai" .. "_compatible"), provider_model: "m"}
}
`,
			want: "protocol must be a string literal",
		},
		{
			name: "provider protocol whitelist",
			src: `
models {
    default: "m"
    m: {protocol: "unsupported-test-protocol", provider_model: "m"}
}
`,
			want: "unsupported model protocol",
		},
		{
			name: "provider config requires model",
			src: `
models {
    default: "m"
    m: {protocol: "openai_compatible", base_url: "http://127.0.0.1:1"}
}
`,
			want: "provider_model or model",
		},
		{
			name: "model alias cycle",
			src: `
models {
    a: "b"
    b: "a"
}
`,
			want: "alias cycle",
		},
		{
			name: "tool missing requires",
			src: `
tool lookup(query) {
    return query, nil
}
`,
			want: "missing gscript:requires",
		},
		{
			name: "tool invalid requires",
			src: `
//gscript:requires docs..read
tool lookup(query) {
    return query, nil
}
`,
			want: "invalid gscript:requires",
		},
		{
			name: "tool unknown param doc",
			src: `
//gscript:requires none
//gscript:param missing not a parameter
tool lookup(query) {
    return query, nil
}
`,
			want: "unknown parameter",
		},
		{
			name: "tool duplicate param doc",
			src: `
//gscript:requires none
//gscript:param query first
//gscript:param query second
tool lookup(query) {
    return query, nil
}
`,
			want: "duplicate gscript:param",
		},
		{
			name: "agent duplicate tools",
			src: `
//gscript:requires none
tool lookup(query) {
    return query, nil
}

agent answer(q) {
    tools: [lookup, lookup]
    user: q
}
`,
			want: "duplicate tool",
		},
		{
			name: "agent unknown static tool",
			src: `
agent answer(q) {
    tools: [missing]
    user: q
}
`,
			want: "undeclared tool",
		},
		{
			name: "defaults unknown static tool",
			src: `
agent defaults {
    tools: [missing]
}
`,
			want: "undeclared tool",
		},
		{
			name: "turn unknown static tool",
			src: `
func f() {
    _ = turn {
        tools: [missing]
        user: "hello"
    }
}
`,
			want: "undeclared tool",
		},
		{
			name: "agent capabilities missing tool requirement",
			src: `
//gscript:requires docs.read, net.client
tool lookup(query) {
    return query, nil
}

agent answer(q) {
    tools: [lookup]
    capabilities: ["docs.read"]
    user: q
}
`,
			want: "capabilities missing required capability \"net.client\"",
		},
		{
			name: "agent defaults merged capabilities missing inherited tool requirement",
			src: `
//gscript:requires net.client
tool lookup(query) {
    return query, nil
}

agent defaults {
    tools: [lookup]
}

agent answer(q) {
    capabilities: []
    user: q
}
`,
			want: "capabilities missing required capability \"net.client\"",
		},
		{
			name: "turn caps missing tool requirement",
			src: `
//gscript:requires payments.refund
tool refund(id) {
    return id, nil
}

func f() {
    _ = turn {
        tools: [refund]
        caps: []
        user: "refund"
    }
}
`,
			want: "capabilities missing required capability \"payments.refund\"",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, mode := range []struct {
				name string
				opts []gs.Option
			}{
				{name: "interpreter"},
				{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
			} {
				t.Run(mode.name, func(t *testing.T) {
					opts := append([]gs.Option{gs.WithLibs(gs.LibString | gs.LibLLM)}, mode.opts...)
					vm := gs.New(opts...)
					err := vm.Exec(tc.src)
					if err == nil || !strings.Contains(err.Error(), tc.want) {
						t.Fatalf("Exec error = %v, want substring %q", err, tc.want)
					}
				})
			}
		})
	}
}

func TestAINativeSyntaxValidationAllowsAliasOnlyModels(t *testing.T) {
	for _, mode := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(mode.name, func(t *testing.T) {
			opts := append([]gs.Option{gs.WithLibs(gs.LibString | gs.LibLLM)}, mode.opts...)
			vm := gs.New(opts...)
			err := vm.Exec(`
models {
    default: "fast"
    fast: "host-fast"
    host: {provider_model: "host-only"}
}
`)
			if err != nil {
				t.Fatalf("Exec: %v", err)
			}
		})
	}
}

func TestAINativeSyntaxValidationAllowsStaticToolCapsCoverage(t *testing.T) {
	for _, mode := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(mode.name, func(t *testing.T) {
			opts := append([]gs.Option{gs.WithLibs(gs.LibString | gs.LibLLM)}, mode.opts...)
			vm := gs.New(opts...)
			err := vm.Exec(`
//gscript:requires docs.read, net.client
tool lookup(query) {
    return query, nil
}

//gscript:requires none
tool local_only(query) {
    return query, nil
}

agent defaults {
    tools: [lookup]
}

agent inherited(q) {
    capabilities: ["docs.read", "net.client"]
    user: q
}

agent override(q) {
    tools: [local_only]
    capabilities: []
    user: q
}

agent answer(q) {
    tools: [lookup]
    capabilities: ["docs.read", "net.client"]
    user: q
}

func f(caps) {
    _ = turn {
        tools: [lookup]
        caps: caps
        user: "hello"
    }
}
`)
			if err != nil {
				t.Fatalf("Exec: %v", err)
			}
		})
	}
}

func TestAINativeSyntaxValidationAllowsDynamicDefaultsToolCapsRefs(t *testing.T) {
	for _, mode := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(mode.name, func(t *testing.T) {
			opts := append([]gs.Option{gs.WithLibs(gs.LibString | gs.LibLLM)}, mode.opts...)
			vm := gs.New(opts...)
			err := vm.Exec(`
//gscript:requires net.client
tool lookup(query) {
    return query, nil
}

default_tools := [lookup]
default_caps := []

agent defaults {
    tools: default_tools
    capabilities: default_caps
}

agent answer(q) {
    user: q
}
`)
			if err != nil {
				t.Fatalf("Exec: %v", err)
			}
		})
	}
}

func TestAINativeSyntaxValidationAllowsDynamicToolRefs(t *testing.T) {
	for _, mode := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(mode.name, func(t *testing.T) {
			opts := append([]gs.Option{gs.WithLibs(gs.LibString | gs.LibLLM)}, mode.opts...)
			vm := gs.New(opts...)
			err := vm.Exec(`
func f(tools) {
    _ = turn {
        tools: [tools[0]]
        user: "hello"
    }
}
`)
			if err != nil {
				t.Fatalf("Exec: %v", err)
			}
		})
	}
}

func TestAINativeSyntaxValidationAllowsScopedToolRefs(t *testing.T) {
	for _, mode := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(mode.name, func(t *testing.T) {
			opts := append([]gs.Option{gs.WithLibs(gs.LibString | gs.LibLLM)}, mode.opts...)
			vm := gs.New(opts...)
			err := vm.Exec(`
func make_agent(prefix) {
    //gscript:requires none
    tool local_lookup(query) {
        return prefix .. query, nil
    }
    return agent(q) {
        tools: [local_lookup]
        user: q
    }
}
`)
			if err != nil {
				t.Fatalf("Exec: %v", err)
			}
		})
	}
}
