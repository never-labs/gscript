package vm

// Standard-library registration and lib-builder helpers, split verbatim from vm.go.

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/never-labs/gscript/internal/runtime"
	"github.com/never-labs/gscript/internal/stdlib/catalog"
	tablelib "github.com/never-labs/gscript/internal/stdlib/table"
	syncrt "github.com/never-labs/gscript/internal/stdlibrt/concurrency"
	"github.com/never-labs/gscript/internal/stdlibrt/host"
	stdlibinstall "github.com/never-labs/gscript/internal/stdlibrt/install"
	llmrt "github.com/never-labs/gscript/internal/stdlibrt/llm"
	"github.com/never-labs/gscript/internal/stdlibrt/modules"
)

func (vm *VM) RestrictStdlib(allowed map[string]bool) {
	for _, name := range catalog.ModuleNames() {
		if allowed[name] {
			continue
		}
		vm.DeleteGlobal(name)
		vm.setPackageLoaded(name, runtime.NilValue())
		if name == "string" {
			vm.stringMeta = nil
		}
	}
	if !allowed["llm"] {
		vm.DeleteGlobal("toolof")
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

type vmStdlibInstallContext struct {
	vm *VM
}

func (vm *VM) newStdlibInstallContext() *vmStdlibInstallContext {
	return &vmStdlibInstallContext{vm: vm}
}

func (ctx *vmStdlibInstallContext) RegisterModule(name string, module runtime.Value) {
	ctx.vm.SetGlobal(name, module)
	ctx.vm.setPackageLoaded(name, module)
}

func (ctx *vmStdlibInstallContext) RegisterTable(name string, table *runtime.Table) {
	ctx.RegisterModule(name, runtime.TableValue(table))
}

func (ctx *vmStdlibInstallContext) RegisterAlias(name string, value runtime.Value) {
	ctx.vm.SetGlobal(name, value)
}

func (vm *VM) RegisterStdlibRuntimeModules() {
	stdlibinstall.InstallModules(vm.newStdlibInstallContext(), func() int64 {
		return vm.maxHostResult
	}, stdlibinstall.ModuleOptions{
		ScriptCaller: vm.callValue,
		Less:         vm.valueLessThan,
		SkipTable:    true,
		Host: host.Options{
			SkipHostIO:     true,
			NetworkAllowed: func() bool { return vm.networkAccess },
			MaxHostResult:  func() int64 { return vm.maxHostResult },
			Call:           vm.callValue,
		},
	})
}

// RegisterProtectedCallLib installs VM-aware pcall/xpcall builtins so protected
// calls can invoke ordinary VM closures.

func (vm *VM) RegisterProtectedCallLib() {
	vm.SetGlobal("pcall", runtime.FunctionValue(runtime.BuildPCallFunction(vm.callValue)))
	vm.SetGlobal("xpcall", runtime.FunctionValue(runtime.BuildXPCallFunction(vm.callValue)))
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
	lib.RawSetString("protect", runtime.FunctionValue(runtime.BuildTestkitProtectFunction(vm.callValue, func() bool {
		return vm.testkitAccess
	})))
	lib.RawSetString("functionInfo", runtime.FunctionValue(&runtime.GoFunction{
		Name: "testkit.functionInfo",
		Fn: func(args []runtime.Value) ([]runtime.Value, error) {
			if !vm.testkitAccess {
				return nil, fmt.Errorf("testkit access disabled")
			}
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
	vm.SetGlobal("tostring", runtime.FunctionValue(runtime.BuildToStringFunction(vm.callValue)))
}

// RegisterTypeLib installs a VM-aware type builtin that returns VM-owned,
// preboxed strings. This keeps the builtin allocation-free while preserving
// ordinary global override semantics through the normal GETGLOBAL guard.

func (vm *VM) RegisterTypeLib() {
	vm.SetGlobal("type", runtime.FunctionValue(runtime.BuildTypeFunction(vm.typeNameValue)))
}

func (vm *VM) newPCallFunction() *runtime.GoFunction {
	return runtime.BuildPCallFunction(vm.callValue)
}

func (vm *VM) newXPCallFunction() *runtime.GoFunction {
	return runtime.BuildXPCallFunction(vm.callValue)
}

func (vm *VM) newTypeFunction() *runtime.GoFunction {
	return runtime.BuildTypeFunction(vm.typeNameValue)
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
	return modules.BuildTableSortFunction(
		vm.callValue,
		vm.valueLessThan,
		vm.tableLenInt,
		vm.tableGet,
		vm.tableSet,
		func(t runtime.Value, length int64) bool {
			if tbl := t.Table(); tbl != nil && tbl.TryPlainArraySort(length) {
				return true
			}
			return false
		},
	)
}

// RegisterTableHigherOrderLib installs VM-aware table higher-order helpers so
// file-loaded VM closures can be used as callbacks.

func (vm *VM) RegisterTableHigherOrderLib() {
	tblVal, ok := vm.globals["table"]
	if !ok || !tblVal.IsTable() {
		return
	}
	modules.BuildTableHigherOrderLibWithCaller(vm.callValue, tblVal.Table())
}

// RegisterSortCallbackLib installs VM-aware sort namespace helpers whose
// callbacks may be file-loaded VM closures.

func (vm *VM) RegisterSortCallbackLib() {
	sortVal, ok := vm.globals["sort"]
	if !ok || !sortVal.IsTable() {
		return
	}
	built := modules.BuildSortLibWithCallerAndLess(vm.callValue, vm.valueLessThan)
	dst := sortVal.Table()
	for _, name := range []string{"by", "byKey", "partition", "min", "max"} {
		dst.RawSetString(name, built.RawGetString(name))
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
		posInput := int64(0)
		if hasPos {
			posInput = vmToInt(posValue)
		}
		pos, err := tablelib.InsertPosition(length, posInput, hasPos)
		if err != nil {
			return err
		}
		if t.Table().TryPlainArrayInsertKnownLength(pos, value, int64(length)) {
			return nil
		}
		if !hasPos {
			return vm.tableSet(t, runtime.IntValue(pos), value)
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
		posInput := int64(0)
		if hasPos {
			posInput = vmToInt(posValue)
		}
		pos, end, err := tablelib.RemovePosition(length, posInput, hasPos)
		if err != nil {
			return runtime.NilValue(), err
		}
		if end {
			return runtime.NilValue(), nil
		}
		if removed, ok := t.Table().TryPlainArrayRemoveKnownLength(pos, int64(length)); ok {
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
		count, err := tablelib.CheckUnpackRange(name, i, j)
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
		plan := tablelib.PlanMove(f, e, tPos, src.Table() == dst.Table())
		if plan.Count > 0 {
			if dst.Table().TryPlainArrayMove(src.Table(), plan.First, plan.Last, plan.Target) {
				return dst, nil
			}
			if handled, result, err := vm.tryForwardingProxyTableMove(src, dst, plan.First, plan.Last, plan.Target); handled || err != nil {
				return result, err
			}
			if plan.Forward {
				for i := int64(0); i < plan.Count; i++ {
					v, err := vm.tableGet(src, runtime.IntValue(plan.First+i))
					if err != nil {
						return runtime.NilValue(), err
					}
					if err := vm.tableSet(dst, runtime.IntValue(plan.Target+i), v); err != nil {
						return runtime.NilValue(), err
					}
				}
			} else {
				for i := plan.Count - 1; i >= 0; i-- {
					v, err := vm.tableGet(src, runtime.IntValue(plan.First+i))
					if err != nil {
						return runtime.NilValue(), err
					}
					if err := vm.tableSet(dst, runtime.IntValue(plan.Target+i), v); err != nil {
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
	std := vm.newStdlibInstallContext()
	var strLib *runtime.Table
	if existing, ok := vm.globals["string"]; ok && existing.IsTable() {
		strLib = modules.RefreshString(existing.Table(), vm.callValue, func() int64 { return vm.maxHostResult })
	} else {
		strLib = modules.BuildString(vm.callValue, func() int64 { return vm.maxHostResult })
	}
	std.RegisterTable("string", strLib)
	meta := runtime.NewTable()
	meta.RawSet(runtime.StringValue("__index"), runtime.TableValue(strLib))
	vm.stringMeta = meta
}

func (vm *VM) RegisterLLMLib() {
	modules.InstallLLM(vm.newStdlibInstallContext(), llmrt.Options{
		Call: vm.callValue,
		Provider: func() runtime.LLMProvider {
			return vm.llmProvider
		},
		ProviderFactory: func() runtime.LLMProviderFactory {
			return vm.llmProviderFactory
		},
		MaxHostResult: func() int64 {
			return vm.maxHostResult
		},
		Context: func() context.Context {
			if vm.ctx == nil {
				return context.Background()
			}
			return vm.ctx
		},
		Trace: func(event runtime.LLMTraceEvent) {
			if vm.llmTraceSink != nil {
				vm.llmTraceSink(event)
			}
		},
	})
}

func (vm *VM) RegisterHTTPLib() {
	std := vm.newStdlibInstallContext()
	httpLib := runtime.TableValue(modules.BuildHTTPWithCallerAndPolicy(vm.callValue, func() bool {
		return vm.networkAccess
	}, func() int64 {
		return vm.maxHostResult
	}))
	std.RegisterModule("http", httpLib)
}

func (vm *VM) RegisterSyncLib() {
	std := vm.newStdlibInstallContext()
	syncLib := runtime.TableValue(modules.BuildSync(syncrt.Options{
		Call:   vm.callValue,
		Launch: vm.launchSyncTask,
	}))
	std.RegisterModule("sync", syncLib)
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
		if !vm.dynamicEval {
			return nil, fmt.Errorf("dynamic eval disabled")
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
		if !vm.dynamicEval {
			return nil, fmt.Errorf("dynamic eval disabled")
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
		if !vm.dynamicEval {
			return []runtime.Value{runtime.NilValue(), runtime.StringValue("dynamic eval disabled")}, nil
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
		if err := vm.enterModuleLoad(); err != nil {
			return nil, err
		}
		defer vm.leaveModuleLoad()
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
