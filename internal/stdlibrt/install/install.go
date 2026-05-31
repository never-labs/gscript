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
	InstallModules(interpreterInstaller{interp: interp}, interp.MaxHostResultBytes)
}

// InstallModules registers stdlibrt-owned modules on a runtime-compatible
// installer. VM and tree-walker entry points use this to avoid separate module
// construction paths while the broader stdlib continues to migrate.
func InstallModules(installer runtime.StdlibInstaller, maxHostResult func() int64) {
	if installer == nil {
		return
	}
	if maxHostResult == nil {
		maxHostResult = func() int64 { return 0 }
	}
	installer.RegisterTable("base64", modules.BuildBase64(maxHostResult))
	installer.RegisterTable("bits", modules.BuildBits())
	installer.RegisterTable("compress", modules.BuildCompress(maxHostResult))
	installer.RegisterTable("csv", modules.BuildCSV(maxHostResult))
	installer.RegisterTable("encoding", modules.BuildEncoding(maxHostResult))
	installer.RegisterTable("hash", modules.BuildHash())
	installer.RegisterTable("path", modules.BuildPath())
	installer.RegisterTable("rand", modules.BuildRand())
	installer.RegisterTable("regexp", modules.BuildRegexp())
	installer.RegisterTable("url", modules.BuildURL(maxHostResult))
	installer.RegisterTable("uuid", modules.BuildUUID())
}

type interpreterInstaller struct {
	interp *runtime.Interpreter
}

func (installer interpreterInstaller) RegisterModule(name string, module runtime.Value) {
	if installer.interp == nil {
		return
	}
	installer.interp.SetGlobal(name, module)
	installer.interp.SetModule(name, module)
}

func (installer interpreterInstaller) RegisterTable(name string, table *runtime.Table) {
	installer.RegisterModule(name, runtime.TableValue(table))
}

func (installer interpreterInstaller) RegisterAlias(name string, value runtime.Value) {
	if installer.interp == nil {
		return
	}
	installer.interp.SetGlobal(name, value)
}
