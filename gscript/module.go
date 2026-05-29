package gscript

// Module is a Go-backed namespace exposed to GScript through require(name).
// Values use the same reflection conversion rules as RegisterFunc and Set.
type Module map[string]interface{}
