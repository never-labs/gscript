package vmtest

import (
	"github.com/never-labs/leia/internal/runtime"
	"github.com/never-labs/leia/internal/stdlib/install"
)

// NewInterpreterGlobals returns globals for VM/JIT tests that need the full
// stdlib surface. It builds that surface through the stdlib installer instead
// of depending on runtime.New(), keeping the runtime core usable without the
// complete standard library.
func NewInterpreterGlobals() map[string]runtime.Value {
	interp := runtime.NewCore()
	install.Install(interp)
	return interp.ExportGlobals()
}
