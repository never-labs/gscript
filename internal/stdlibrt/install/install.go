package install

import (
	"github.com/never-labs/gscript/internal/runtime"
	"github.com/never-labs/gscript/internal/stdlibrt/modules"
)

// Install registers the standard library on interp.
// The implementation currently delegates to runtime while modules migrate out
// of that package; keeping this boundary lets callers stop depending on
// runtime.New's historical implicit stdlib install.
func Install(interp *runtime.Interpreter) {
	if interp == nil {
		return
	}
	interp.InstallStdlib()
	installModule(interp, "base64", runtime.TableValue(modules.BuildBase64(interp.MaxHostResultBytes)))
	installModule(interp, "bits", runtime.TableValue(modules.BuildBits()))
	installModule(interp, "hash", runtime.TableValue(modules.BuildHash()))
	installModule(interp, "path", runtime.TableValue(modules.BuildPath()))
	installModule(interp, "uuid", runtime.TableValue(modules.BuildUUID()))
}

func installModule(interp *runtime.Interpreter, name string, module runtime.Value) {
	interp.SetGlobal(name, module)
	interp.SetModule(name, module)
}
