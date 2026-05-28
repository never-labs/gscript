package embedding_test

import (
	"errors"
	"fmt"

	gs "github.com/gscript/gscript/gscript"
)

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
