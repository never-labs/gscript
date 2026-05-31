package runtime

// InstallRuntimeStdlib registers runtime-owned standard-library tables as
// globals. stdlibrt/install calls this before installing modules that have
// migrated out of runtime, so this method must not register migrated modules.
func (interp *Interpreter) InstallRuntimeStdlib() {
	interp.installStdlib(false)
}

// InstallStdlib registers compatibility standard-library tables that are still
// owned by runtime. Embeddings that need the full public stdlib should use
// runtime.NewCore plus stdlibrt/install.Install.
func (interp *Interpreter) InstallStdlib() {
	interp.installStdlib(true)
}

func (interp *Interpreter) installStdlib(includeMigratedCompat bool) {
	std := newStdlibInstallContext(interp)

	if includeMigratedCompat {
		interp.installRuntimeOwnedStdlib(std)
	}

	// --- System ---
	std.RegisterTable("script", buildScriptLib(interp))

	std.InstallPackage(interp.scriptDir)
}

// installRuntimeOwnedStdlib registers compatibility stdlib surfaces that have
// not yet been removed from runtime.InstallStdlib. Public embedding goes
// through stdlibrt/install, so migrated modules should not be added here.
func (interp *Interpreter) installRuntimeOwnedStdlib(std StdlibInstaller) {
	strLib := BuildStringLibWithCaller(interp.callFunction, func() int64 { return interp.maxHostResult })
	std.RegisterTable("string", strLib)
	interp.SetStringLibrary(strLib)
}
