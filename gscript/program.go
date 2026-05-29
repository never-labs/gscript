package gscript

import (
	"context"
	"os"
	"path/filepath"
	"sync"

	"github.com/gscript/gscript/internal/ast"
	"github.com/gscript/gscript/internal/lexer"
	"github.com/gscript/gscript/internal/parser"
	bytecodevm "github.com/gscript/gscript/internal/vm"
)

// Program is a compiled GScript source unit.
//
// Program hides parser, AST, bytecode, and JIT details from embedding callers.
// It may cache VM/JIT state while running, so callers should not run the same
// Program concurrently.
type Program struct {
	sourceName string
	scriptDir  string
	ast        *ast.Program
	protoMu    sync.Mutex
	proto      *bytecodevm.FuncProto
}

type compileOptions struct {
	sourceName string
}

// CompileOption configures Compile.
type CompileOption func(*compileOptions)

// WithSourceName sets the source name used in diagnostics for a compiled
// string. CompileFile uses the path by default.
func WithSourceName(name string) CompileOption {
	return func(o *compileOptions) {
		o.sourceName = name
	}
}

// Compile parses a GScript source string into a reusable Program.
func Compile(src string, opts ...CompileOption) (*Program, error) {
	cfg := compileOptions{sourceName: "<string>"}
	for _, opt := range opts {
		opt(&cfg)
	}
	return compileSource(src, cfg.sourceName, "")
}

// CompileContext is like Compile, but returns ctx.Err() if the context is
// already cancelled before or after parsing. It does not yet preempt parser work
// in the middle of a single parse.
func CompileContext(ctx context.Context, src string, opts ...CompileOption) (*Program, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	prog, err := Compile(src, opts...)
	if err != nil {
		return nil, err
	}
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	return prog, nil
}

// CompileFile reads and parses a GScript file into a reusable Program.
func CompileFile(path string, opts ...CompileOption) (*Program, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, newError(ErrRuntime, err, path)
	}
	cfg := compileOptions{sourceName: path}
	for _, opt := range opts {
		opt(&cfg)
	}
	abs, _ := filepath.Abs(path)
	return compileSource(string(src), cfg.sourceName, filepath.Dir(abs))
}

// CompileFileContext is like CompileFile, but returns ctx.Err() if the context
// is already cancelled before or after file parsing.
func CompileFileContext(ctx context.Context, path string, opts ...CompileOption) (*Program, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	prog, err := CompileFile(path, opts...)
	if err != nil {
		return nil, err
	}
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	return prog, nil
}

// SourceName returns the diagnostic source name attached to the Program.
func (p *Program) SourceName() string {
	if p == nil {
		return ""
	}
	return p.sourceName
}

func compileSource(src, filename, scriptDir string) (*Program, error) {
	tokens, err := lexer.New(src).Tokenize()
	if err != nil {
		return nil, newError(ErrLex, err, filename)
	}
	parsed, err := parser.New(tokens).Parse()
	if err != nil {
		return nil, newError(ErrParse, err, filename)
	}
	return &Program{
		sourceName: filename,
		scriptDir:  scriptDir,
		ast:        parsed,
	}, nil
}

func (p *Program) bytecodeProto() (*bytecodevm.FuncProto, error) {
	p.protoMu.Lock()
	defer p.protoMu.Unlock()
	if p.proto != nil {
		return p.proto, nil
	}
	proto, err := bytecodevm.Compile(p.ast)
	if err != nil {
		return nil, err
	}
	setBytecodeSource(proto, p.sourceName)
	p.proto = proto
	return proto, nil
}

func checkContext(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}
