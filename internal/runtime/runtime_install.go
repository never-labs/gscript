package runtime

// InstallRuntimeStdlib registers only the runtime-owned compatibility surface.
// stdlibrt/install composes the full public standard library on top of this
// core, so this method must not register stdlibrt-owned modules.
func (interp *Interpreter) InstallRuntimeStdlib() {
	interp.installStdlib()
}

// InstallStdlib registers the runtime-owned compatibility surface. Embeddings
// that need the full public stdlib should use runtime.NewCore plus
// stdlibrt/install.Install.
func (interp *Interpreter) InstallStdlib() {
	interp.installStdlib()
}

func (interp *Interpreter) installStdlib() {
	std := newStdlibInstallContext(interp)

	// --- System ---
	std.RegisterTable("script", buildScriptLib(interp))

	std.InstallPackage(interp.scriptDir)
}
