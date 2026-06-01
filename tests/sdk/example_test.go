package leia_test

import (
	"errors"
	"fmt"

	leia "github.com/never-labs/leia"
)

func ExampleCompile() {
	prog, err := leia.Compile(`result := 40 + 2`, leia.WithSourceName("calc.leia"))
	if err != nil {
		panic(err)
	}

	vm := leia.New(leia.WithVM())
	if err := vm.Run(prog); err != nil {
		panic(err)
	}

	result, err := vm.Get("result")
	if err != nil {
		panic(err)
	}
	fmt.Println(prog.SourceName(), result)

	// Output:
	// calc.leia 42
}

func ExampleValue() {
	vm := leia.New()
	if err := vm.Set("seed", leia.Int(21)); err != nil {
		panic(err)
	}
	if err := vm.Exec(`answer := seed * 2`); err != nil {
		panic(err)
	}

	answer, err := vm.Get("answer")
	if err != nil {
		panic(err)
	}
	fmt.Println(leia.Int(21).Kind(), answer)

	// Output:
	// int 42
}

func ExampleVM_RegisterFunc() {
	vm := leia.New()
	if err := vm.RegisterFunc("double", func(v int64) int64 {
		return v * 2
	}); err != nil {
		panic(err)
	}

	results, err := vm.Call("double", 21)
	if err != nil {
		panic(err)
	}
	fmt.Println(results[0])

	// Output:
	// 42
}

func ExampleWithSandbox() {
	vm := leia.New(leia.WithSandbox())
	if err := vm.Exec(`answer := 40 + 2`); err != nil {
		panic(err)
	}

	fs, err := vm.Get("fs")
	if err != nil {
		panic(err)
	}
	process, err := vm.Get("process")
	if err != nil {
		panic(err)
	}
	fmt.Println(fs == nil && process == nil)

	// Output:
	// true
}

func ExampleWithMaxSteps() {
	vm := leia.New(leia.WithMaxSteps(8))
	err := vm.Exec(`
		i := 0
		for {
			i += 1
		}
	`)

	var budgetErr *leia.BudgetError
	fmt.Println(errors.As(err, &budgetErr), budgetErr.Resource, budgetErr.Limit)

	// Output:
	// true steps 8
}

func ExampleError() {
	sentinel := errors.New("host failed")
	vm := leia.New()
	if err := vm.RegisterFunc("fail", func() error {
		return sentinel
	}); err != nil {
		panic(err)
	}

	err := vm.Exec(`fail()`)

	var leiaErr *leia.Error
	var hostErr *leia.HostCallbackError
	fmt.Println(errors.As(err, &leiaErr), leiaErr.Kind)
	fmt.Println(errors.As(err, &hostErr), hostErr.Name, errors.Is(err, sentinel))

	// Output:
	// true runtime
	// true fail true
}
