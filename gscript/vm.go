package gscript

import (
	"context"
	"fmt"
	"reflect"

	"github.com/gscript/gscript/internal/runtime"
	bytecodevm "github.com/gscript/gscript/internal/vm"
)

// VM is a GScript virtual machine instance.
// A VM is NOT goroutine-safe; use Pool for concurrent access.
type VM struct {
	interp *runtime.Interpreter
	opts   vmOptions
	bvm    *bytecodevm.VM // persisted bytecode VM for Call routing (nil if tree-walker mode)
}

// New creates a new GScript VM with the given options.
func New(opts ...Option) *VM {
	o := vmOptions{
		libs:         LibAll,
		capabilities: CapAll,
	}
	for _, opt := range opts {
		opt(&o)
	}
	return newVM(o)
}

func newVM(o vmOptions) *VM {
	interp := runtime.New()
	allowedStdlib := stdlibAllowedNames(o.libs)
	interp.RestrictStdlib(allowedStdlib)
	interp.SetModuleLoading(o.capabilities&CapModuleLoading != 0)
	interp.SetFilesystemRoot(o.filesystemRoot)
	interp.SetFilesystemCapabilities(
		o.capabilities&CapFilesystemRead != 0,
		o.capabilities&CapFilesystemWrite != 0,
	)
	interp.SetEnvironmentCapabilities(
		o.capabilities&CapEnvironmentRead != 0,
		o.capabilities&CapEnvironmentWrite != 0,
	)
	if o.maxSteps > 0 {
		interp.SetMaxSteps(o.maxSteps)
		o.useJIT = false
	}

	// Override print if requested
	if o.printFunc != nil {
		interp.SetGlobal("print", runtime.FunctionValue(&runtime.GoFunction{
			Name: "print",
			Fn: func(args []runtime.Value) ([]runtime.Value, error) {
				iArgs := make([]interface{}, len(args))
				for i, a := range args {
					iArgs[i] = a.String()
				}
				o.printFunc(iArgs...)
				return nil, nil
			},
		}))
	}

	if o.requirePath != "" {
		interp.SetScriptDir(o.requirePath)
	}

	return &VM{interp: interp, opts: o}
}

// Exec compiles and executes a GScript source string.
func (vm *VM) Exec(src string) error {
	return vm.ExecContext(context.Background(), src)
}

// ExecContext compiles and executes a GScript source string.
//
// Context cancellation is checked before starting and after completion. Runtime
// preemption for long-running scripts is a separate sandbox/resource-control
// feature and is not implied by this method.
func (vm *VM) ExecContext(ctx context.Context, src string) error {
	prog, err := CompileContext(ctx, src)
	if err != nil {
		return err
	}
	return vm.RunContext(ctx, prog)
}

// ExecFile reads and executes a GScript source file.
func (vm *VM) ExecFile(path string) error {
	return vm.ExecFileContext(context.Background(), path)
}

// ExecFileContext reads and executes a GScript source file.
//
// Context cancellation is checked before starting and after completion. Runtime
// preemption for long-running scripts is a separate sandbox/resource-control
// feature and is not implied by this method.
func (vm *VM) ExecFileContext(ctx context.Context, path string) error {
	prog, err := CompileFileContext(ctx, path)
	if err != nil {
		return err
	}
	return vm.RunContext(ctx, prog)
}

// Run executes a previously compiled Program.
func (vm *VM) Run(prog *Program) error {
	return vm.RunContext(context.Background(), prog)
}

// RunContext executes a previously compiled Program.
//
// Context cancellation is checked before starting and after completion. Runtime
// preemption for long-running scripts is a separate sandbox/resource-control
// feature and is not implied by this method.
func (vm *VM) RunContext(ctx context.Context, prog *Program) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	if prog == nil {
		return &Error{Kind: ErrRuntime, Message: "nil program"}
	}
	if prog.scriptDir != "" {
		vm.interp.SetScriptDir(prog.scriptDir)
	}
	if vm.opts.useVM {
		// Bytecode VM path
		proto, err := prog.bytecodeProto()
		if err != nil {
			return runtimeError(err, prog.sourceName)
		}
		globals := vm.interp.ExportGlobals()
		// Reuse existing bytecode VM if available (preserves JIT state)
		bvm := vm.bvm
		if bvm == nil {
			bvm = bytecodevm.New(globals)
			bvm.SetStringMeta(vm.interp.StringMeta())
			bvm.SetScriptDir(vm.interp.ScriptDir())
			bvm.RestrictStdlib(stdlibAllowedNames(vm.opts.libs))
			vm.applyBytecodeCapabilities(bvm)
			if vm.opts.maxSteps > 0 {
				bvm.SetMaxSteps(vm.opts.maxSteps)
			}
			if vm.opts.useJIT {
				enableJIT(bvm)
			}
		} else if vm.opts.maxSteps > 0 {
			bvm.SetMaxSteps(vm.opts.maxSteps)
		}
		if _, err := bvm.Execute(proto); err != nil {
			return runtimeError(err, prog.sourceName)
		}
		// Persist the bytecode VM for future Call routing
		vm.bvm = bvm
		vm.syncBytecodeGlobals()
		if err := checkContext(ctx); err != nil {
			return err
		}
		return nil
	}

	if err := vm.interp.Exec(prog.ast); err != nil {
		return runtimeError(err, prog.sourceName)
	}
	if err := checkContext(ctx); err != nil {
		return err
	}
	return nil
}

func (vm *VM) applyBytecodeCapabilities(bvm *bytecodevm.VM) {
	if vm.opts.capabilities&CapModuleLoading == 0 || vm.opts.filesystemRoot != "" {
		vm.copyInterpreterGlobalToBytecode(bvm, "require")
	}
	if vm.opts.capabilities&CapFilesystem == 0 {
		for _, name := range []string{"fs", "dofile", "loadfile"} {
			bvm.DeleteGlobal(name)
		}
	} else if vm.opts.capabilities&CapFilesystemRead == 0 {
		for _, name := range []string{"dofile", "loadfile"} {
			bvm.DeleteGlobal(name)
		}
	} else if vm.opts.filesystemRoot != "" {
		for _, name := range []string{"dofile", "loadfile"} {
			vm.copyInterpreterGlobalToBytecode(bvm, name)
		}
	}
}

func (vm *VM) copyInterpreterGlobalToBytecode(bvm *bytecodevm.VM, name string) {
	val := vm.interp.GetGlobal(name)
	if val.IsNil() {
		bvm.DeleteGlobal(name)
		return
	}
	bvm.SetGlobal(name, val)
}

func stdlibAllowedNames(libs LibFlags) map[string]bool {
	allowed := map[string]bool{
		"array":     libs&LibArray != 0,
		"base64":    libs&LibBase64 != 0,
		"binary":    libs&LibBinary != 0,
		"bit32":     libs&LibBit32 != 0,
		"bits":      libs&LibBits != 0,
		"bytes":     libs&LibBytes != 0,
		"color":     libs&LibColor != 0,
		"compress":  libs&LibCompress != 0,
		"container": libs&LibContainer != 0,
		"crypto":    libs&LibCrypto != 0,
		"csv":       libs&LibCSV != 0,
		"debug":     libs&LibDebug != 0,
		"encoding":  libs&LibEncoding != 0,
		"fs":        libs&LibFS != 0,
		"hash":      libs&LibHash != 0,
		"http":      libs&LibHTTP != 0,
		"io":        libs&LibIO != 0,
		"json":      libs&LibJSON != 0,
		"log":       libs&LibLog != 0,
		"math":      libs&LibMath != 0,
		"matrix":    libs&LibMatrix != 0,
		"net":       libs&LibNet != 0,
		"os":        libs&LibOS != 0,
		"path":      libs&LibPath != 0,
		"process":   libs&LibProcess != 0,
		"rand":      libs&LibRand != 0,
		"regexp":    libs&LibRegexp != 0,
		"rl":        libs&LibGL != 0,
		"script":    libs&LibScript != 0,
		"soa":       libs&LibSoA != 0,
		"sort":      libs&LibSort != 0,
		"string":    libs&LibString != 0,
		"table":     libs&LibTable != 0,
		"testkit":   libs&LibTestkit != 0,
		"time":      libs&LibTime != 0,
		"url":       libs&LibURL != 0,
		"utf8":      libs&LibUTF8 != 0,
		"uuid":      libs&LibUUID != 0,
		"vec":       libs&LibVec != 0,
	}
	return allowed
}

func setBytecodeSource(proto *bytecodevm.FuncProto, filename string) {
	if proto == nil {
		return
	}
	if proto.Source == "" {
		proto.Source = filename
	}
	for _, child := range proto.Protos {
		setBytecodeSource(child, filename)
	}
}

func (vm *VM) syncBytecodeGlobals() {
	if vm.bvm == nil {
		return
	}
	for name, val := range vm.bvm.Globals() {
		vm.interp.SetGlobal(name, val)
	}
}

func (vm *VM) snapshotGlobals() map[string]runtime.Value {
	if vm.bvm != nil {
		vm.syncBytecodeGlobals()
	}
	return vm.interp.ExportGlobals()
}

func (vm *VM) restoreGlobals(globals map[string]runtime.Value) {
	vm.interp.ReplaceGlobals(globals)
	vm.bvm = nil
}

func (vm *VM) preserveHotReloadState(before map[string]runtime.Value, _ *Program) {
	after := vm.interp.ExportGlobals()
	for name, old := range before {
		current, ok := after[name]
		if !ok || current.IsFunction() || old.IsFunction() {
			continue
		}
		if old.IsTable() && current.IsTable() {
			hotReloadMergeTables(old.Table(), current.Table(), make(map[*runtime.Table]bool))
		}
		vm.interp.AssignGlobal(name, old)
		if vm.bvm != nil {
			vm.bvm.SetGlobal(name, old)
		}
	}
}

// Call calls a named GScript function with Go arguments and returns Go values.
// Args and return values are automatically converted via reflection.
func (vm *VM) Call(name string, args ...interface{}) ([]interface{}, error) {
	return vm.CallContext(context.Background(), name, args...)
}

// CallContext calls a named GScript function with Go arguments and returns Go values.
//
// Context cancellation is checked before starting and after completion. Runtime
// preemption for long-running scripts is a separate sandbox/resource-control
// feature and is not implied by this method.
func (vm *VM) CallContext(ctx context.Context, name string, args ...interface{}) ([]interface{}, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	fn := vm.interp.GetGlobal(name)
	if fn.IsNil() {
		return nil, &Error{Kind: ErrRuntime, Message: fmt.Sprintf("function %q not found", name)}
	}
	results, err := vm.callValue(fn, args...)
	if err != nil {
		return nil, err
	}
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	return results, nil
}

// CallValue calls a GScript function value (obtained via Get) with Go arguments.
func (vm *VM) CallValue(fn interface{}, args ...interface{}) ([]interface{}, error) {
	return vm.CallValueContext(context.Background(), fn, args...)
}

// CallValueContext calls a GScript function value with Go arguments.
//
// Context cancellation is checked before starting and after completion. Runtime
// preemption for long-running scripts is a separate sandbox/resource-control
// feature and is not implied by this method.
func (vm *VM) CallValueContext(ctx context.Context, fn interface{}, args ...interface{}) ([]interface{}, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	var gsVal runtime.Value
	if v, ok := fn.(runtime.Value); ok {
		gsVal = v
	} else {
		v2, err := ToValue(fn)
		if err != nil {
			return nil, err
		}
		gsVal = v2
	}
	results, err := vm.callValue(gsVal, args...)
	if err != nil {
		return nil, err
	}
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	return results, nil
}

func (vm *VM) callValue(fn runtime.Value, args ...interface{}) ([]interface{}, error) {
	gsArgs := make([]runtime.Value, len(args))
	for i, a := range args {
		v, err := ToValue(a)
		if err != nil {
			return nil, &Error{Kind: ErrRuntime, Message: fmt.Sprintf("arg %d: %v", i, err)}
		}
		gsArgs[i] = v
	}

	var results []runtime.Value
	var err error
	// Route through bytecode VM if available (handles bytecode closures correctly)
	if vm.bvm != nil {
		results, err = vm.bvm.CallValue(fn, gsArgs)
	} else {
		results, err = vm.interp.CallFunction(fn, gsArgs)
	}
	if err != nil {
		return nil, runtimeError(err, "")
	}
	out := make([]interface{}, len(results))
	for i, r := range results {
		rv, err := fromValueDefault(r)
		if err != nil || !rv.IsValid() {
			out[i] = nil
		} else {
			out[i] = rv.Interface()
		}
	}
	return out, nil
}

// Set sets a global variable to a Go value (auto-converted).
func (vm *VM) Set(name string, val interface{}) error {
	gsVal, err := ToValue(val)
	if err != nil {
		return err
	}
	vm.interp.SetGlobal(name, gsVal)
	return nil
}

// Get gets a global variable as a Go interface{} (auto-converted).
func (vm *VM) Get(name string) (interface{}, error) {
	gsVal := vm.interp.GetGlobal(name)
	rv, err := fromValueDefault(gsVal)
	if err != nil {
		return nil, err
	}
	if !rv.IsValid() {
		return nil, nil
	}
	return rv.Interface(), nil
}

// GetValue gets a global as a raw runtime.Value.
func (vm *VM) GetValue(name string) runtime.Value {
	return vm.interp.GetGlobal(name)
}

// SetValue sets a global to a raw runtime.Value.
func (vm *VM) SetValue(name string, val runtime.Value) {
	vm.interp.SetGlobal(name, val)
	if vm.bvm != nil {
		vm.bvm.SetGlobal(name, val)
	}
}

// RegisterFunc registers a Go function as a GScript global.
// fn must be a func type. Args/returns are auto-converted via reflection.
//
// Example:
//
//	vm.RegisterFunc("distance", func(x1, y1, x2, y2 float64) float64 {
//	    dx, dy := x2-x1, y2-y1
//	    return math.Sqrt(dx*dx + dy*dy)
//	})
func (vm *VM) RegisterFunc(name string, fn interface{}) error {
	rv := reflect.ValueOf(fn)
	if rv.Kind() != reflect.Func {
		return fmt.Errorf("RegisterFunc: %q must be a func, got %T", name, fn)
	}
	wrapped, err := wrapGoFunc(rv)
	if err != nil {
		return fmt.Errorf("RegisterFunc: %q: %v", name, err)
	}
	wrapped.Name = name
	vm.interp.SetGlobal(name, runtime.FunctionValue(wrapped))
	if vm.bvm != nil {
		vm.bvm.SetGlobal(name, runtime.FunctionValue(wrapped))
	}
	return nil
}

// RegisterTable registers a table of Go functions as a global namespace.
//
// Example:
//
//	vm.RegisterTable("vec", map[string]interface{}{
//	    "dot":   func(ax,ay,bx,by float64) float64 { return ax*bx + ay*by },
//	    "cross": func(ax,ay,bx,by float64) float64 { return ax*by - ay*bx },
//	})
func (vm *VM) RegisterTable(name string, members map[string]interface{}) error {
	t := runtime.NewTable()
	for k, v := range members {
		gsVal, err := ToValue(v)
		if err != nil {
			return fmt.Errorf("RegisterTable %s.%s: %v", name, k, err)
		}
		t.RawSet(runtime.StringValue(k), gsVal)
	}
	vm.interp.SetGlobal(name, runtime.TableValue(t))
	if vm.bvm != nil {
		vm.bvm.SetGlobal(name, runtime.TableValue(t))
	}
	return nil
}

// RegisterModule registers a Go-backed module for require(name).
//
// Go-backed host modules are explicit embedding capabilities: this API exposes
// only the names the embedding application registers. Built-in stdlib modules
// and filesystem-loaded .gs modules keep their existing controls. Registered
// host modules remain available when filesystem module loading is disabled,
// which makes them suitable for sandboxed embeddings that need a narrow Go API
// surface.
//
// Example:
//
//	vm.RegisterModule("go/strings", gscript.Module{
//	    "upper": strings.ToUpper,
//	    "trim":  strings.TrimSpace,
//	})
func (vm *VM) RegisterModule(name string, members Module) error {
	if name == "" {
		return fmt.Errorf("RegisterModule: empty module name")
	}
	t := runtime.NewTable()
	for k, v := range members {
		gsVal, err := ToValue(v)
		if err != nil {
			return fmt.Errorf("RegisterModule %s.%s: %v", name, k, err)
		}
		t.RawSet(runtime.StringValue(k), gsVal)
	}
	val := runtime.TableValue(t)
	vm.interp.SetModule(name, val)
	if vm.bvm != nil {
		vm.bvm.SetGlobal(name, val)
	}
	return nil
}

// RegisterModuleFrom exposes exported fields and methods from source as a
// Go-backed module. It is a convenience wrapper around ModuleFrom followed by
// RegisterModule.
func (vm *VM) RegisterModuleFrom(name string, source interface{}, opts ...ModuleFromOption) error {
	members, err := ModuleFrom(source, opts...)
	if err != nil {
		return err
	}
	return vm.RegisterModule(name, members)
}

// BindStruct registers a Go struct type as a GScript class.
// proto should be a zero value or example of the struct (e.g. Vec2{} or &Vec2{}).
//
// This creates a GScript global named `name` with a .new() constructor
// and field/method access via metatable.
//
// Example:
//
//	type Vec2 struct{ X, Y float64 }
//	func (v Vec2) Length() float64 { return math.Sqrt(v.X*v.X + v.Y*v.Y) }
//
//	vm.BindStruct("Vec2", Vec2{})
//
//	// In GScript:
//	v := Vec2.new(3, 4)
//	print(v.Length())  // 5
//	print(v.X)         // 3
//	v.X = 10
func (vm *VM) BindStruct(name string, proto interface{}) error {
	return bindStructToInterp(vm.interp, name, proto, nil)
}

// BindStructWithConstructor is like BindStruct but uses a custom constructor function.
// The constructor is called when GScript calls Name.new(args...).
func (vm *VM) BindStructWithConstructor(name string, proto interface{}, ctor interface{}) error {
	return bindStructToInterp(vm.interp, name, proto, ctor)
}

// BindMethod adds a method to an already-registered struct class.
func (vm *VM) BindMethod(className, methodName string, fn interface{}) error {
	classVal := vm.interp.GetGlobal(className)
	if !classVal.IsTable() {
		return fmt.Errorf("BindMethod: %q is not a registered class", className)
	}
	rv := reflect.ValueOf(fn)
	wrapped, err := wrapGoFunc(rv)
	if err != nil {
		return err
	}
	wrapped.Name = className + "." + methodName
	classVal.Table().RawSet(runtime.StringValue(methodName), runtime.FunctionValue(wrapped))
	return nil
}

// CallFunction exposes the interpreter's CallFunction for advanced use.
// Useful when you have a runtime.Value function and want to call it.
func (vm *VM) CallFunction(fn runtime.Value, args []runtime.Value) ([]runtime.Value, error) {
	if vm.bvm != nil {
		return vm.bvm.CallValue(fn, args)
	}
	return vm.interp.CallFunction(fn, args)
}

// Interpreter returns the underlying runtime.Interpreter.
// Use for advanced access; prefer the VM methods when possible.
func (vm *VM) Interpreter() *runtime.Interpreter {
	return vm.interp
}
