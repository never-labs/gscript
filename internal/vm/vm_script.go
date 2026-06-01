package vm

// Script load/compile/execute-in-child helpers, split verbatim from vm.go.

import (
	"fmt"
	"github.com/never-labs/gscript/internal/lexer"
	"github.com/never-labs/gscript/internal/parser"
	"github.com/never-labs/gscript/internal/runtime"
	"os"
	"path/filepath"
	"strings"
	"unsafe"
)

type vmScriptConfig struct {
	sourceName string
	scriptDir  string
	env        *runtime.Table
	sandbox    bool
}

func (vm *VM) compileScriptChunk(src string, opt runtime.Value, defaultSource string) ([]runtime.Value, error) {
	cfg, err := vm.scriptConfigFromValue(opt, defaultSource)
	if err != nil {
		return nil, err
	}
	proto, err := compileScriptSource(src, cfg.sourceName)
	if err != nil {
		return nil, err
	}
	if cfg.sourceName != "" {
		setProtoSource(proto, cfg.sourceName)
	}
	if cfg.scriptDir != "" && cfg.env == nil {
		// Preserve the chunk directory for later relative loads.
		cl := NewClosure(proto)
		return []runtime.Value{runtime.FunctionValue(&runtime.GoFunction{Name: "script.chunk", Fn: func(args []runtime.Value) ([]runtime.Value, error) {
			prev := vm.scriptDir
			vm.scriptDir = cfg.scriptDir
			defer func() { vm.scriptDir = prev }()
			return vm.callValue(runtime.VMClosureFunctionValue(unsafe.Pointer(cl), cl), args)
		}})}, nil
	}
	if cfg.env != nil {
		return []runtime.Value{runtime.FunctionValue(&runtime.GoFunction{Name: "script.chunk", Fn: func(args []runtime.Value) ([]runtime.Value, error) {
			return vm.executeScriptInChild(proto, cfg, args)
		}})}, nil
	}
	cl := NewClosure(proto)
	return []runtime.Value{runtime.VMClosureFunctionValue(unsafe.Pointer(cl), cl)}, nil
}

func (vm *VM) loadScriptFile(filename string, opt runtime.Value) ([]runtime.Value, error) {
	cfg, err := vm.scriptConfigFromValue(opt, filename)
	if err != nil {
		return nil, err
	}
	resolveDir := cfg.scriptDir
	if resolveDir == "" {
		resolveDir = vm.scriptDir
	}
	resolved := vm.resolveScriptPathWithDir(filename, resolveDir)
	if err := vm.checkModuleFileBudget(resolved); err != nil {
		return nil, err
	}
	src, err := os.ReadFile(resolved)
	if err != nil {
		return nil, fmt.Errorf("cannot open %s: %s", resolved, err)
	}
	if cfg.sourceName == "" {
		cfg.sourceName = resolved
	}
	if cfg.scriptDir == "" {
		if abs, err := filepath.Abs(resolved); err == nil {
			cfg.scriptDir = filepath.Dir(abs)
		}
	}
	return vm.compileScriptChunk(string(src), vmScriptConfigValue(cfg), cfg.sourceName)
}

func (vm *VM) checkModuleFileBudget(filename string) error {
	if vm.maxModuleBytes <= 0 {
		return nil
	}
	info, err := os.Stat(filename)
	if err != nil {
		return err
	}
	if info.Size() > vm.maxModuleBytes {
		return fmt.Errorf("module byte limit exceeded (%d)", vm.maxModuleBytes)
	}
	return nil
}

func compileScriptSource(src string, sourceName string) (*FuncProto, error) {
	tokens, err := lexer.New(src).Tokenize()
	if err != nil {
		return nil, wrapScriptCompileSourceError(err, sourceName)
	}
	prog, err := parser.New(tokens).Parse()
	if err != nil {
		return nil, wrapScriptCompileSourceError(err, sourceName)
	}
	proto, err := Compile(prog)
	if err != nil {
		return nil, wrapScriptCompileSourceError(err, sourceName)
	}
	setProtoSource(proto, sourceName)
	return proto, nil
}

func wrapScriptCompileSourceError(err error, sourceName string) error {
	if err == nil || sourceName == "" || strings.Contains(err.Error(), sourceName) {
		return err
	}
	return fmt.Errorf("%s: %w", sourceName, err)
}

func setProtoSource(proto *FuncProto, sourceName string) {
	if proto == nil {
		return
	}
	if proto.Source == "" {
		proto.Source = sourceName
	}
	for _, child := range proto.Protos {
		setProtoSource(child, sourceName)
	}
}

func (vm *VM) scriptConfigFromValue(opt runtime.Value, defaultSource string) (vmScriptConfig, error) {
	cfg := vmScriptConfig{sourceName: defaultSource}
	if vmScriptOptionIsNil(opt) {
		return cfg, nil
	}
	if opt.IsString() {
		cfg.sourceName = opt.Str()
		return cfg, nil
	}
	if !opt.IsTable() {
		return cfg, fmt.Errorf("script environment options must be a table, string, or nil")
	}
	tbl := opt.Table()
	if v := tbl.RawGetString("sourceName"); !v.IsNil() {
		if !v.IsString() {
			return cfg, fmt.Errorf("script environment option 'sourceName' must be a string")
		}
		cfg.sourceName = v.Str()
	}
	if v := tbl.RawGetString("source"); !v.IsNil() {
		if !v.IsString() {
			return cfg, fmt.Errorf("script environment option 'source' must be a string")
		}
		cfg.sourceName = v.Str()
	}
	if v := tbl.RawGetString("scriptDir"); !v.IsNil() {
		if !v.IsString() {
			return cfg, fmt.Errorf("script environment option 'scriptDir' must be a string")
		}
		cfg.scriptDir = v.Str()
	}
	envVal := tbl.RawGetString("env")
	if envVal.IsNil() {
		if !vmScriptOptionsTableHasConfigKeys(tbl) {
			envVal = opt
		}
	} else if !envVal.IsTable() {
		return cfg, fmt.Errorf("script environment option 'env' must be a table")
	}
	cfg.sandbox = tbl.RawGetString("sandbox").Truthy()
	if envVal.IsTable() {
		cfg.env = envVal.Table()
	}
	return cfg, nil
}

func vmScriptOptionIsNil(opt runtime.Value) bool {
	return opt.IsNil() || uint64(opt) == 0
}

func vmScriptConfigValue(cfg vmScriptConfig) runtime.Value {
	t := runtime.NewTable()
	if cfg.sourceName != "" {
		t.RawSetString("sourceName", runtime.StringValue(cfg.sourceName))
	}
	if cfg.scriptDir != "" {
		t.RawSetString("scriptDir", runtime.StringValue(cfg.scriptDir))
	}
	if cfg.env != nil {
		t.RawSetString("env", runtime.TableValue(cfg.env))
	}
	if cfg.sandbox {
		t.RawSetString("sandbox", runtime.BoolValue(true))
	}
	return runtime.TableValue(t)
}

func vmScriptEnvOptions(seed *runtime.Table, sandbox bool) *runtime.Table {
	opts := runtime.NewTable()
	opts.RawSetString("env", runtime.TableValue(seed))
	opts.RawSetString("sandbox", runtime.BoolValue(sandbox))
	return opts
}

func vmScriptOptionsTableHasConfigKeys(tbl *runtime.Table) bool {
	for _, key := range []string{"env", "sandbox", "sourceName", "source", "scriptDir"} {
		if !tbl.RawGetString(key).IsNil() {
			return true
		}
	}
	return false
}

func (vm *VM) executeScriptInChild(proto *FuncProto, cfg vmScriptConfig, args []runtime.Value) ([]runtime.Value, error) {
	base := make(map[string]runtime.Value)
	original := make(map[string]runtime.Value)
	originalSet := make(map[string]bool)
	if !cfg.sandbox {
		for name, val := range vm.globals {
			base[name] = val
			original[name] = val
			originalSet[name] = true
		}
	}
	envKeys := make(map[string]bool)
	if cfg.env != nil {
		k, v, ok := cfg.env.Next(runtime.NilValue())
		for ok {
			if k.IsString() {
				name := k.Str()
				envKeys[name] = true
				base[name] = v
				original[name] = v
				originalSet[name] = true
			}
			k, v, ok = cfg.env.Next(k)
		}
	}
	child := New(base)
	child.SetStringMeta(vm.stringMeta)
	child.scriptDir = cfg.scriptDir
	if child.scriptDir == "" {
		child.scriptDir = vm.scriptDir
	}
	cl := NewClosure(proto)
	var results []runtime.Value
	var err error
	if len(args) == 0 {
		results, err = child.Execute(proto)
	} else {
		results, err = child.callValue(runtime.VMClosureFunctionValue(unsafe.Pointer(cl), cl), args)
	}
	if cfg.env != nil {
		for name, idx := range child.globalIndex {
			if idx < 0 || idx >= len(child.globalArray) {
				continue
			}
			val := child.globalArray[idx]
			if cfg.sandbox || envKeys[name] || !originalSet[name] || original[name] != val {
				cfg.env.RawSetString(name, val)
			}
		}
	}
	return results, err
}

func (vm *VM) resolveScriptPath(filename string) string {
	return vm.resolveScriptPathWithDir(filename, vm.scriptDir)
}

func (vm *VM) resolveModulePath(name string) string {
	if idx := strings.Index(name, ":"); idx > 0 && vm.moduleCollections != nil {
		prefix := name[:idx]
		if root := vm.moduleCollections[prefix]; root != "" {
			rest := strings.ReplaceAll(name[idx+1:], ".", "/") + ".gs"
			return filepath.Join(root, rest)
		}
	}
	return vm.resolveScriptPath(strings.ReplaceAll(name, ".", "/") + ".gs")
}

func (vm *VM) resolveScriptPathWithDir(filename string, dir string) string {
	if filename == "" || filepath.IsAbs(filename) || dir == "" {
		return filename
	}
	candidate := filepath.Join(dir, filename)
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	return filename
}

func (vm *VM) SetScriptDir(dir string) {
	vm.scriptDir = dir
}

func (vm *VM) ScriptDir() string {
	return vm.scriptDir
}

func (vm *VM) SetModuleCollection(name, root string) {
	if name == "" {
		return
	}
	if vm.moduleCollections == nil {
		vm.moduleCollections = make(map[string]string)
	}
	vm.moduleCollections[name] = root
}

// Execute runs a top-level function prototype.
