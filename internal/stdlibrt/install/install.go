package install

import "github.com/never-labs/gscript/internal/runtime"

// Install registers the standard library on interp.
// The implementation currently delegates to runtime while modules migrate out
// of that package; keeping this boundary lets callers stop depending on
// runtime.New's historical implicit stdlib install.
func Install(interp *runtime.Interpreter) {
	if interp == nil {
		return
	}
	interp.InstallStdlib()
}
