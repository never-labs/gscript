package main

import (
	"github.com/never-labs/gscript/internal/runtime"
	stdlibinstall "github.com/never-labs/gscript/internal/stdlibrt/install"
)

func newCLIInterpreter() *runtime.Interpreter {
	interp := runtime.NewCore()
	stdlibinstall.Install(interp)
	return interp
}
