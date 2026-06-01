package embedding_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	leia "github.com/never-labs/leia"
	"github.com/never-labs/leia/llm"
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
				Args: map[string]any{"name": "leia"},
			}},
		}, nil
	}
	return llm.TurnResult{Status: "final_answer", Text: "hello"}, nil
}

func Example_compileRun() {
	prog, err := leia.Compile(`
		func score(base, bonus) {
			return base * 10 + bonus
		}
		result := score(4, 2)
	`, leia.WithSourceName("score.leia"))
	if err != nil {
		panic(err)
	}

	vm := leia.New()
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
	vm := leia.New()
	if err := vm.Set("name", leia.String("embedder")); err != nil {
		panic(err)
	}
	if err := vm.Set("count", leia.Int(3)); err != nil {
		panic(err)
	}

	if err := vm.Exec(`message := name .. ":" .. tostring(count * 2)`); err != nil {
		panic(err)
	}

	message, err := vm.Get("message")
	if err != nil {
		panic(err)
	}
	encoded, err := leia.String("ready").Encode()
	if err != nil {
		panic(err)
	}

	fmt.Println(message)
	fmt.Println(leia.Int(42).Kind(), leia.Int(42).Int())
	fmt.Println(encoded)

	// Output:
	// embedder:6
	// int 42
	// ready
}

func Example_hostFunctionBinding() {
	vm := leia.New()
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
	vm := leia.New(leia.WithSandbox(), leia.WithModuleLoading(false))
	if err := vm.RegisterModule("go/strings", leia.Module{
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
	vm := leia.New(leia.WithLibs(leia.LibString|leia.LibLLM), leia.WithLLMProvider(exampleLLMProvider{}))
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
	// docs:leia
}

func Example_hotLoader() {
	dir, err := os.MkdirTemp("", "leia-hotloader-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "logic.leia")
	if err := os.WriteFile(path, []byte(`func answer() { return 1 }`), 0644); err != nil {
		panic(err)
	}

	loader := leia.NewHotLoader()
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

	result, err := handle.Call(leia.New(), "answer")
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
	dir, err := os.MkdirTemp("", "leia-hotinstance-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "logic.leia")
	if err := os.WriteFile(path, []byte(`
counter := 0
func inc() {
	counter += 1
	return counter
}
`), 0644); err != nil {
		panic(err)
	}

	loader := leia.NewHotLoader()
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
	sandbox := leia.New(leia.WithSandbox())
	if err := sandbox.Exec(`safe := true`); err != nil {
		panic(err)
	}
	fsGlobal, err := sandbox.Get("fs")
	if err != nil {
		panic(err)
	}
	fmt.Println("sandbox fs", fsGlobal)

	limited := leia.New(leia.WithMaxSteps(8))
	err = limited.Exec(`
		i := 0
		for {
			i += 1
		}
	`)
	var budgetErr *leia.BudgetError
	fmt.Println("budget", errors.As(err, &budgetErr), budgetErr.Resource, budgetErr.Limit)

	// Output:
	// sandbox fs <nil>
	// budget true steps 8
}

func Example_structuredErrors() {
	hostFailed := errors.New("host failed")
	vm := leia.New()
	if err := vm.RegisterFunc("fail", func() error {
		return hostFailed
	}); err != nil {
		panic(err)
	}

	err := vm.Exec(`fail()`)

	var scriptErr *leia.Error
	var hostErr *leia.HostCallbackError
	fmt.Println(errors.As(err, &scriptErr), scriptErr.Kind)
	fmt.Println(errors.As(err, &hostErr), hostErr.Name, errors.Is(err, hostFailed))

	// Output:
	// true runtime
	// true fail true
}
