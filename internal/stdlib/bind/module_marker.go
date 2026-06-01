package bind

func markRuntimeModule(t *Table) *Table {
	if t != nil {
		t.RawSetString("__runtime_module", BoolValue(true))
	}
	return t
}

func IsStdlibrtModule(t *Table) bool {
	return t != nil && t.RawGetString("__runtime_module").Truthy()
}
