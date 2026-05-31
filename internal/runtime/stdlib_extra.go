package runtime

// registerStdlibExtra registers additional standard library tables.
// This is called from New() after registerStdlib().
func (interp *Interpreter) registerStdlibExtra() {
	// UTF-8 library
	interp.globals.Define("utf8", TableValue(buildUTF8Lib(interp)))
}
