package leia_test

import (
	"testing"

	leia "github.com/never-labs/leia"
)

func TestLLMSyntaxValidationAllowsStaticToolCapsCoverage(t *testing.T) {
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
lookup := llm.tool("lookup", func(query) {
    return query, nil
}, {params: {"query"}, requires: {"docs.read", "net.client"}})

local_only := llm.tool("local_only", func(query) {
    return query, nil
}, {params: {"query"}, requires: {"none"}})

llm.agent_defaults({tools: {lookup}})

inherited := llm.agent("inherited", func(q) {
    return {
        capabilities: {"docs.read", "net.client"}
        user: q
    }, nil
})

override := llm.agent("override", func(q) {
    return {
        tools: {local_only}
        capabilities: {}
        user: q
    }, nil
})

answer := llm.agent("answer", func(q) {
    return {
        tools: {lookup}
        capabilities: {"docs.read", "net.client"}
        user: q
    }, nil
})

func f(caps) {
    _ := {
        tools: {lookup}
        caps: caps
        messages: {llm.user("hello")}
    }
}

ok, err := llm.check_tools({lookup}, {"docs.read", "net.client"})
override_ok, override_err := llm.check_tools({local_only}, {})
`)
			if err != nil {
				t.Fatalf("Exec: %v", err)
			}
		})
	}
}

func TestLLMSyntaxValidationAllowsDynamicDefaultsToolCapsRefs(t *testing.T) {
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
lookup := llm.tool("lookup", func(query) {
    return query, nil
}, {params: {"query"}, requires: {"net.client"}})

default_tools := {lookup}
default_caps := {}

llm.agent_defaults({
    tools: default_tools
    capabilities: default_caps
})

answer := llm.agent("answer", func(q) {
    return {user: q}, nil
})
`)
			if err != nil {
				t.Fatalf("Exec: %v", err)
			}
		})
	}
}

func TestLLMSyntaxValidationAllowsDynamicToolRefs(t *testing.T) {
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
func f(tools) {
    _ := {
        tools: {tools[1]}
        messages: {llm.user("hello")}
    }
}
`)
			if err != nil {
				t.Fatalf("Exec: %v", err)
			}
		})
	}
}

func TestLLMSyntaxValidationAllowsScopedToolRefs(t *testing.T) {
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
func make_agent(prefix) {
    local_lookup := llm.tool("local_lookup", func(query) {
        return prefix .. query, nil
    }, {params: {"query"}, requires: {"none"}})
    return llm.agent("local_agent", func(q) {
        return {
            tools: {local_lookup}
            user: q
        }, nil
    })
}
`)
			if err != nil {
				t.Fatalf("Exec: %v", err)
			}
		})
	}
}
