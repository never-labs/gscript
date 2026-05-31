package runtime

// registerStdlibExtra registers additional standard library tables (utf8, bit32).
// This is called from New() after registerStdlib().
func (interp *Interpreter) registerStdlibExtra() {
	// UTF-8 library
	interp.globals.Define("utf8", TableValue(buildUTF8Lib(interp)))

	// Bit32 library
	interp.globals.Define("bit32", TableValue(buildBit32Lib()))
}
