package vm

// Standard-library registration and lib-builder helpers, split verbatim from vm.go.

import (
	"errors"
	"fmt"
	"github.com/gscript/gscript/internal/runtime"
	"os"
	"sort"
	"strings"
)

func (vm *VM) RestrictStdlib(allowed map[string]bool) {
	for _, name := range runtime.StdlibModuleNames() {
		if allowed[name] {
			continue
		}
		vm.DeleteGlobal(name)
		vm.setPackageLoaded(name, runtime.NilValue())
		if name == "string" {
			vm.stringMeta = nil
		}
	}
}

func (vm *VM) setPackageLoaded(name string, val runtime.Value) {
	pkgVal, ok := vm.globals["package"]
	if !ok || !pkgVal.IsTable() {
		return
	}
	loaded := pkgVal.Table().RawGet(runtime.StringValue("loaded"))
	if loaded.IsTable() {
		loaded.Table().RawSetString(name, val)
	}
}

// RegisterProtectedCallLib installs VM-aware pcall/xpcall builtins so protected
// calls can invoke ordinary VM closures.

func (vm *VM) RegisterProtectedCallLib() {
	vm.SetGlobal("pcall", runtime.FunctionValue(vm.newPCallFunction()))
	vm.SetGlobal("xpcall", runtime.FunctionValue(vm.newXPCallFunction()))
}

// RegisterTestkitLib installs VM-aware testkit helpers for APIs that need to
// call or introspect ordinary VM closures. Pure runtime diagnostics stay in
// the runtime-provided testkit table.

func (vm *VM) RegisterTestkitLib() {
	val, ok := vm.globals["testkit"]
	if !ok || !val.IsTable() {
		return
	}
	lib := val.Table()
	lib.RawSetString("protect", runtime.FunctionValue(&runtime.GoFunction{
		Name: "testkit.protect",
		Fn: func(args []runtime.Value) ([]runtime.Value, error) {
			if len(args) < 1 {
				return nil, fmt.Errorf("bad argument #1 to 'testkit.protect' (function expected)")
			}
			if !args[0].IsFunction() {
				return nil, fmt.Errorf("bad argument #1 to 'testkit.protect' (function expected)")
			}
			results, err := vm.callValue(args[0], args[1:])
			out := runtime.NewTable()
			if err != nil {
				out.RawSetString("ok", runtime.BoolValue(false))
				out.RawSetString("error", protectedErrorValue(err))
				return []runtime.Value{runtime.TableValue(out)}, nil
			}
			values := runtime.NewTable()
			for i, result := range results {
				values.RawSet(runtime.IntValue(int64(i+1)), result)
			}
			out.RawSetString("ok", runtime.BoolValue(true))
			out.RawSetString("values", runtime.TableValue(values))
			out.RawSetString("n", runtime.IntValue(int64(len(results))))
			return []runtime.Value{runtime.TableValue(out)}, nil
		},
	}))
	lib.RawSetString("functionInfo", runtime.FunctionValue(&runtime.GoFunction{
		Name: "testkit.functionInfo",
		Fn: func(args []runtime.Value) ([]runtime.Value, error) {
			if len(args) < 1 || !args[0].IsFunction() {
				return nil, fmt.Errorf("bad argument #1 to 'testkit.functionInfo' (function expected)")
			}
			out := runtime.NewTable()
			out.RawSetString("type", runtime.StringValue("function"))
			out.RawSetString("raw", runtime.StringValue(fmt.Sprintf("0x%x", args[0].Raw())))
			out.RawSetString("identity", runtime.StringValue(fmt.Sprintf("function:%x", args[0].Raw())))
			if gf := args[0].GoFunction(); gf != nil {
				out.RawSetString("name", runtime.StringValue(gf.Name))
				out.RawSetString("kind", runtime.StringValue("native"))
				return []runtime.Value{runtime.TableValue(out)}, nil
			}
			if cl, ok := closureFromValue(args[0]); ok && cl != nil && cl.Proto != nil {
				name := cl.Proto.Name
				if name == "" {
					name = "<anonymous>"
				}
				out.RawSetString("name", runtime.StringValue(name))
				out.RawSetString("kind", runtime.StringValue("script"))
				if cl.Proto.Source != "" {
					out.RawSetString("sourceName", runtime.StringValue(cl.Proto.Source))
				}
				if cl.Proto.LineDefined > 0 {
					out.RawSetString("line", runtime.IntValue(int64(cl.Proto.LineDefined)))
					out.RawSetString("column", runtime.IntValue(1))
				}
				out.RawSetString("params", runtime.IntValue(int64(cl.Proto.NumParams)))
				out.RawSetString("vararg", runtime.BoolValue(cl.Proto.IsVarArg))
				out.RawSetString("upvalues", runtime.IntValue(int64(len(cl.Upvalues))))
				return []runtime.Value{runtime.TableValue(out)}, nil
			}
			out.RawSetString("name", runtime.StringValue("<unknown>"))
			out.RawSetString("kind", runtime.StringValue("unknown"))
			return []runtime.Value{runtime.TableValue(out)}, nil
		},
	}))
}

// RegisterToStringLib installs a VM-aware tostring builtin so __tostring
// metamethods implemented as VM closures can be invoked correctly.

func (vm *VM) RegisterToStringLib() {
	vm.SetGlobal("tostring", runtime.FunctionValue(&runtime.GoFunction{
		Name: "tostring",
		Fn: func(args []runtime.Value) ([]runtime.Value, error) {
			if len(args) == 0 {
				return nil, fmt.Errorf("bad argument #1 to 'tostring' (value expected)")
			}
			if args[0].IsInt() {
				if v, ok := runtime.CachedIntStringValue(args[0].Int()); ok {
					return []runtime.Value{v}, nil
				}
			}
			s, err := vm.luaToString(args[0])
			if err != nil {
				return nil, err
			}
			return []runtime.Value{runtime.StringValue(s)}, nil
		},
		FastArg1: func(arg runtime.Value) (runtime.Value, error) {
			if arg.IsInt() {
				if v, ok := runtime.CachedIntStringValue(arg.Int()); ok {
					return v, nil
				}
			}
			s, err := vm.luaToString(arg)
			if err != nil {
				return runtime.NilValue(), err
			}
			return runtime.StringValue(s), nil
		},
		Fast1: func(args []runtime.Value) (runtime.Value, error) {
			if len(args) == 0 {
				return runtime.NilValue(), fmt.Errorf("bad argument #1 to 'tostring' (value expected)")
			}
			if args[0].IsInt() {
				if v, ok := runtime.CachedIntStringValue(args[0].Int()); ok {
					return v, nil
				}
			}
			s, err := vm.luaToString(args[0])
			if err != nil {
				return runtime.NilValue(), err
			}
			return runtime.StringValue(s), nil
		},
	}))
}

// RegisterTypeLib installs a VM-aware type builtin that returns VM-owned,
// preboxed strings. This keeps the builtin allocation-free while preserving
// ordinary global override semantics through the normal GETGLOBAL guard.

func (vm *VM) RegisterTypeLib() {
	vm.SetGlobal("type", runtime.FunctionValue(vm.newTypeFunction()))
}

func (vm *VM) newTypeFunction() *runtime.GoFunction {
	return &runtime.GoFunction{
		Name: "type",
		Fn: func(args []runtime.Value) ([]runtime.Value, error) {
			if len(args) == 0 {
				return nil, fmt.Errorf("bad argument #1 to 'type' (value expected)")
			}
			return []runtime.Value{vm.typeNameValue(args[0])}, nil
		},
		FastArg1: func(arg runtime.Value) (runtime.Value, error) {
			return vm.typeNameValue(arg), nil
		},
		NativeKind: runtime.NativeKindStdType,
		NativeData: runtime.StdTypeIdentityPtr(),
	}
}

func (vm *VM) luaToString(v runtime.Value) (string, error) {
	if v.IsTable() {
		if mt := v.Table().GetMetatable(); mt != nil {
			if mm := mt.RawGetString("__tostring"); !mm.IsNil() {
				results, err := vm.callValue(mm, []runtime.Value{v})
				if err != nil {
					return "", err
				}
				if len(results) == 0 || !results[0].IsString() {
					return "", fmt.Errorf("'__tostring' must return a string")
				}
				return results[0].Str(), nil
			}
			if name := mt.RawGetString("__name"); name.IsString() {
				return name.Str() + ": " + strings.TrimPrefix(v.String(), "table: "), nil
			}
		}
	}
	return v.String(), nil
}

func protectedErrorValue(err error) runtime.Value {
	var luaErr *runtime.LuaError
	if errors.As(err, &luaErr) {
		return luaErr.Value
	}
	return runtime.StringValue(err.Error())
}

func (vm *VM) newPCallFunction() *runtime.GoFunction {
	return &runtime.GoFunction{
		Name: "pcall",
		Fn: func(args []runtime.Value) ([]runtime.Value, error) {
			if len(args) == 0 {
				return nil, fmt.Errorf("bad argument #1 to 'pcall' (value expected)")
			}
			results, err := vm.callValue(args[0], args[1:])
			if err != nil {
				return []runtime.Value{runtime.BoolValue(false), protectedErrorValue(err)}, nil
			}
			return append([]runtime.Value{runtime.BoolValue(true)}, results...), nil
		},
	}
}

func (vm *VM) newXPCallFunction() *runtime.GoFunction {
	return &runtime.GoFunction{
		Name: "xpcall",
		Fn: func(args []runtime.Value) ([]runtime.Value, error) {
			if len(args) < 2 {
				return nil, fmt.Errorf("bad argument #%d to 'xpcall' (value expected)", len(args)+1)
			}
			results, err := vm.callValue(args[0], args[2:])
			if err == nil {
				return append([]runtime.Value{runtime.BoolValue(true)}, results...), nil
			}
			handlerResults, handlerErr := vm.callValue(args[1], []runtime.Value{protectedErrorValue(err)})
			if handlerErr != nil {
				return []runtime.Value{runtime.BoolValue(false), protectedErrorValue(handlerErr)}, nil
			}
			msg := runtime.NilValue()
			if len(handlerResults) > 0 {
				msg = handlerResults[0]
			}
			return []runtime.Value{runtime.BoolValue(false), msg}, nil
		},
	}
}

// RegisterPairsLib installs a VM-aware pairs builtin so __pairs metamethods
// can be ordinary VM closures.

func (vm *VM) RegisterPairsLib() {
	vm.SetGlobal("pairs", runtime.FunctionValue(vm.newPairsFunction()))
}

// RegisterTableSortLib installs a VM-aware table.sort so file-loaded VM
// closures can be used as comparators.

func (vm *VM) RegisterTableSortLib() {
	tblVal, ok := vm.globals["table"]
	if !ok || !tblVal.IsTable() {
		return
	}
	tblVal.Table().RawSet(runtime.StringValue("sort"), runtime.FunctionValue(vm.newTableSortFunction()))
}

func (vm *VM) newTableSortFunction() *runtime.GoFunction {
	sortTable := func(t runtime.Value, comp runtime.Value, hasComp bool) error {
		length, err := vm.tableLenInt(t)
		if err != nil {
			return err
		}
		if length < 0 {
			length = 0
		}
		if !hasComp {
			if tbl := t.Table(); tbl != nil && tbl.TryPlainArraySort(length) {
				return nil
			}
		}
		elems := make([]runtime.Value, int(length))
		for i := 0; i < len(elems); i++ {
			v, err := vm.tableGet(t, runtime.IntValue(int64(i+1)))
			if err != nil {
				return err
			}
			elems[i] = v
		}

		var sortErr error
		if hasComp && comp.IsFunction() {
			sort.SliceStable(elems, func(a, b int) bool {
				if sortErr != nil {
					return false
				}
				results, err := vm.callValue(comp, []runtime.Value{elems[a], elems[b]})
				if err != nil {
					sortErr = err
					return false
				}
				if len(results) > 0 && results[0].Truthy() {
					reverse, err := vm.callValue(comp, []runtime.Value{elems[b], elems[a]})
					if err != nil {
						sortErr = err
						return false
					}
					if len(reverse) > 0 && reverse[0].Truthy() {
						sortErr = fmt.Errorf("invalid order function for sorting")
						return false
					}
					return true
				}
				return false
			})
		} else {
			sort.SliceStable(elems, func(a, b int) bool {
				if sortErr != nil {
					return false
				}
				less, err := vm.valueLessThan(elems[a], elems[b])
				if err != nil {
					sortErr = err
					return false
				}
				return less
			})
		}
		if sortErr != nil {
			return sortErr
		}
		for i, val := range elems {
			if err := vm.tableSet(t, runtime.IntValue(int64(i+1)), val); err != nil {
				return err
			}
		}
		return nil
	}
	return &runtime.GoFunction{
		Name: "table.sort",
		Fn: func(args []runtime.Value) ([]runtime.Value, error) {
			if len(args) < 1 || !args[0].IsTable() {
				return nil, fmt.Errorf("bad argument #1 to 'table.sort' (table expected)")
			}
			comp := runtime.NilValue()
			if len(args) >= 2 {
				comp = args[1]
			}
			if err := sortTable(args[0], comp, len(args) >= 2); err != nil {
				return nil, err
			}
			return nil, nil
		},
		FastArg1: func(t runtime.Value) (runtime.Value, error) {
			if !t.IsTable() {
				return runtime.NilValue(), fmt.Errorf("bad argument #1 to 'table.sort' (table expected)")
			}
			if err := sortTable(t, runtime.NilValue(), false); err != nil {
				return runtime.NilValue(), err
			}
			return runtime.NilValue(), nil
		},
		FastArg2: func(t, comp runtime.Value) (runtime.Value, error) {
			if !t.IsTable() {
				return runtime.NilValue(), fmt.Errorf("bad argument #1 to 'table.sort' (table expected)")
			}
			if err := sortTable(t, comp, true); err != nil {
				return runtime.NilValue(), err
			}
			return runtime.NilValue(), nil
		},
	}
}

// RegisterTableHigherOrderLib installs VM-aware table higher-order helpers so
// file-loaded VM closures can be used as callbacks.

func (vm *VM) RegisterTableHigherOrderLib() {
	tblVal, ok := vm.globals["table"]
	if !ok || !tblVal.IsTable() {
		return
	}
	tbl := tblVal.Table()
	tbl.RawSet(runtime.StringValue("filter"), runtime.FunctionValue(vm.newTableFilterFunction()))
	tbl.RawSet(runtime.StringValue("map"), runtime.FunctionValue(vm.newTableMapFunction()))
	tbl.RawSet(runtime.StringValue("reduce"), runtime.FunctionValue(vm.newTableReduceFunction()))
	tbl.RawSet(runtime.StringValue("fromArray"), runtime.FunctionValue(vm.newTableFromArrayFunction()))
}

func (vm *VM) newTableFilterFunction() *runtime.GoFunction {
	return &runtime.GoFunction{
		Name: "table.filter",
		Fn: func(args []runtime.Value) ([]runtime.Value, error) {
			if len(args) < 2 || !args[0].IsTable() || !args[1].IsFunction() {
				return nil, fmt.Errorf("bad argument to 'table.filter'")
			}
			src := args[0].Table()
			fn := args[1]
			result := runtime.NewTable()
			out := int64(1)
			for i := int64(1); i <= int64(src.Length()); i++ {
				v := src.RawGet(runtime.IntValue(i))
				results, err := vm.callValue(fn, []runtime.Value{v, runtime.IntValue(i)})
				if err != nil {
					return nil, err
				}
				if len(results) > 0 && results[0].Truthy() {
					result.RawSet(runtime.IntValue(out), v)
					out++
				}
			}
			return []runtime.Value{runtime.TableValue(result)}, nil
		},
	}
}

func (vm *VM) newTableMapFunction() *runtime.GoFunction {
	return &runtime.GoFunction{
		Name: "table.map",
		Fn: func(args []runtime.Value) ([]runtime.Value, error) {
			if len(args) < 2 || !args[0].IsTable() || !args[1].IsFunction() {
				return nil, fmt.Errorf("bad argument to 'table.map'")
			}
			src := args[0].Table()
			fn := args[1]
			result := runtime.NewTable()
			for i := int64(1); i <= int64(src.Length()); i++ {
				v := src.RawGet(runtime.IntValue(i))
				results, err := vm.callValue(fn, []runtime.Value{v, runtime.IntValue(i)})
				if err != nil {
					return nil, err
				}
				if len(results) > 0 {
					result.RawSet(runtime.IntValue(i), results[0])
				} else {
					result.RawSet(runtime.IntValue(i), runtime.NilValue())
				}
			}
			return []runtime.Value{runtime.TableValue(result)}, nil
		},
	}
}

func (vm *VM) newTableReduceFunction() *runtime.GoFunction {
	return &runtime.GoFunction{
		Name: "table.reduce",
		Fn: func(args []runtime.Value) ([]runtime.Value, error) {
			if len(args) < 3 || !args[0].IsTable() || !args[1].IsFunction() {
				return nil, fmt.Errorf("bad argument to 'table.reduce'")
			}
			src := args[0].Table()
			fn := args[1]
			acc := args[2]
			for i := int64(1); i <= int64(src.Length()); i++ {
				v := src.RawGet(runtime.IntValue(i))
				results, err := vm.callValue(fn, []runtime.Value{acc, v})
				if err != nil {
					return nil, err
				}
				if len(results) > 0 {
					acc = results[0]
				}
			}
			return []runtime.Value{acc}, nil
		},
	}
}

func (vm *VM) newTableFromArrayFunction() *runtime.GoFunction {
	return &runtime.GoFunction{
		Name: "table.fromArray",
		Fn: func(args []runtime.Value) ([]runtime.Value, error) {
			if len(args) < 2 || !args[0].IsTable() || !args[1].IsFunction() {
				return nil, fmt.Errorf("bad argument to 'table.fromArray'")
			}
			src := args[0].Table()
			fn := args[1]
			result := runtime.NewTable()
			for i := int64(1); i <= int64(src.Length()); i++ {
				v := src.RawGet(runtime.IntValue(i))
				keys, err := vm.callValue(fn, []runtime.Value{v})
				if err != nil {
					return nil, err
				}
				if len(keys) > 0 {
					result.RawSet(keys[0], v)
				}
			}
			return []runtime.Value{runtime.TableValue(result)}, nil
		},
	}
}

// RegisterSortCallbackLib installs VM-aware sort namespace helpers whose
// callbacks may be file-loaded VM closures.

func (vm *VM) RegisterSortCallbackLib() {
	sortVal, ok := vm.globals["sort"]
	if !ok || !sortVal.IsTable() {
		return
	}
	tbl := sortVal.Table()
	tbl.RawSet(runtime.StringValue("by"), runtime.FunctionValue(vm.newSortByFunction()))
	tbl.RawSet(runtime.StringValue("byKey"), runtime.FunctionValue(vm.newSortByKeyFunction()))
	tbl.RawSet(runtime.StringValue("partition"), runtime.FunctionValue(vm.newSortPartitionFunction()))
	tbl.RawSet(runtime.StringValue("min"), runtime.FunctionValue(vm.newSortMinMaxFunction(false)))
	tbl.RawSet(runtime.StringValue("max"), runtime.FunctionValue(vm.newSortMinMaxFunction(true)))
}

func sortArrayValues(tbl *runtime.Table) []runtime.Value {
	length := tbl.Length()
	elems := make([]runtime.Value, length)
	for i := 0; i < length; i++ {
		elems[i] = tbl.RawGet(runtime.IntValue(int64(i + 1)))
	}
	return elems
}

func writeSortArrayValues(tbl *runtime.Table, elems []runtime.Value) {
	for i, v := range elems {
		tbl.RawSet(runtime.IntValue(int64(i+1)), v)
	}
}

func (vm *VM) newSortByFunction() *runtime.GoFunction {
	return &runtime.GoFunction{
		Name: "sort.by",
		Fn: func(args []runtime.Value) ([]runtime.Value, error) {
			if len(args) < 2 || !args[0].IsTable() || !args[1].IsFunction() {
				return nil, fmt.Errorf("bad arguments to 'sort.by' (table and function expected)")
			}
			tbl := args[0].Table()
			fn := args[1]
			elems := sortArrayValues(tbl)
			var sortErr error
			sort.SliceStable(elems, func(a, b int) bool {
				if sortErr != nil {
					return false
				}
				results, err := vm.callValue(fn, []runtime.Value{elems[a], elems[b]})
				if err != nil {
					sortErr = err
					return false
				}
				return len(results) > 0 && results[0].Truthy()
			})
			if sortErr != nil {
				return nil, sortErr
			}
			writeSortArrayValues(tbl, elems)
			return []runtime.Value{args[0]}, nil
		},
	}
}

func (vm *VM) newSortByKeyFunction() *runtime.GoFunction {
	return &runtime.GoFunction{
		Name: "sort.byKey",
		Fn: func(args []runtime.Value) ([]runtime.Value, error) {
			if len(args) < 2 || !args[0].IsTable() || !args[1].IsFunction() {
				return nil, fmt.Errorf("bad arguments to 'sort.byKey' (table and function expected)")
			}
			tbl := args[0].Table()
			fn := args[1]
			elems := sortArrayValues(tbl)
			type keyedValue struct {
				value runtime.Value
				key   runtime.Value
			}
			pairs := make([]keyedValue, len(elems))
			for i, elem := range elems {
				results, err := vm.callValue(fn, []runtime.Value{elem})
				if err != nil {
					return nil, err
				}
				pairs[i].value = elem
				if len(results) > 0 {
					pairs[i].key = results[0]
				} else {
					pairs[i].key = runtime.NilValue()
				}
			}
			var sortErr error
			sort.SliceStable(pairs, func(a, b int) bool {
				if sortErr != nil {
					return false
				}
				less, err := vm.valueLessThan(pairs[a].key, pairs[b].key)
				if err != nil {
					sortErr = err
					return false
				}
				return less
			})
			if sortErr != nil {
				return nil, sortErr
			}
			for i, pair := range pairs {
				elems[i] = pair.value
			}
			writeSortArrayValues(tbl, elems)
			return []runtime.Value{args[0]}, nil
		},
	}
}

func (vm *VM) newSortPartitionFunction() *runtime.GoFunction {
	return &runtime.GoFunction{
		Name: "sort.partition",
		Fn: func(args []runtime.Value) ([]runtime.Value, error) {
			if len(args) < 2 || !args[0].IsTable() || !args[1].IsFunction() {
				return nil, fmt.Errorf("bad arguments to 'sort.partition' (table and function expected)")
			}
			src := args[0].Table()
			fn := args[1]
			truthy := runtime.NewTable()
			falsy := runtime.NewTable()
			trueIdx := int64(1)
			falseIdx := int64(1)
			for i := int64(1); i <= int64(src.Length()); i++ {
				v := src.RawGet(runtime.IntValue(i))
				results, err := vm.callValue(fn, []runtime.Value{v})
				if err != nil {
					return nil, err
				}
				if len(results) > 0 && results[0].Truthy() {
					truthy.RawSet(runtime.IntValue(trueIdx), v)
					trueIdx++
				} else {
					falsy.RawSet(runtime.IntValue(falseIdx), v)
					falseIdx++
				}
			}
			return []runtime.Value{runtime.TableValue(truthy), runtime.TableValue(falsy)}, nil
		},
	}
}

func (vm *VM) newSortMinMaxFunction(max bool) *runtime.GoFunction {
	name := "sort.min"
	if max {
		name = "sort.max"
	}
	return &runtime.GoFunction{
		Name: name,
		Fn: func(args []runtime.Value) ([]runtime.Value, error) {
			if len(args) < 1 || !args[0].IsTable() {
				return nil, fmt.Errorf("bad argument #1 to '%s' (table expected)", name)
			}
			src := args[0].Table()
			length := src.Length()
			if length == 0 {
				return []runtime.Value{runtime.NilValue()}, nil
			}
			hasKeyFn := len(args) >= 2 && args[1].IsFunction()
			best := src.RawGet(runtime.IntValue(1))
			bestKey := best
			if hasKeyFn {
				results, err := vm.callValue(args[1], []runtime.Value{best})
				if err != nil {
					return nil, err
				}
				if len(results) > 0 {
					bestKey = results[0]
				}
			}
			for i := int64(2); i <= int64(length); i++ {
				candidate := src.RawGet(runtime.IntValue(i))
				candidateKey := candidate
				if hasKeyFn {
					results, err := vm.callValue(args[1], []runtime.Value{candidate})
					if err != nil {
						return nil, err
					}
					if len(results) > 0 {
						candidateKey = results[0]
					}
				}
				less, err := vm.valueLessThan(candidateKey, bestKey)
				if err != nil {
					return nil, err
				}
				if !max && less {
					best = candidate
					bestKey = candidateKey
					continue
				}
				if max {
					reverseLess, err := vm.valueLessThan(bestKey, candidateKey)
					if err != nil {
						return nil, err
					}
					if reverseLess {
						best = candidate
						bestKey = candidateKey
					}
				}
			}
			return []runtime.Value{best}, nil
		},
	}
}

// RegisterTableProxyLib installs VM-aware table functions that honor __index,
// __newindex, and __len for proxy tables.

func (vm *VM) RegisterTableProxyLib() {
	tblVal, ok := vm.globals["table"]
	if !ok || !tblVal.IsTable() {
		return
	}
	tbl := tblVal.Table()
	insert := func(t, posValue, value runtime.Value, hasPos bool) error {
		if !t.IsTable() {
			return fmt.Errorf("bad argument #1 to 'table.insert' (table expected)")
		}
		length, err := vm.tableLenInt(t)
		if err != nil {
			return err
		}
		if !hasPos {
			if t.Table().TryPlainArrayInsert(int64(length+1), value) {
				return nil
			}
			return vm.tableSet(t, runtime.IntValue(length+1), value)
		}
		pos := vmToInt(posValue)
		if pos < 1 || pos > length+1 {
			return fmt.Errorf("bad argument #2 to 'table.insert' (position out of bounds)")
		}
		if t.Table().TryPlainArrayInsert(pos, value) {
			return nil
		}
		for i := length; i >= pos; i-- {
			v, err := vm.tableGet(t, runtime.IntValue(i))
			if err != nil {
				return err
			}
			if err := vm.tableSet(t, runtime.IntValue(i+1), v); err != nil {
				return err
			}
		}
		return vm.tableSet(t, runtime.IntValue(pos), value)
	}
	remove := func(t, posValue runtime.Value, hasPos bool) (runtime.Value, error) {
		if !t.IsTable() {
			return runtime.NilValue(), fmt.Errorf("bad argument #1 to 'table.remove' (table expected)")
		}
		length, err := vm.tableLenInt(t)
		if err != nil {
			return runtime.NilValue(), err
		}
		pos := length
		if hasPos {
			pos = vmToInt(posValue)
		}
		if pos < 0 || pos > length+1 || (pos == 0 && length > 0) {
			return runtime.NilValue(), fmt.Errorf("bad argument #2 to 'table.remove' (position out of bounds)")
		}
		if pos == length+1 {
			return runtime.NilValue(), nil
		}
		if removed, ok := t.Table().TryPlainArrayRemove(pos); ok {
			return removed, nil
		}
		removed, err := vm.tableGet(t, runtime.IntValue(pos))
		if err != nil {
			return runtime.NilValue(), err
		}
		for i := pos; i < length; i++ {
			v, err := vm.tableGet(t, runtime.IntValue(i+1))
			if err != nil {
				return runtime.NilValue(), err
			}
			if err := vm.tableSet(t, runtime.IntValue(i), v); err != nil {
				return runtime.NilValue(), err
			}
		}
		if err := vm.tableSet(t, runtime.IntValue(length), runtime.NilValue()); err != nil {
			return runtime.NilValue(), err
		}
		return removed, nil
	}
	tbl.RawSet(runtime.StringValue("insert"), runtime.FunctionValue(&runtime.GoFunction{
		Name: "table.insert",
		Fn: func(args []runtime.Value) ([]runtime.Value, error) {
			if len(args) < 1 || !args[0].IsTable() {
				return nil, fmt.Errorf("bad argument #1 to 'table.insert' (table expected)")
			}
			if len(args) != 2 && len(args) != 3 {
				return nil, fmt.Errorf("wrong number of arguments to 'table.insert'")
			}
			if len(args) == 2 {
				return nil, insert(args[0], runtime.NilValue(), args[1], false)
			}
			return nil, insert(args[0], args[1], args[2], true)
		},
		FastArg2: func(t, value runtime.Value) (runtime.Value, error) {
			if err := insert(t, runtime.NilValue(), value, false); err != nil {
				return runtime.NilValue(), err
			}
			return runtime.NilValue(), nil
		},
		FastArg3: func(t, pos, value runtime.Value) (runtime.Value, error) {
			if err := insert(t, pos, value, true); err != nil {
				return runtime.NilValue(), err
			}
			return runtime.NilValue(), nil
		},
	}))
	tbl.RawSet(runtime.StringValue("remove"), runtime.FunctionValue(&runtime.GoFunction{
		Name: "table.remove",
		Fn: func(args []runtime.Value) ([]runtime.Value, error) {
			if len(args) < 1 {
				return nil, fmt.Errorf("bad argument #1 to 'table.remove' (table expected)")
			}
			pos := runtime.NilValue()
			if len(args) >= 2 {
				pos = args[1]
			}
			removed, err := remove(args[0], pos, len(args) >= 2)
			if err != nil {
				return nil, err
			}
			return []runtime.Value{removed}, nil
		},
		FastArg1: func(t runtime.Value) (runtime.Value, error) {
			return remove(t, runtime.NilValue(), false)
		},
		FastArg2: func(t, pos runtime.Value) (runtime.Value, error) {
			return remove(t, pos, true)
		},
	}))
	tbl.RawSet(runtime.StringValue("concat"), runtime.FunctionValue(&runtime.GoFunction{
		Name: "table.concat",
		Fn: func(args []runtime.Value) ([]runtime.Value, error) {
			if len(args) < 1 || !args[0].IsTable() {
				return nil, fmt.Errorf("bad argument #1 to 'table.concat' (table expected)")
			}
			t := args[0]
			sep := ""
			if len(args) >= 2 && args[1].IsString() {
				sep = args[1].Str()
			}
			i := int64(1)
			j, err := vm.tableLenInt(t)
			if err != nil {
				return nil, err
			}
			if len(args) >= 3 {
				i = vmToInt(args[2])
			}
			if len(args) >= 4 {
				j = vmToInt(args[3])
			}
			var b strings.Builder
			for k := i; k <= j; k++ {
				v, err := vm.tableGet(t, runtime.IntValue(k))
				if err != nil {
					return nil, err
				}
				s, ok := runtime.ConcatOperandString(v)
				if !ok {
					return nil, fmt.Errorf("invalid value at index %d in table for 'concat'", k)
				}
				if k > i {
					b.WriteString(sep)
				}
				b.WriteString(s)
			}
			return []runtime.Value{runtime.StringValue(b.String())}, nil
		},
	}))
	tableUnpack := func(name string, args []runtime.Value) ([]runtime.Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'table.%s' (table expected)", name)
		}
		t := args[0]
		i := int64(1)
		j, err := vm.tableLenInt(t)
		if err != nil {
			return nil, err
		}
		if len(args) >= 2 {
			i = vmToInt(args[1])
		}
		if len(args) >= 3 {
			j = vmToInt(args[2])
		}
		count, err := runtime.CheckTableUnpackRange(name, i, j)
		if err != nil {
			return nil, err
		}
		result := make([]runtime.Value, 0, count)
		for k := i; k <= j; k++ {
			v, err := vm.tableGet(t, runtime.IntValue(k))
			if err != nil {
				return nil, err
			}
			result = append(result, v)
		}
		return result, nil
	}
	tbl.RawSet(runtime.StringValue("unpack"), runtime.FunctionValue(&runtime.GoFunction{Name: "table.unpack", Fn: func(args []runtime.Value) ([]runtime.Value, error) {
		return tableUnpack("unpack", args)
	}}))
	tbl.RawSet(runtime.StringValue("spread"), runtime.FunctionValue(&runtime.GoFunction{Name: "table.spread", Fn: func(args []runtime.Value) ([]runtime.Value, error) {
		return tableUnpack("spread", args)
	}}))
	tableMove := func(src, first, last, target, dstArg runtime.Value, hasDst bool) (runtime.Value, error) {
		if !src.IsTable() {
			return runtime.NilValue(), fmt.Errorf("bad argument to 'table.move'")
		}
		f := vmToInt(first)
		e := vmToInt(last)
		tPos := vmToInt(target)
		dst := src
		if hasDst {
			if !dstArg.IsTable() {
				return runtime.NilValue(), fmt.Errorf("bad argument to 'table.move'")
			}
			dst = dstArg
		}
		if e >= f {
			if dst.Table().TryPlainArrayMove(src.Table(), f, e, tPos) {
				return dst, nil
			}
			if handled, result, err := vm.tryForwardingProxyTableMove(src, dst, f, e, tPos); handled || err != nil {
				return result, err
			}
			count := e - f + 1
			if tPos <= f || src.Table() != dst.Table() {
				for i := int64(0); i < count; i++ {
					v, err := vm.tableGet(src, runtime.IntValue(f+i))
					if err != nil {
						return runtime.NilValue(), err
					}
					if err := vm.tableSet(dst, runtime.IntValue(tPos+i), v); err != nil {
						return runtime.NilValue(), err
					}
				}
			} else {
				for i := count - 1; i >= 0; i-- {
					v, err := vm.tableGet(src, runtime.IntValue(f+i))
					if err != nil {
						return runtime.NilValue(), err
					}
					if err := vm.tableSet(dst, runtime.IntValue(tPos+i), v); err != nil {
						return runtime.NilValue(), err
					}
				}
			}
		}
		return dst, nil
	}
	tbl.RawSet(runtime.StringValue("move"), runtime.FunctionValue(&runtime.GoFunction{
		Name: "table.move",
		Fn: func(args []runtime.Value) ([]runtime.Value, error) {
			if len(args) < 4 {
				return nil, fmt.Errorf("bad argument to 'table.move'")
			}
			dstArg := runtime.NilValue()
			if len(args) >= 5 {
				dstArg = args[4]
			}
			dst, err := tableMove(args[0], args[1], args[2], args[3], dstArg, len(args) >= 5)
			if err != nil {
				return nil, err
			}
			return []runtime.Value{dst}, nil
		},
		FastArg4: func(src, first, last, target runtime.Value) (runtime.Value, error) {
			return tableMove(src, first, last, target, runtime.NilValue(), false)
		},
		FastArg5: func(src, first, last, target, dst runtime.Value) (runtime.Value, error) {
			return tableMove(src, first, last, target, dst, true)
		},
	}))
}

// RegisterIPairsLib installs a VM-aware ipairs builtin so ordinary indexing
// during iteration can invoke VM __index closures.

func (vm *VM) RegisterIPairsLib() {
	if vm.ipairsIteratorFn == nil {
		vm.ipairsIteratorFn = vm.newIPairsIteratorFunction()
	}
	vm.SetGlobal("ipairs", runtime.FunctionValue(vm.newIPairsFunction()))
}

func (vm *VM) newIPairsFunction() *runtime.GoFunction {
	return &runtime.GoFunction{
		Name: "ipairs",
		Fn: func(args []runtime.Value) ([]runtime.Value, error) {
			if len(args) < 1 || !args[0].IsTable() {
				return nil, fmt.Errorf("bad argument #1 to 'ipairs' (table expected)")
			}
			return []runtime.Value{runtime.FunctionValue(vm.ipairsIteratorFn), args[0], runtime.IntValue(0)}, nil
		},
		NativeKind: runtime.NativeKindStdIPairs,
		NativeData: runtime.StdIPairsIdentityPtr(),
	}
}

func (vm *VM) newIPairsIteratorFunction() *runtime.GoFunction {
	return &runtime.GoFunction{
		Name: "ipairs_iterator",
		Fn: func(args []runtime.Value) ([]runtime.Value, error) {
			if len(args) < 1 || !args[0].IsTable() {
				return nil, fmt.Errorf("bad argument #1 to 'for iterator' (table expected)")
			}
			i := int64(0)
			if len(args) >= 2 && !args[1].IsNil() {
				if args[1].IsInt() {
					i = args[1].Int()
				} else if args[1].IsFloat() {
					i = int64(args[1].Float())
				} else {
					return nil, fmt.Errorf("bad argument #2 to 'for iterator' (number expected)")
				}
			}
			i++
			key := runtime.IntValue(i)
			v, err := vm.tableGet(args[0], key)
			if err != nil {
				return nil, err
			}
			if v.IsNil() {
				return []runtime.Value{runtime.NilValue()}, nil
			}
			return []runtime.Value{key, v}, nil
		},
		FastArg2Ret2: func(table, keyValue runtime.Value) (runtime.Value, runtime.Value, int, error) {
			if !table.IsTable() {
				return runtime.NilValue(), runtime.NilValue(), 0, fmt.Errorf("bad argument #1 to 'for iterator' (table expected)")
			}
			i := int64(0)
			if !keyValue.IsNil() {
				if keyValue.IsInt() {
					i = keyValue.Int()
				} else if keyValue.IsFloat() {
					i = int64(keyValue.Float())
				} else {
					return runtime.NilValue(), runtime.NilValue(), 0, fmt.Errorf("bad argument #2 to 'for iterator' (number expected)")
				}
			}
			i++
			key := runtime.IntValue(i)
			var v runtime.Value
			if tbl := table.Table(); tbl.GetMetatable() == nil {
				v = tbl.RawGetInt(i)
			} else {
				var err error
				v, err = vm.tableGet(table, key)
				if err != nil {
					return runtime.NilValue(), runtime.NilValue(), 0, err
				}
			}
			if v.IsNil() {
				return runtime.NilValue(), runtime.NilValue(), 1, nil
			}
			return key, v, 2, nil
		},
	}
}

func (vm *VM) newPairsFunction() *runtime.GoFunction {
	return &runtime.GoFunction{
		Name: "pairs",
		Fn: func(args []runtime.Value) ([]runtime.Value, error) {
			if len(args) < 1 || !args[0].IsTable() {
				return nil, fmt.Errorf("bad argument #1 to 'pairs' (table expected)")
			}
			tbl := args[0].Table()
			if mt := tbl.GetMetatable(); mt != nil {
				mm := mt.RawGetString("__pairs")
				if !mm.IsNil() {
					if cl, ok := closureFromValue(mm); ok && vm.activeCoroutine() != nil && protoContainsOp(cl.Proto, OP_YIELD) {
						return nil, fmt.Errorf("__pairs cannot yield through the host pairs() setup; return a coroutine-backed iterator instead")
					}
					if cl, ok := closureFromValue(mm); ok {
						newBase := vm.top
						if vm.frameCount > 0 {
							curFrame := &vm.frames[vm.frameCount-1]
							minBase := curFrame.base + curFrame.closure.Proto.MaxStack
							if newBase < minBase {
								newBase = minBase
							}
						}
						return vm.call(cl, []runtime.Value{args[0]}, newBase, -1)
					}
					return vm.callValue(mm, []runtime.Value{args[0]})
				}
			}
			return []runtime.Value{runtime.FunctionValue(vm.newPairsIteratorFunction(tbl)), args[0], runtime.NilValue()}, nil
		},
		NativeKind: runtime.NativeKindStdPairs,
		NativeData: runtime.StdPairsIdentityPtr(),
	}
}

func (vm *VM) newPairsIteratorFunction(tbl *runtime.Table) *runtime.GoFunction {
	keys := tbl.PairsKeysSnapshot()
	idx := 0
	return &runtime.GoFunction{
		Name: "pairs_iterator",
		Fn: func(_ []runtime.Value) ([]runtime.Value, error) {
			if idx >= len(keys) {
				return []runtime.Value{runtime.NilValue()}, nil
			}
			k := keys[idx]
			idx++
			return []runtime.Value{k, tbl.RawGet(k)}, nil
		},
		FastArg2Ret2: func(_, _ runtime.Value) (runtime.Value, runtime.Value, int, error) {
			if idx >= len(keys) {
				return runtime.NilValue(), runtime.NilValue(), 1, nil
			}
			k := keys[idx]
			idx++
			return k, tbl.RawGet(k), 2, nil
		},
	}
}

// PrepareTier2GlobalArray resolves the requested string constants as indexed
// globals and returns the data needed by the Tier 2 indexed-global fast path.
// The native path is enabled only for single-threaded VMs without per-VM
// overrides; other VM shapes fall back to the existing exit-resume protocol.

func (vm *VM) registerChannelBuiltins() {
	vm.SetGlobal("close", runtime.FunctionValue(&runtime.GoFunction{
		Name: "close",
		Fn: func(args []runtime.Value) ([]runtime.Value, error) {
			if len(args) < 1 || !args[0].IsChannel() {
				return nil, fmt.Errorf("close expects a channel")
			}
			ch := args[0].Channel()
			if err := ch.Close(); err != nil {
				return nil, err
			}
			return nil, nil
		},
	}))
}

// SetStringMeta sets the string metatable.

func (vm *VM) SetStringMeta(meta *runtime.Table) {
	vm.stringMeta = meta
}

// RegisterStringLib installs VM-aware string callbacks such as function-valued
// string.gsub replacements.

func (vm *VM) RegisterStringLib() {
	var strLib *runtime.Table
	if existing, ok := vm.globals["string"]; ok && existing.IsTable() {
		strLib = runtime.RefreshStringLibWithCaller(existing.Table(), vm.callValue)
	} else {
		strLib = runtime.BuildStringLibWithCaller(vm.callValue)
		vm.SetGlobal("string", runtime.TableValue(strLib))
	}
	meta := runtime.NewTable()
	meta.RawSet(runtime.StringValue("__index"), runtime.TableValue(strLib))
	vm.stringMeta = meta
	vm.setPackageLoaded("string", runtime.TableValue(strLib))
}

func (vm *VM) RegisterHTTPLib() {
	httpLib := runtime.TableValue(runtime.BuildHTTPLibWithCaller(vm.callValue))
	vm.SetGlobal("http", httpLib)
	vm.setPackageLoaded("http", httpLib)
}

func (vm *VM) RegisterScriptLib() {
	t := runtime.NewTable()
	set := func(name string, fn func([]runtime.Value) ([]runtime.Value, error)) {
		t.RawSetString(name, runtime.FunctionValue(&runtime.GoFunction{Name: "script." + name, Fn: fn}))
	}
	set("env", func(args []runtime.Value) ([]runtime.Value, error) {
		seed := runtime.NewTable()
		if len(args) >= 1 && !args[0].IsNil() {
			if !args[0].IsTable() {
				return nil, fmt.Errorf("bad argument #1 to 'script.env' (table expected)")
			}
			seed = args[0].Table()
		}
		return []runtime.Value{runtime.TableValue(vmScriptEnvOptions(seed, false))}, nil
	})
	set("sandbox", func(args []runtime.Value) ([]runtime.Value, error) {
		seed := runtime.NewTable()
		if len(args) >= 1 && !args[0].IsNil() {
			if !args[0].IsTable() {
				return nil, fmt.Errorf("bad argument #1 to 'script.sandbox' (table expected)")
			}
			seed = args[0].Table()
		}
		return []runtime.Value{runtime.TableValue(vmScriptEnvOptions(seed, true))}, nil
	})
	set("compile", func(args []runtime.Value) ([]runtime.Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'script.compile' (string expected)")
		}
		opt := runtime.NilValue()
		if len(args) >= 2 {
			opt = args[1]
		}
		return vm.compileScriptChunk(args[0].Str(), opt, "<script.compile>")
	})
	set("eval", func(args []runtime.Value) ([]runtime.Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'script.eval' (string expected)")
		}
		opt := runtime.NilValue()
		if len(args) >= 2 {
			opt = args[1]
		}
		fn, err := vm.compileScriptChunk(args[0].Str(), opt, "<script.eval>")
		if err != nil {
			return nil, err
		}
		return vm.callValue(fn[0], nil)
	})
	set("loadFile", func(args []runtime.Value) ([]runtime.Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'script.loadFile' (string expected)")
		}
		opt := runtime.NilValue()
		if len(args) >= 2 {
			opt = args[1]
		}
		return vm.loadScriptFile(args[0].Str(), opt)
	})
	set("runFile", func(args []runtime.Value) ([]runtime.Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'script.runFile' (string expected)")
		}
		opt := runtime.NilValue()
		if len(args) >= 2 {
			opt = args[1]
		}
		fn, err := vm.loadScriptFile(args[0].Str(), opt)
		if err != nil {
			return nil, err
		}
		return vm.callValue(fn[0], nil)
	})
	set("dir", func(args []runtime.Value) ([]runtime.Value, error) {
		return []runtime.Value{runtime.StringValue(vm.scriptDir)}, nil
	})
	set("setDir", func(args []runtime.Value) ([]runtime.Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'script.setDir' (string expected)")
		}
		old := vm.scriptDir
		vm.scriptDir = args[0].Str()
		return []runtime.Value{runtime.StringValue(old)}, nil
	})
	val := runtime.TableValue(t)
	vm.SetGlobal("script", val)
	vm.setPackageLoaded("script", val)
}

func (vm *VM) RegisterLoaderLib() {
	vm.SetGlobal("load", runtime.FunctionValue(&runtime.GoFunction{Name: "load", Fn: func(args []runtime.Value) ([]runtime.Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'load' (string expected)")
		}
		opt := runtime.NilValue()
		if len(args) >= 2 {
			opt = args[1]
		}
		fn, err := vm.compileScriptChunk(args[0].Str(), opt, "<load>")
		if err != nil {
			return []runtime.Value{runtime.NilValue(), runtime.StringValue(err.Error())}, nil
		}
		return fn, nil
	}}))
	vm.SetGlobal("loadstring", vm.GetGlobal("load"))
	vm.SetGlobal("loadfile", runtime.FunctionValue(&runtime.GoFunction{Name: "loadfile", Fn: func(args []runtime.Value) ([]runtime.Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'loadfile' (string expected)")
		}
		opt := runtime.NilValue()
		if len(args) >= 2 {
			opt = args[1]
		}
		fn, err := vm.loadScriptFile(args[0].Str(), opt)
		if err != nil {
			return []runtime.Value{runtime.NilValue(), runtime.StringValue(err.Error())}, nil
		}
		return fn, nil
	}}))
	vm.SetGlobal("dofile", runtime.FunctionValue(&runtime.GoFunction{Name: "dofile", Fn: func(args []runtime.Value) ([]runtime.Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'dofile' (string expected)")
		}
		fn, err := vm.loadScriptFile(args[0].Str(), runtime.NilValue())
		if err != nil {
			return nil, err
		}
		return vm.callValue(fn[0], nil)
	}}))
	vm.SetGlobal("require", runtime.FunctionValue(&runtime.GoFunction{Name: "require", Fn: func(args []runtime.Value) ([]runtime.Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'require' (string expected)")
		}
		name := args[0].Str()
		if loaded := vm.packageLoaded(name); !loaded.IsNil() {
			return []runtime.Value{loaded}, nil
		}
		if module := vm.GetGlobal(name); module.IsTable() || module.IsFunction() {
			vm.setPackageLoaded(name, module)
			return []runtime.Value{module}, nil
		}
		filename := vm.resolveScriptPath(strings.ReplaceAll(name, ".", "/") + ".gs")
		if _, err := os.Stat(filename); err != nil {
			return nil, fmt.Errorf("module '%s' not found", name)
		}
		fn, err := vm.loadScriptFile(filename, runtime.NilValue())
		if err != nil {
			return nil, err
		}
		results, err := vm.callValue(fn[0], nil)
		if err != nil {
			return nil, err
		}
		module := runtime.BoolValue(true)
		if len(results) > 0 {
			module = results[0]
		}
		vm.setPackageLoaded(name, module)
		return []runtime.Value{module}, nil
	}}))
}

func (vm *VM) packageLoaded(name string) runtime.Value {
	pkg := vm.GetGlobal("package")
	if !pkg.IsTable() {
		return runtime.NilValue()
	}
	loaded := pkg.Table().RawGetString("loaded")
	if !loaded.IsTable() {
		return runtime.NilValue()
	}
	return loaded.Table().RawGetString(name)
}
