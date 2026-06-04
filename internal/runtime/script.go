package runtime

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/never-labs/leia/internal/ast"
	"github.com/never-labs/leia/internal/lexer"
	"github.com/never-labs/leia/internal/parser"
)

// ScriptOptions configures runtime source compilation and execution.
type ScriptOptions struct {
	Env        *Environment
	SourceName string
	ScriptDir  string
	Isolated   bool
}

// ScriptOption configures runtime source compilation and execution.
type ScriptOption func(*ScriptOptions)

// WithScriptEnv runs the source with env as its lexical global environment.
func WithScriptEnv(env *Environment) ScriptOption {
	return func(opts *ScriptOptions) { opts.Env = env }
}

// WithScriptSourceName sets the diagnostic/source name for compiled chunks.
func WithScriptSourceName(name string) ScriptOption {
	return func(opts *ScriptOptions) { opts.SourceName = name }
}

// WithScriptDir sets the script directory while the chunk runs.
func WithScriptDir(dir string) ScriptOption {
	return func(opts *ScriptOptions) { opts.ScriptDir = dir }
}

// WithIsolatedEnv prevents the execution environment from falling back to the
// interpreter's existing globals. The provided env must contain any allowed API.
func WithIsolatedEnv() ScriptOption {
	return func(opts *ScriptOptions) { opts.Isolated = true }
}

// NewScriptEnvironment creates a lexical environment seeded from vars.
func (interp *Interpreter) NewScriptEnvironment(vars map[string]Value, isolated bool) *Environment {
	var parent *Environment
	if !isolated {
		parent = interp.globals
	}
	env := NewEnvironment(parent)
	for name, val := range vars {
		env.Define(name, val)
	}
	return env
}

// CompileString parses src and returns a callable chunk. Calling the chunk
// executes the generated program and returns any top-level return values.
func (interp *Interpreter) CompileString(src string, options ...ScriptOption) (Value, error) {
	opts := interp.scriptOptions(options...)
	prog, err := parseProgram(src, opts.SourceName)
	if err != nil {
		return NilValue(), err
	}
	return interp.compileProgram(prog, opts, nil), nil
}

// EvalString parses and immediately executes src with optional env controls.
func (interp *Interpreter) EvalString(src string, options ...ScriptOption) ([]Value, error) {
	opts := interp.scriptOptions(options...)
	prog, err := parseProgram(src, opts.SourceName)
	if err != nil {
		return nil, err
	}
	return interp.execProgramWithOptions(prog, opts, nil)
}

// LoadFile parses a file and returns a callable chunk without running it.
func (interp *Interpreter) LoadFile(filename string, options ...ScriptOption) (Value, error) {
	if !interp.filesystemEnabled {
		return NilValue(), fmt.Errorf("filesystem access disabled")
	}
	filename = interp.resolveScriptPath(filename)
	resolved, err := interp.resolveFilesystemPath(filename)
	if err != nil {
		return NilValue(), err
	}
	filename = resolved
	if err := interp.checkModuleFileBudget(filename); err != nil {
		return NilValue(), err
	}
	src, err := os.ReadFile(filename)
	if err != nil {
		return NilValue(), fmt.Errorf("cannot open %s: %s", filename, err)
	}
	opts := interp.scriptOptions(options...)
	if opts.SourceName == "" {
		opts.SourceName = filename
	}
	if opts.ScriptDir == "" {
		if abs, err := filepath.Abs(filename); err == nil {
			opts.ScriptDir = filepath.Dir(abs)
		}
	}
	prog, err := parseProgram(string(src), opts.SourceName)
	if err != nil {
		return NilValue(), err
	}
	return interp.compileProgram(prog, opts, nil), nil
}

// RunFile reads, parses, and executes a file with optional env controls.
func (interp *Interpreter) RunFile(filename string, options ...ScriptOption) ([]Value, error) {
	fn, err := interp.LoadFile(filename, options...)
	if err != nil {
		return nil, err
	}
	return interp.callFunction(fn, nil)
}

func parseProgram(src string, sourceName string) (*ast.Program, error) {
	tokens, err := lexer.New(src).Tokenize()
	if err != nil {
		return nil, wrapCompileError(err, sourceName)
	}
	prog, err := parser.New(tokens).Parse()
	if err != nil {
		return nil, wrapCompileError(err, sourceName)
	}
	return prog, nil
}

func (interp *Interpreter) scriptOptions(options ...ScriptOption) ScriptOptions {
	opts := ScriptOptions{Env: interp.globals}
	for _, opt := range options {
		opt(&opts)
	}
	if opts.Env == nil {
		opts.Env = NewEnvironment(nil)
	}
	return opts
}

func (interp *Interpreter) compileProgram(prog *ast.Program, opts ScriptOptions, syncBack func()) Value {
	return FunctionValue(&GoFunction{
		Name: "script.chunk",
		Fn: func(args []Value) ([]Value, error) {
			results, err := interp.execProgramWithOptions(prog, opts, syncBack)
			if err != nil {
				return nil, err
			}
			return results, nil
		},
	})
}

func (interp *Interpreter) execProgramWithOptions(prog *ast.Program, opts ScriptOptions, syncBack func()) ([]Value, error) {
	env := opts.Env
	if env == nil {
		env = interp.globals
	}

	oldGlobals := interp.globals
	oldScriptDir := interp.scriptDir
	oldSourceName := interp.currentSourceName
	if env != nil {
		interp.globals = env
	}
	if opts.ScriptDir != "" {
		interp.scriptDir = opts.ScriptDir
	}
	interp.currentSourceName = opts.SourceName
	defer func() {
		interp.globals = oldGlobals
		interp.scriptDir = oldScriptDir
		interp.currentSourceName = oldSourceName
		if syncBack != nil {
			syncBack()
		}
	}()

	return interp.execProgram(prog, env)
}

func (interp *Interpreter) execProgram(prog *ast.Program, env *Environment) ([]Value, error) {
	prog = ast.DesugarSyntax(prog)
	var lastRet []Value
	interp.pushDeferFrame()
	for _, stmt := range prog.Stmts {
		retVals, isRet, _, _, err := interp.execStmt(stmt, env)
		if err != nil {
			_ = interp.runAndPopDeferFrame()
			return nil, err
		}
		if isRet {
			lastRet = retVals
			break
		}
	}
	if err := interp.runAndPopDeferFrame(); err != nil {
		return nil, err
	}
	return lastRet, nil
}

func (interp *Interpreter) resolveScriptPath(filename string) string {
	if filename == "" || filepath.IsAbs(filename) || interp.scriptDir == "" {
		return filename
	}
	candidate := filepath.Join(interp.scriptDir, filename)
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	return filename
}

func buildScriptLib(interp *Interpreter) *Table {
	t := NewTable()

	set := func(name string, fn func([]Value) ([]Value, error)) {
		t.RawSet(StringValue(name), FunctionValue(&GoFunction{
			Name: "script." + name,
			Fn:   fn,
		}))
	}

	set("env", func(args []Value) ([]Value, error) {
		seed := NewTable()
		if len(args) >= 1 && !args[0].IsNil() {
			if !args[0].IsTable() {
				return nil, fmt.Errorf("bad argument #1 to 'script.env' (table expected)")
			}
			seed = args[0].Table()
		}
		return []Value{TableValue(scriptEnvOptions(seed, false))}, nil
	})

	set("sandbox", func(args []Value) ([]Value, error) {
		seed := NewTable()
		if len(args) >= 1 && !args[0].IsNil() {
			if !args[0].IsTable() {
				return nil, fmt.Errorf("bad argument #1 to 'script.sandbox' (table expected)")
			}
			seed = args[0].Table()
		}
		return []Value{TableValue(scriptEnvOptions(seed, true))}, nil
	})

	set("compile", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'script.compile' (string expected)")
		}
		if !interp.dynamicEval {
			return nil, fmt.Errorf("dynamic eval disabled")
		}
		var opt Value
		if len(args) >= 2 {
			opt = args[1]
		}
		fn, err := interp.compileStringWithConfig(args[0].Str(), opt, "<script.compile>")
		if err != nil {
			return nil, err
		}
		return []Value{fn}, nil
	})

	set("eval", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'script.eval' (string expected)")
		}
		if !interp.dynamicEval {
			return nil, fmt.Errorf("dynamic eval disabled")
		}
		var opt Value
		if len(args) >= 2 {
			opt = args[1]
		}
		cfg, err := interp.scriptConfigFromValue(opt, "<script.eval>")
		if err != nil {
			return nil, err
		}
		prog, err := parseProgram(args[0].Str(), cfg.opts.SourceName)
		if err != nil {
			return nil, err
		}
		return interp.execProgramWithOptions(prog, cfg.opts, cfg.syncBack)
	})

	set("loadFile", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'script.loadFile' (string expected)")
		}
		var opt Value
		if len(args) >= 2 {
			opt = args[1]
		}
		fn, err := interp.loadFileWithConfig(args[0].Str(), opt)
		if err != nil {
			return nil, err
		}
		return []Value{fn}, nil
	})

	set("runFile", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'script.runFile' (string expected)")
		}
		var opt Value
		if len(args) >= 2 {
			opt = args[1]
		}
		cfg, err := interp.scriptConfigFromValue(opt, args[0].Str())
		if err != nil {
			return nil, err
		}
		if cfg.opts.ScriptDir == "" {
			resolved := interp.resolveScriptPath(args[0].Str())
			if abs, err := filepath.Abs(resolved); err == nil {
				cfg.opts.ScriptDir = filepath.Dir(abs)
			}
		}
		fn, err := interp.LoadFile(args[0].Str(), func(opts *ScriptOptions) {
			*opts = cfg.opts
		})
		if err != nil {
			return nil, err
		}
		return interp.callFunction(fn, nil)
	})

	set("dir", func(args []Value) ([]Value, error) {
		return []Value{StringValue(interp.ScriptDir())}, nil
	})

	set("setDir", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'script.setDir' (string expected)")
		}
		old := interp.ScriptDir()
		interp.SetScriptDir(args[0].Str())
		return []Value{StringValue(old)}, nil
	})

	return t
}

type scriptConfig struct {
	opts     ScriptOptions
	syncBack func()
}

func (interp *Interpreter) compileStringWithConfig(src string, opt Value, sourceName string) (Value, error) {
	cfg, err := interp.scriptConfigFromValue(opt, sourceName)
	if err != nil {
		return NilValue(), err
	}
	prog, err := parseProgram(src, cfg.opts.SourceName)
	if err != nil {
		return NilValue(), err
	}
	return interp.compileProgram(prog, cfg.opts, cfg.syncBack), nil
}

func (interp *Interpreter) loadFileWithConfig(filename string, opt Value) (Value, error) {
	cfg, err := interp.scriptConfigFromValue(opt, filename)
	if err != nil {
		return NilValue(), err
	}
	resolved := interp.resolveScriptPath(filename)
	if cfg.opts.ScriptDir == "" {
		if abs, err := filepath.Abs(resolved); err == nil {
			cfg.opts.ScriptDir = filepath.Dir(abs)
		}
	}
	return interp.LoadFile(resolved, func(opts *ScriptOptions) {
		*opts = cfg.opts
	})
}

func (interp *Interpreter) scriptConfigFromValue(opt Value, sourceName string) (scriptConfig, error) {
	cfg := scriptConfig{
		opts: ScriptOptions{
			Env:        interp.globals,
			SourceName: sourceName,
		},
	}
	if opt.IsNil() {
		return cfg, nil
	}
	if opt.IsString() {
		cfg.opts.SourceName = opt.Str()
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
		cfg.opts.SourceName = v.Str()
	}
	if v := tbl.RawGetString("source"); !v.IsNil() {
		if !v.IsString() {
			return cfg, fmt.Errorf("script environment option 'source' must be a string")
		}
		cfg.opts.SourceName = v.Str()
	}
	if v := tbl.RawGetString("scriptDir"); !v.IsNil() {
		if !v.IsString() {
			return cfg, fmt.Errorf("script environment option 'scriptDir' must be a string")
		}
		cfg.opts.ScriptDir = v.Str()
	}
	envVal := tbl.RawGetString("env")
	if envVal.IsNil() {
		if !scriptOptionsTableHasConfigKeys(tbl) {
			envVal = opt
		}
	} else if !envVal.IsTable() {
		return cfg, fmt.Errorf("script environment option 'env' must be a table")
	}
	if envVal.IsNil() {
		return cfg, nil
	}
	envTable := envVal.Table()
	sandbox := tbl.RawGetString("sandbox").Truthy()

	var parent *Environment
	if !sandbox {
		parent = interp.globals
	}
	env := environmentFromTable(envTable, parent)
	cfg.opts.Env = env
	cfg.opts.Isolated = sandbox
	cfg.syncBack = func() {
		copyEnvironmentToTable(env, envTable)
	}
	return cfg, nil
}

func scriptEnvOptions(seed *Table, sandbox bool) *Table {
	opts := NewTable()
	opts.RawSetString("env", TableValue(seed))
	opts.RawSetString("sandbox", BoolValue(sandbox))
	return opts
}

func scriptOptionsTableHasConfigKeys(tbl *Table) bool {
	for _, key := range []string{"env", "sandbox", "sourceName", "source", "scriptDir"} {
		if !tbl.RawGetString(key).IsNil() {
			return true
		}
	}
	return false
}

func environmentFromTable(tbl *Table, parent *Environment) *Environment {
	env := NewEnvironment(parent)
	k, v, ok := tbl.Next(NilValue())
	for ok {
		if k.IsString() {
			env.Define(k.Str(), v)
		}
		k, v, ok = tbl.Next(k)
	}
	return env
}

func copyEnvironmentToTable(env *Environment, tbl *Table) {
	for name, uv := range env.vars {
		tbl.RawSetString(name, uv.Get())
	}
}
