// vm_jit_interface.go exposes VM internals needed by the JIT deopt recovery
// path. These functions are separate from vm.go to keep that file under the
// 1000-line limit.

package vm

import (
	"fmt"

	"github.com/never-labs/gscript/internal/runtime"
)

// ResumeFromPC resumes execution of the current top call frame starting at
// startPC. The VM's register file must already contain the execution state up
// to that point (written by the JIT before the deopt).
//
// Used by the baseline JIT overflow deopt path: instead of restarting at
// pc=0 (which replays side effects like table writes or SETGLOBAL exits that
// occurred before the overflowing instruction), the interpreter resumes at
// the exact bytecode where the int-spec guard fired.
//
// The caller must hold an active call frame (vm.frameCount > 0). The frame
// was pushed by vm.call before invoking vm.methodJIT.Execute; ResumeFromPC
// reuses it so no double-frame is created.
func (vm *VM) ResumeFromPC(startPC int) ([]runtime.Value, error) {
	if vm.frameCount == 0 {
		return nil, fmt.Errorf("ResumeFromPC: no active call frame")
	}
	vm.frames[vm.frameCount-1].pc = startPC
	return vm.run()
}

// TableGetForJIT exposes the VM's full table-get semantics to JIT slow paths.
// It includes non-table errors and __index metamethod dispatch, unlike
// runtime.Table.RawGet.
func (vm *VM) TableGetForJIT(table, key runtime.Value) (runtime.Value, error) {
	return vm.tableGet(table, key)
}

// TableSetForJIT exposes the VM's full table-set semantics to JIT slow paths.
// It includes __newindex dispatch and recursive table __newindex handling,
// unlike runtime.Table.RawSet.
func (vm *VM) TableSetForJIT(table, key, val runtime.Value) error {
	return vm.tableSet(table, key, val)
}

// MetamethodForJIT returns the current metamethod value for a table-backed
// value. The optional field cache is scoped to the callsite and caches lookup
// into the current metatable shape; it never caches the object table itself.
func (vm *VM) MetamethodForJIT(v runtime.Value, name string, cache *runtime.FieldCacheEntry) runtime.Value {
	if !v.IsTable() {
		return runtime.NilValue()
	}
	mt := v.Table().GetMetatable()
	if mt == nil {
		return runtime.NilValue()
	}
	if cache != nil {
		return mt.RawGetStringCached(name, cache)
	}
	return mt.RawGetString(name)
}

// LengthForJIT exposes OP_LEN semantics to JIT slow paths, including table
// __len metamethods. The cache only accelerates lookup on the metatable.
func (vm *VM) LengthForJIT(v runtime.Value, cache *runtime.FieldCacheEntry) (runtime.Value, error) {
	if v.IsString() {
		return runtime.IntValue(int64(runtime.StringLen(v))), nil
	}
	if v.IsTable() {
		if mm := vm.MetamethodForJIT(v, "__len", cache); !mm.IsNil() {
			if result, ok := fastReturnUpvalueClosure(mm); ok {
				return result, nil
			}
			args := [1]runtime.Value{v}
			results, err := vm.callValue(mm, args[:])
			if err != nil {
				return runtime.NilValue(), err
			}
			if len(results) > 0 {
				return results[0], nil
			}
			return runtime.IntValue(0), nil
		}
		return runtime.IntValue(int64(v.Table().Len())), nil
	}
	return runtime.NilValue(), fmt.Errorf("attempt to get length of a %s value", v.TypeName())
}

// SetGlobalForJIT exposes OP_SETGLOBAL semantics to JIT slow paths. It must
// keep the same readonly-binding checks as the bytecode interpreter; plain
// SetGlobal is a host API and intentionally bypasses script-level const checks.
func (vm *VM) SetGlobalForJIT(name string, val runtime.Value) error {
	if vm.isGlobalReadOnly(name) {
		return fmt.Errorf("cannot assign to readonly variable %q", name)
	}
	vm.SetGlobal(name, val)
	return nil
}
