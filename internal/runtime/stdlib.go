package runtime

// InstallRuntimeStdlib registers runtime-owned standard-library tables as
// globals. stdlibrt/install calls this before installing modules that have
// migrated out of runtime, so this method must not register migrated modules.
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
