package bind

func markStdlibBoundModule(t *Table) *Table {
	if t != nil {
		t.RawSetString("__stdlib_bound_module", BoolValue(true))
	}
	return t
}

func IsStdlibBoundModule(t *Table) bool {
	return t != nil && t.RawGetString("__stdlib_bound_module").Truthy()
}
