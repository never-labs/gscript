package gscript

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/gscript/gscript/internal/ast"
	"github.com/gscript/gscript/internal/runtime"
)

// HotLoader compiles GScript files into atomically swappable program handles.
//
// It intentionally does not start a filesystem watcher. Embedding applications
// can call Reload from their own watcher, admin endpoint, or deployment hook.
// HotLoader does not register files into require(); it is a Go embedding API
// for managing the latest successfully compiled Program.
type HotLoader struct {
	mu      sync.Mutex
	opts    []CompileOption
	vmOpts  []Option
	modules map[string]*ModuleHandle
}

// HotLoaderOption configures a HotLoader.
type HotLoaderOption func(*HotLoader)

// WithHotLoaderCompileOptions applies compile options to each Load/Reload.
func WithHotLoaderCompileOptions(opts ...CompileOption) HotLoaderOption {
	return func(loader *HotLoader) {
		loader.opts = append(loader.opts, opts...)
	}
}

// WithHotLoaderVMOptions applies VM options to HotInstance VMs created by this
// loader.
func WithHotLoaderVMOptions(opts ...Option) HotLoaderOption {
	return func(loader *HotLoader) {
		loader.vmOpts = append(loader.vmOpts, opts...)
	}
}

// NewHotLoader creates a hot loader for GScript source files.
func NewHotLoader(opts ...HotLoaderOption) *HotLoader {
	loader := &HotLoader{
		modules: make(map[string]*ModuleHandle),
	}
	for _, opt := range opts {
		opt(loader)
	}
	return loader
}

// Load compiles path and returns a handle. Repeated Load calls for the same
// path return the existing handle after reloading it.
func (loader *HotLoader) Load(path string) (*ModuleHandle, error) {
	return loader.LoadContext(context.Background(), path)
}

// LoadContext is the context-aware form of Load.
func (loader *HotLoader) LoadContext(ctx context.Context, path string) (*ModuleHandle, error) {
	if path == "" {
		return nil, fmt.Errorf("HotLoader.Load: empty path")
	}

	loader.mu.Lock()
	defer loader.mu.Unlock()

	prog, err := loader.compileLocked(ctx, path)
	if err != nil {
		return nil, err
	}

	handle := loader.modules[path]
	if handle == nil {
		handle = &ModuleHandle{path: path}
		loader.modules[path] = handle
	}
	handle.install(prog)
	return handle, nil
}

func (loader *HotLoader) compileLocked(ctx context.Context, path string) (*Program, error) {
	return CompileFileContext(ctx, path, loader.opts...)
}

func (loader *HotLoader) handleLocked(path string) *ModuleHandle {
	handle := loader.modules[path]
	if handle == nil {
		handle = &ModuleHandle{path: path}
		loader.modules[path] = handle
	}
	return handle
}

// Reload recompiles path and atomically swaps the handle on success. If
// compilation fails, the previously installed program remains active.
func (loader *HotLoader) Reload(path string) error {
	return loader.ReloadContext(context.Background(), path)
}

// ReloadContext is the context-aware form of Reload.
func (loader *HotLoader) ReloadContext(ctx context.Context, path string) error {
	_, err := loader.LoadContext(ctx, path)
	return err
}

// Handle returns a previously loaded module handle.
func (loader *HotLoader) Handle(path string) (*ModuleHandle, bool) {
	loader.mu.Lock()
	defer loader.mu.Unlock()
	handle, ok := loader.modules[path]
	return handle, ok
}

// LoadInstance compiles path, creates a persistent VM, and runs the program
// once. Future Reload calls on the returned instance preserve existing
// non-function globals by default while replacing function definitions.
func (loader *HotLoader) LoadInstance(path string) (*HotInstance, error) {
	return loader.LoadInstanceContext(context.Background(), path)
}

// LoadInstanceContext is the context-aware form of LoadInstance.
func (loader *HotLoader) LoadInstanceContext(ctx context.Context, path string) (*HotInstance, error) {
	if path == "" {
		return nil, fmt.Errorf("HotLoader.LoadInstance: empty path")
	}

	loader.mu.Lock()
	prog, err := loader.compileLocked(ctx, path)
	if err != nil {
		loader.mu.Unlock()
		return nil, err
	}
	handle := loader.handleLocked(path)
	loader.mu.Unlock()

	vm := New(loader.vmOpts...)
	inst := &HotInstance{
		loader: loader,
		handle: handle,
		vm:     vm,
	}
	if err := inst.applyProgram(ctx, prog, false); err != nil {
		return nil, err
	}
	handle.install(prog)
	inst.appliedSnapshot = handle.current.Load()
	return inst, nil
}

// ModuleHandle points to the latest compiled generation of one GScript file.
// Its atomic snapshot makes reload publication safe, but each Program must
// still be run according to Program's concurrency contract.
type ModuleHandle struct {
	path    string
	current atomic.Pointer[moduleSnapshot]
}

type moduleSnapshot struct {
	generation uint64
	program    *Program
}

func (handle *ModuleHandle) install(prog *Program) {
	var generation uint64 = 1
	if current := handle.current.Load(); current != nil {
		generation = current.generation + 1
	}
	handle.current.Store(&moduleSnapshot{generation: generation, program: prog})
}

// Path returns the source path associated with this handle.
func (handle *ModuleHandle) Path() string {
	if handle == nil {
		return ""
	}
	return handle.path
}

// Generation returns the current generation. It increments only after a
// successful Load or Reload.
func (handle *ModuleHandle) Generation() uint64 {
	if handle == nil {
		return 0
	}
	snapshot := handle.current.Load()
	if snapshot == nil {
		return 0
	}
	return snapshot.generation
}

// Program returns the current compiled program.
func (handle *ModuleHandle) Program() (*Program, bool) {
	if handle == nil {
		return nil, false
	}
	snapshot := handle.current.Load()
	if snapshot == nil || snapshot.program == nil {
		return nil, false
	}
	return snapshot.program, true
}

// Run executes the latest generation on vm.
func (handle *ModuleHandle) Run(vm *VM) error {
	return handle.RunContext(context.Background(), vm)
}

// RunContext executes the latest generation on vm.
func (handle *ModuleHandle) RunContext(ctx context.Context, vm *VM) error {
	if vm == nil {
		return fmt.Errorf("ModuleHandle.Run: nil VM")
	}
	prog, ok := handle.Program()
	if !ok {
		return fmt.Errorf("ModuleHandle.Run: no program loaded")
	}
	return vm.RunContext(ctx, prog)
}

// Call runs the latest generation and then calls a named function on vm. The
// top-level program is executed on every Call, so embedders that want to avoid
// replaying top-level side effects should call Run once on their own VM and
// then use VM.Call.
func (handle *ModuleHandle) Call(vm *VM, name string, args ...interface{}) ([]interface{}, error) {
	return handle.CallContext(context.Background(), vm, name, args...)
}

// CallContext runs the latest generation and then calls a named function on vm.
// See Call for the top-level execution behavior.
func (handle *ModuleHandle) CallContext(ctx context.Context, vm *VM, name string, args ...interface{}) ([]interface{}, error) {
	if err := handle.RunContext(ctx, vm); err != nil {
		return nil, err
	}
	return vm.CallContext(ctx, name, args...)
}

// HotInstance is a loaded GScript file with a persistent VM. Reloading an
// instance keeps the VM and its existing non-function globals by default, so
// ordinary script state survives code replacement without an explicit migration
// hook. Running goroutines or externally saved old closures are not migrated.
type HotInstance struct {
	mu              sync.Mutex
	loader          *HotLoader
	handle          *ModuleHandle
	vm              *VM
	appliedSnapshot *moduleSnapshot
}

// Handle returns the underlying latest-program handle.
func (inst *HotInstance) Handle() *ModuleHandle {
	if inst == nil {
		return nil
	}
	return inst.handle
}

// VM returns the persistent VM owned by this instance. Callers must not use it
// concurrently with HotInstance methods.
func (inst *HotInstance) VM() *VM {
	if inst == nil {
		return nil
	}
	return inst.vm
}

// Generation returns the current successfully applied generation.
func (inst *HotInstance) Generation() uint64 {
	if inst == nil || inst.appliedSnapshot == nil {
		return 0
	}
	return inst.appliedSnapshot.generation
}

// Reload recompiles and applies the source file while preserving state.
func (inst *HotInstance) Reload() error {
	return inst.ReloadContext(context.Background())
}

// ReloadContext is the context-aware form of Reload.
func (inst *HotInstance) ReloadContext(ctx context.Context) error {
	if inst == nil || inst.loader == nil || inst.handle == nil {
		return fmt.Errorf("HotInstance.Reload: nil instance")
	}

	inst.loader.mu.Lock()
	prog, err := inst.loader.compileLocked(ctx, inst.handle.path)
	if err != nil {
		inst.loader.mu.Unlock()
		return err
	}
	inst.loader.mu.Unlock()

	inst.mu.Lock()
	defer inst.mu.Unlock()

	if err := inst.applyProgram(ctx, prog, true); err != nil {
		return err
	}
	inst.handle.install(prog)
	inst.appliedSnapshot = inst.handle.current.Load()
	return nil
}

// Call calls a function on the persistent VM without rerunning top-level code.
func (inst *HotInstance) Call(name string, args ...interface{}) ([]interface{}, error) {
	return inst.CallContext(context.Background(), name, args...)
}

// CallContext is the context-aware form of Call.
func (inst *HotInstance) CallContext(ctx context.Context, name string, args ...interface{}) ([]interface{}, error) {
	if inst == nil || inst.vm == nil {
		return nil, fmt.Errorf("HotInstance.Call: nil instance")
	}
	inst.mu.Lock()
	defer inst.mu.Unlock()
	return inst.vm.CallContext(ctx, name, args...)
}

func (inst *HotInstance) applyProgram(ctx context.Context, prog *Program, preserve bool) error {
	before := inst.vm.snapshotGlobals()
	beforeDeep := hotReloadDeepSnapshotGlobals(before)
	runProg := prog
	if preserve {
		runProg = hotReloadProgram(prog, before, inst.appliedSnapshot)
	}
	if err := inst.vm.RunContext(ctx, runProg); err != nil {
		hotReloadRestoreGlobals(inst.vm, before, beforeDeep)
		return err
	}
	if preserve {
		inst.vm.preserveHotReloadState(before, runProg)
	}
	return nil
}

func hotReloadProgram(prog *Program, before map[string]runtime.Value, previous *moduleSnapshot) *Program {
	if prog == nil || prog.ast == nil {
		return prog
	}
	previousFuncs := hotReloadFunctionFingerprints(previous)
	stmts := make([]ast.Stmt, 0, len(prog.ast.Stmts))
	for _, stmt := range prog.ast.Stmts {
		stmts = append(stmts, hotReloadRewriteStmt(stmt, before, previousFuncs)...)
	}
	return &Program{
		sourceName: prog.sourceName,
		scriptDir:  prog.scriptDir,
		ast:        &ast.Program{Stmts: stmts},
	}
}

func hotReloadRewriteStmt(stmt ast.Stmt, before map[string]runtime.Value, previousFuncs map[string]string) []ast.Stmt {
	if hotReloadShouldSkipStmt(stmt, before, previousFuncs) {
		return nil
	}
	decl, ok := stmt.(*ast.DeclareStmt)
	if !ok {
		return []ast.Stmt{stmt}
	}
	names := make([]string, 0, len(decl.Names))
	values := make([]ast.Expr, 0, len(decl.Values))
	for i, name := range decl.Names {
		if hotReloadShouldPreserveDeclaredName(name, i, decl.Values, before, previousFuncs) {
			continue
		}
		names = append(names, name)
		if i < len(decl.Values) {
			values = append(values, decl.Values[i])
		}
	}
	if len(names) == 0 {
		return nil
	}
	return []ast.Stmt{&ast.DeclareStmt{
		P:        decl.P,
		Names:    names,
		Values:   values,
		ReadOnly: decl.ReadOnly,
	}}
}

func hotReloadShouldSkipStmt(stmt ast.Stmt, before map[string]runtime.Value, previousFuncs map[string]string) bool {
	switch s := stmt.(type) {
	case *ast.FuncDeclStmt:
		return previousFuncs[s.Name] == hotReloadNodeFingerprint(s)
	case *ast.DeclareStmt:
		if len(s.Names) == 0 {
			return false
		}
		for i, name := range s.Names {
			if !hotReloadShouldPreserveDeclaredName(name, i, s.Values, before, previousFuncs) {
				return false
			}
		}
		return true
	case *ast.AssignStmt:
		if len(s.Targets) == 0 {
			return false
		}
		for _, target := range s.Targets {
			ident, ok := target.(*ast.IdentExpr)
			if !ok || !hotReloadHasStateBinding(ident.Name, before) {
				return false
			}
		}
		return true
	case *ast.CompoundAssignStmt:
		ident, ok := s.Target.(*ast.IdentExpr)
		return ok && hotReloadHasStateBinding(ident.Name, before)
	case *ast.IncDecStmt:
		ident, ok := s.Target.(*ast.IdentExpr)
		return ok && hotReloadHasStateBinding(ident.Name, before)
	default:
		return false
	}
}

func hotReloadShouldPreserveDeclaredName(name string, index int, values []ast.Expr, before map[string]runtime.Value, previousFuncs map[string]string) bool {
	old, ok := before[name]
	if !ok {
		return false
	}
	if old.IsTable() {
		return false
	}
	if !old.IsFunction() {
		return true
	}
	return index < len(values) && previousFuncs[name] == hotReloadNodeFingerprint(values[index])
}

func hotReloadHasStateBinding(name string, before map[string]runtime.Value) bool {
	val, ok := before[name]
	return ok && !val.IsFunction()
}

func hotReloadFunctionFingerprints(snapshot *moduleSnapshot) map[string]string {
	out := make(map[string]string)
	if snapshot == nil || snapshot.program == nil || snapshot.program.ast == nil {
		return out
	}
	for _, stmt := range snapshot.program.ast.Stmts {
		switch s := stmt.(type) {
		case *ast.FuncDeclStmt:
			out[s.Name] = hotReloadNodeFingerprint(s)
		case *ast.DeclareStmt:
			for i, name := range s.Names {
				if i < len(s.Values) {
					out[name] = hotReloadNodeFingerprint(s.Values[i])
				}
			}
		}
	}
	return out
}

func hotReloadNodeFingerprint(node interface{}) string {
	var b strings.Builder
	hotReloadWriteFingerprint(&b, reflect.ValueOf(node))
	return b.String()
}

func hotReloadWriteFingerprint(b *strings.Builder, v reflect.Value) {
	if !v.IsValid() {
		b.WriteString("<nil>")
		return
	}
	for v.Kind() == reflect.Interface || v.Kind() == reflect.Pointer {
		if v.IsNil() {
			b.WriteString("<nil>")
			return
		}
		v = v.Elem()
	}
	t := v.Type()
	b.WriteString(t.String())
	b.WriteByte('{')
	switch v.Kind() {
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			field := t.Field(i)
			if field.Name == "P" || field.Type == reflect.TypeOf(ast.Pos{}) {
				continue
			}
			b.WriteString(field.Name)
			b.WriteByte(':')
			hotReloadWriteFingerprint(b, v.Field(i))
			b.WriteByte(';')
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			hotReloadWriteFingerprint(b, v.Index(i))
			b.WriteByte(',')
		}
	case reflect.Map:
		keys := v.MapKeys()
		sort.Slice(keys, func(i, j int) bool {
			return fmt.Sprint(keys[i].Interface()) < fmt.Sprint(keys[j].Interface())
		})
		for _, key := range keys {
			hotReloadWriteFingerprint(b, key)
			b.WriteByte('=')
			hotReloadWriteFingerprint(b, v.MapIndex(key))
			b.WriteByte(';')
		}
	case reflect.String:
		b.WriteString(v.String())
	case reflect.Bool:
		if v.Bool() {
			b.WriteByte('1')
		} else {
			b.WriteByte('0')
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		b.WriteString(fmt.Sprint(v.Int()))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		b.WriteString(fmt.Sprint(v.Uint()))
	case reflect.Float32, reflect.Float64:
		b.WriteString(fmt.Sprint(v.Float()))
	default:
		b.WriteString(fmt.Sprint(v.Interface()))
	}
	b.WriteByte('}')
}

func hotReloadRestoreTable(dst, src *runtime.Table, seen map[*runtime.Table]bool) {
	if dst == nil || src == nil || seen[dst] {
		return
	}
	seen[dst] = true
	hotReloadClearTable(dst)
	key := runtime.NilValue()
	for {
		k, v, ok := src.Next(key)
		if !ok {
			break
		}
		dst.RawSet(k, v)
		key = k
	}
	if mt := src.GetMetatable(); mt != nil {
		dst.SetMetatable(mt)
	}
}

func hotReloadClearTable(t *runtime.Table) {
	var keys []runtime.Value
	key := runtime.NilValue()
	for {
		k, _, ok := t.Next(key)
		if !ok {
			break
		}
		keys = append(keys, k)
		key = k
	}
	for _, k := range keys {
		t.RawSet(k, runtime.NilValue())
	}
}

func hotReloadDeepSnapshotGlobals(globals map[string]runtime.Value) map[string]runtime.Value {
	seen := make(map[*runtime.Table]runtime.Value)
	out := make(map[string]runtime.Value, len(globals))
	for name, val := range globals {
		out[name] = hotReloadCloneValue(val, seen)
	}
	return out
}

func hotReloadCloneValue(v runtime.Value, seen map[*runtime.Table]runtime.Value) runtime.Value {
	if !v.IsTable() {
		return v
	}
	t := v.Table()
	if cloned, ok := seen[t]; ok {
		return cloned
	}
	copy := runtime.NewTable()
	cloned := runtime.TableValue(copy)
	seen[t] = cloned
	key := runtime.NilValue()
	for {
		k, val, ok := t.Next(key)
		if !ok {
			break
		}
		copy.RawSet(hotReloadCloneValue(k, seen), hotReloadCloneValue(val, seen))
		key = k
	}
	if mt := t.GetMetatable(); mt != nil {
		copy.SetMetatable(mt)
	}
	return cloned
}

func hotReloadRestoreGlobals(vm *VM, before map[string]runtime.Value, snapshot map[string]runtime.Value) {
	restored := make(map[string]runtime.Value, len(before))
	for name, old := range before {
		restored[name] = old
		if old.IsTable() {
			if snap, ok := snapshot[name]; ok && snap.IsTable() {
				hotReloadRestoreTable(old.Table(), snap.Table(), make(map[*runtime.Table]bool))
			}
		}
	}
	vm.restoreGlobals(restored)
}

func hotReloadMergeTables(dst, fresh *runtime.Table, seen map[*runtime.Table]bool) {
	if dst == nil || fresh == nil || seen[dst] {
		return
	}
	seen[dst] = true
	key := runtime.NilValue()
	for {
		k, v, ok := fresh.Next(key)
		if !ok {
			break
		}
		current := dst.RawGet(k)
		switch {
		case current.IsNil():
			dst.RawSet(k, v)
		case current.IsFunction() && v.IsFunction():
			dst.RawSet(k, v)
		case current.IsTable() && v.IsTable():
			hotReloadMergeTables(current.Table(), v.Table(), seen)
		}
		key = k
	}
}
