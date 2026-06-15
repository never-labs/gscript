package install

import "github.com/never-labs/leia/internal/runtime"

// DefaultAlias describes one prelude/default-import binding. Aliases are
// global-only conveniences owned by their source module; require(name) still
// resolves only real modules.
type DefaultAlias struct {
	Name   string
	Module string
	Member string
}

var defaultAliases = []DefaultAlias{
	{Name: "append", Module: "table", Member: "append"},

	{Name: "abs", Module: "math", Member: "abs"},
	{Name: "sqrt", Module: "math", Member: "sqrt"},
	{Name: "exp", Module: "math", Member: "exp"},
	{Name: "sin", Module: "math", Member: "sin"},
	{Name: "cos", Module: "math", Member: "cos"},
	{Name: "tan", Module: "math", Member: "tan"},
	{Name: "asin", Module: "math", Member: "asin"},
	{Name: "acos", Module: "math", Member: "acos"},
	{Name: "atan", Module: "math", Member: "atan"},
	{Name: "floor", Module: "math", Member: "floor"},
	{Name: "ceil", Module: "math", Member: "ceil"},
	{Name: "round", Module: "math", Member: "round"},
	{Name: "min", Module: "math", Member: "min"},
	{Name: "max", Module: "math", Member: "max"},
	{Name: "clamp", Module: "math", Member: "clamp"},
	{Name: "near", Module: "math", Member: "near"},
	{Name: "pow", Module: "math", Member: "pow"},

	{Name: "vector", Module: "linalg", Member: "vector"},
	{Name: "vec", Module: "linalg", Member: "vec"},
	{Name: "mat", Module: "linalg", Member: "matrix"},
	{Name: "row", Module: "linalg", Member: "row"},
	{Name: "col", Module: "linalg", Member: "col"},
	{Name: "eye", Module: "linalg", Member: "eye"},
	{Name: "diag", Module: "linalg", Member: "diag"},
	{Name: "zeros", Module: "linalg", Member: "zeros"},
	{Name: "ones", Module: "linalg", Member: "ones"},
	{Name: "at", Module: "linalg", Member: "at"},
	{Name: "norm", Module: "linalg", Member: "norm"},
	{Name: "dot", Module: "linalg", Member: "dot"},
	{Name: "matvec", Module: "linalg", Member: "matvec"},
	{Name: "matmul", Module: "linalg", Member: "matmul"},
	{Name: "axpy", Module: "linalg", Member: "axpy"},
	{Name: "solve", Module: "linalg", Member: "solve"},
	{Name: "trace", Module: "linalg", Member: "trace"},
	{Name: "transpose", Module: "linalg", Member: "transpose"},

	{Name: "sum", Module: "stats", Member: "sum"},
	{Name: "mean", Module: "stats", Member: "mean"},
	{Name: "avg", Module: "stats", Member: "mean"},
	{Name: "variance", Module: "stats", Member: "variance"},
	{Name: "std", Module: "stats", Member: "std"},
	{Name: "describe", Module: "stats", Member: "describe"},
	{Name: "rms", Module: "stats", Member: "rms"},
	{Name: "rmse", Module: "stats", Member: "rmse"},
	{Name: "cumsum", Module: "stats", Member: "cumsum"},
	{Name: "diff", Module: "stats", Member: "diff"},
	{Name: "normalize", Module: "stats", Member: "normalize"},

	{Name: "randn", Module: "rand", Member: "normal_vec"},
	{Name: "sample", Module: "rand", Member: "sample"},
}

// DefaultAliases returns the default-import alias table. The returned slice is
// a copy so callers can use it for restriction checks without owning state.
func DefaultAliases() []DefaultAlias {
	return append([]DefaultAlias(nil), defaultAliases...)
}

func registerDefaultAliases(installer runtime.StdlibInstaller, moduleName string, table *runtime.Table) {
	if installer == nil || table == nil {
		return
	}
	for _, alias := range defaultAliases {
		if alias.Module != moduleName {
			continue
		}
		value := table.RawGetString(alias.Member)
		if value.IsNil() {
			continue
		}
		installer.RegisterAlias(alias.Name, value)
	}
}
