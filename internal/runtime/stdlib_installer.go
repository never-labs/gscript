package runtime

// StdlibInstaller is the narrow boundary used by standard-library installers.
// Modules register their public runtime value once; the installer owns the
// shared global, require-cache, and package.loaded bookkeeping. Alias bindings
// are global-only and do not become require-able modules.
type StdlibInstaller interface {
	RegisterModule(name string, module Value)
	RegisterTable(name string, table *Table)
	RegisterAlias(name string, value Value)
}

type stdlibInstallContext struct {
	interp *Interpreter
	loaded *Table
}

func newStdlibInstallContext(interp *Interpreter) *stdlibInstallContext {
	return &stdlibInstallContext{
		interp: interp,
		loaded: NewTable(),
	}
}

func (ctx *stdlibInstallContext) RegisterModule(name string, module Value) {
	ctx.interp.globals.Define(name, module)
	ctx.interp.modules[name] = module
	ctx.loaded.RawSetString(name, module)
}

func (ctx *stdlibInstallContext) RegisterTable(name string, table *Table) {
	ctx.RegisterModule(name, TableValue(table))
}

func (ctx *stdlibInstallContext) RegisterAlias(name string, value Value) {
	ctx.interp.globals.Define(name, value)
}

func (ctx *stdlibInstallContext) InstallPackage(path string) {
	pkg := NewTable()
	pkg.RawSetString("loaded", TableValue(ctx.loaded))
	pkg.RawSetString("path", StringValue(path))
	ctx.interp.globals.Define("package", TableValue(pkg))
}
