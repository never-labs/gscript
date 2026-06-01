package main

import (
	"github.com/never-labs/leia/internal/runtime"
	stdlibinstall "github.com/never-labs/leia/internal/stdlib/install"
)

func newCLIInterpreter() *runtime.Interpreter {
	interp := runtime.NewCore()
	stdlibinstall.Install(interp)
	return interp
}
