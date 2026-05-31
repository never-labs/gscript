package embedding_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	gs "github.com/never-labs/gscript"
	"github.com/never-labs/gscript/llm"
)

type exampleLLMProvider struct{}

func (exampleLLMProvider) Turn(_ context.Context, req llm.TurnRequest) (llm.TurnResult, error) {
	for _, msg := range req.Messages {
		if msg.Role == "tool" && msg.Value != nil {
			return llm.TurnResult{Status: "final_answer", Text: fmt.Sprint(msg.Value)}, nil
		}
	}
	if len(req.Tools) > 0 {
		return llm.TurnResult{
			Status: "tool_calls",
			Calls: []llm.ToolCall{{
				ID:   "call_1",
				Tool: req.Tools[0].Name,
				Args: map[string]any{"name": "gscript"},
			}},
		}, nil
	}
	return llm.TurnResult{Status: "final_answer", Text: "hello"}, nil
}

func Example_compileRun() {
	prog, err := gs.Compile(`
		func score(base, bonus) {
			return base * 10 + bonus
		}
		result := score(4, 2)
	`, gs.WithSourceName("score.gs"))
	if err != nil {
		panic(err)
	}

	vm := gs.New()
	if err := vm.Run(prog); err != nil {
		panic(err)
	}

	result, err := vm.Get("result")
	if err != nil {
		panic(err)
	}
	fmt.Println(result)

	// Output:
	// 42
}

func Example_value() {
	vm := gs.New()
	if err := vm.Set("name", gs.String("embedder")); err != nil {
		panic(err)
	}
	if err := vm.Set("count", gs.Int(3)); err != nil {
		panic(err)
	}

	if err := vm.Exec(`message := name .. ":" .. tostring(count * 2)`); err != nil {
		panic(err)
	}

	message, err := vm.Get("message")
	if err != nil {
		panic(err)
	}
	encoded, err := gs.String("ready").Encode()
	if err != nil {
		panic(err)
	}

	fmt.Println(message)
	fmt.Println(gs.Int(42).Kind(), gs.Int(42).Int())
	fmt.Println(encoded)

	// Output:
	// embedder:6
	// int 42
	// ready
}

func Example_hostFunctionBinding() {
	vm := gs.New()
	if err := vm.RegisterFunc("label", func(prefix string, id int64) string {
		return fmt.Sprintf("%s-%03d", prefix, id)
	}); err != nil {
		panic(err)
	}

	if err := vm.Exec(`ticket := label("job", 7)`); err != nil {
		panic(err)
	}

	ticket, err := vm.Get("ticket")
	if err != nil {
		panic(err)
	}
	fmt.Println(ticket)

	// Output:
	// job-007
}

func Example_hostModuleRequire() {
	vm := gs.New(gs.WithSandbox(), gs.WithModuleLoading(false))
	if err := vm.RegisterModule("go/strings", gs.Module{
		"upper": strings.ToUpper,
	}); err != nil {
		panic(err)
	}

	if err := vm.Exec(`
strings := require("go/strings")
result := strings.upper("hello")
`); err != nil {
		panic(err)
	}

	result, err := vm.Get("result")
	if err != nil {
		panic(err)
	}
	fmt.Println(result)

	// Output:
	// HELLO
}

func Example_llmProvider() {
	vm := gs.New(gs.WithLibs(gs.LibString|gs.LibLLM), gs.WithLLMProvider(exampleLLMProvider{}))
	if err := vm.Exec(`
lookup := llm.tool("lookup", func(name) {
    return "docs:" .. name, nil
}, {description: "lookup documentation", params: {"name"}})

tools := {lookup}
	result, err := llm.react({
    messages: {llm.system("Use tools."), llm.user("find docs")},
    tools: tools,
    max_steps: 2,
})
answer := result.text
`); err != nil {
		panic(err)
	}
	answer, err := vm.Get("answer")
	if err != nil {
		panic(err)
	}
	fmt.Println(answer)

	// Output:
	// docs:gscript
}

func Example_hotLoader() {
	dir, err := os.MkdirTemp("", "gscript-hotloader-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "logic.gs")
	if err := os.WriteFile(path, []byte(`func answer() { return 1 }`), 0644); err != nil {
		panic(err)
	}

	loader := gs.NewHotLoader()
	handle, err := loader.Load(path)
	if err != nil {
		panic(err)
	}

	if err := os.WriteFile(path, []byte(`func answer() { return 2 }`), 0644); err != nil {
		panic(err)
	}
	if err := loader.Reload(path); err != nil {
		panic(err)
	}

	result, err := handle.Call(gs.New(), "answer")
	if err != nil {
		panic(err)
	}
	fmt.Println(handle.Generation(), result[0])

	if err := os.WriteFile(path, []byte(`func answer() {`), 0644); err != nil {
		panic(err)
	}
	fmt.Println(loader.Reload(path) != nil, handle.Generation())

	// Output:
	// 2 2
	// true 2
}

func Example_hotInstance() {
	dir, err := os.MkdirTemp("", "gscript-hotinstance-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "logic.gs")
	if err := os.WriteFile(path, []byte(`
counter := 0
func inc() {
	counter += 1
	return counter
}
`), 0644); err != nil {
		panic(err)
	}

	loader := gs.NewHotLoader()
	inst, err := loader.LoadInstance(path)
	if err != nil {
		panic(err)
	}
	first, _ := inst.Call("inc")
	second, _ := inst.Call("inc")

	if err := os.WriteFile(path, []byte(`
counter := 0
func inc() {
	counter += 10
	return counter
}
`), 0644); err != nil {
		panic(err)
	}
	if err := inst.Reload(); err != nil {
		panic(err)
	}
	third, _ := inst.Call("inc")

	fmt.Println(first[0], second[0], third[0])

	// Output:
	// 1 2 12
}

func Example_sandboxAndMaxSteps() {
	sandbox := gs.New(gs.WithSandbox())
	if err := sandbox.Exec(`safe := true`); err != nil {
		panic(err)
	}
	fsGlobal, err := sandbox.Get("fs")
	if err != nil {
		panic(err)
	}
	fmt.Println("sandbox fs", fsGlobal)

	limited := gs.New(gs.WithMaxSteps(8))
	err = limited.Exec(`
		i := 0
		for {
			i += 1
		}
	`)
	var budgetErr *gs.BudgetError
	fmt.Println("budget", errors.As(err, &budgetErr), budgetErr.Resource, budgetErr.Limit)

	// Output:
	// sandbox fs <nil>
	// budget true steps 8
}

func Example_structuredErrors() {
	hostFailed := errors.New("host failed")
	vm := gs.New()
	if err := vm.RegisterFunc("fail", func() error {
		return hostFailed
	}); err != nil {
		panic(err)
	}

	err := vm.Exec(`fail()`)

	var scriptErr *gs.Error
	var hostErr *gs.HostCallbackError
	fmt.Println(errors.As(err, &scriptErr), scriptErr.Kind)
	fmt.Println(errors.As(err, &hostErr), hostErr.Name, errors.Is(err, hostFailed))

	// Output:
	// true runtime
	// true fail true
}
