package main

import (
	"github.com/never-labs/leia/internal/runtime"
	stdlibinstall "github.com/never-labs/leia/internal/stdlibrt/install"
)

func newCLIInterpreter() *runtime.Interpreter {
	interp := runtime.NewCore()
	stdlibinstall.Install(interp)
	return interp
}
