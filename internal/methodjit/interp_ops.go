// interp_ops.go contains call handling and global lookup for the IR interpreter.
// Split from interp.go to keep file sizes manageable.

package methodjit

import (
	"fmt"
	"unsafe"

	"github.com/gscript/gscript/internal/runtime"
	"github.com/gscript/gscript/internal/vm"
)

// execCall handles function call instructions.
func (s *interpState) execCall(instr *Instr) (runtime.Value, error) {
	if len(instr.Args) == 0 {
		return runtime.NilValue(), fmt.Errorf("IR interpreter: OpCall with no args")
	}

	fnVal := s.val(instr.Args[0])
	callArgs := make([]runtime.Value, len(instr.Args)-1)
	for i := 1; i < len(instr.Args); i++ {
		callArgs[i-1] = s.val(instr.Args[i])
	}

	// Check if this is a self-recursive call.
	if fnVal.IsFunction() {
		if cl, ok := vmClosureFromValue(fnVal); ok && cl.Proto == s.fn.Proto {
			// Self-recursive call: interpret recursively with the same IR.
			results, err := interpretImpl(s.fn, callArgs, s.depth+1)
			if err != nil {
				return runtime.NilValue(), err
			}
			if len(results) > 0 {
				return results[0], nil
			}
			return runtime.NilValue(), nil
		}
	}

	// For non-self calls, use the VM to execute.
	results, err := s.callViaVM(fnVal, callArgs)
	if err != nil {
		return runtime.NilValue(), err
	}
	if len(results) > 0 {
		return results[0], nil
	}
	return runtime.NilValue(), nil
}

// callViaVM executes a function call using the VM interpreter.
func (s *interpState) callViaVM(fnVal runtime.Value, args []runtime.Value) ([]runtime.Value, error) {
	v := s.vm()
	// Populate VM globals with any function protos known to the caller.
	// This lets residual cross-function calls (e.g., after bounded recursive
	// inlining) resolve their callees when the VM executes their bytecode.
	if s.fn.Analysis.GlobalFacts().GlobalsPopulated() {
		s.fn.Analysis.GlobalFacts().ForEachGlobal(func(name string, p *vm.FuncProto) {
			if p == nil {
				return
			}
			cl := vm.NewClosure(p)
			v.SetGlobal(name, runtime.VMClosureFastValue(unsafe.Pointer(cl)))
		})
	}

	return v.CallValue(fnVal, args)
}

func (s *interpState) execTForCall(instr *Instr) ([]runtime.Value, error) {
	key := instr.SourcePC
	if !instr.HasSource {
		key = instr.ID
	}
	resultIndex := int(instr.Aux2)
	if resultIndex > 0 {
		if cached, ok := s.tforResults[key]; ok {
			return cached, nil
		}
	}
	if len(instr.Args) == 0 {
		return nil, fmt.Errorf("IR interpreter: OpTForCall with no callee")
	}
	fnVal := s.val(instr.Args[0])
	args := make([]runtime.Value, len(instr.Args)-1)
	for i := 1; i < len(instr.Args); i++ {
		args[i-1] = s.val(instr.Args[i])
	}
	results, err := s.callViaVM(fnVal, args)
	if err != nil {
		return nil, err
	}
	n := int(instr.Aux)
	if n < 0 {
		n = 0
	}
	if len(results) < n {
		padded := make([]runtime.Value, n)
		copy(padded, results)
		for i := len(results); i < n; i++ {
			padded[i] = runtime.NilValue()
		}
		results = padded
	}
	s.tforResults[key] = results
	return results, nil
}

func (s *interpState) tableGet(table, key runtime.Value) (runtime.Value, error) {
	return s.vm().TableGetForJIT(table, key)
}

func (s *interpState) vm() *vm.VM {
	if s.hostVM != nil {
		return s.hostVM
	}
	globals := make(map[string]runtime.Value)
	s.hostVM = vm.New(globals)
	return s.hostVM
}

func (s *interpState) close() {
	if s.hostVM != nil {
		s.hostVM.Close()
		s.hostVM = nil
	}
}

// getGlobal looks up a global variable by name.
// In the IR interpreter, globals are not available unless we have a VM context.
// For self-recursive functions, the function itself is the only global needed.
// When the Function's Globals table is populated (e.g., after bounded recursive
// inlining leaves residual cross-function calls), other named protos are also
// resolvable.
func (s *interpState) getGlobal(name string) runtime.Value {
	// If the name matches the function being interpreted, return a closure
	// wrapping the current proto so self-recursive calls work.
	if name == s.fn.Proto.Name {
		cl := vm.NewClosure(s.fn.Proto)
		return runtime.VMClosureFastValue(unsafe.Pointer(cl))
	}
	// Consult the inline pass's globals table if available.
	if p, ok := s.fn.Analysis.GlobalFacts().GlobalProto(name); ok && p != nil {
		cl := vm.NewClosure(p)
		return runtime.VMClosureFastValue(unsafe.Pointer(cl))
	}
	if s.hostVM == nil {
		return s.vm().GetGlobal(name)
	}
	if v := s.hostVM.GetGlobal(name); !v.IsNil() {
		return v
	}
	return runtime.NilValue()
}
