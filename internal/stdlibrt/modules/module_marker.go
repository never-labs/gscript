package modules

func markStdlibrtModule(t *Table) *Table {
	if t != nil {
		t.RawSetString("__stdlibrt_module", BoolValue(true))
	}
	return t
}

func IsStdlibrtModule(t *Table) bool {
	return t != nil && t.RawGetString("__stdlibrt_module").Truthy()
}
