package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/never-labs/leia/internal/ast"
	"github.com/never-labs/leia/internal/lexer"
	"github.com/never-labs/leia/internal/parser"
	bytecodevm "github.com/never-labs/leia/internal/vm"
)

func runInspectCommand(args []string, outw, errw io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(errw, "usage: leia inspect bytecode [--proto NAME] <file.leia>")
		fmt.Fprintln(errw, "       leia inspect directives [--json] <file.leia>")
		return 2
	}
	switch args[0] {
	case "bytecode":
		return runInspectBytecodeCommand(args[1:], outw, errw)
	case "directives":
		return runInspectDirectivesCommand(args[1:], outw, errw)
	case "help", "-h", "--help":
		fmt.Fprintln(outw, "usage: leia inspect bytecode [--proto NAME] <file.leia>")
		fmt.Fprintln(outw, "       leia inspect directives [--json] <file.leia>")
		return 0
	default:
		fmt.Fprintf(errw, "leia inspect: unknown mode %q (want bytecode or directives)\n", args[0])
		return 2
	}
}

func runInspectBytecodeCommand(args []string, outw, errw io.Writer) int {
	fs := flag.NewFlagSet("inspect bytecode", flag.ContinueOnError)
	fs.SetOutput(errw)
	protoName := fs.String("proto", "", "dump only a named proto")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	paths := fs.Args()
	if len(paths) != 1 {
		fmt.Fprintln(errw, "usage: leia inspect bytecode [--proto NAME] <file.leia>")
		return 2
	}
	proto, err := compileFileForInspect(paths[0])
	if err != nil {
		fmt.Fprintf(errw, "%s: %v\n", paths[0], err)
		return 1
	}
	if *protoName != "" {
		target := findInspectProto(proto, *protoName)
		if target == nil {
			fmt.Fprintf(errw, "leia inspect bytecode: proto %q not found\n", *protoName)
			return 1
		}
		fmt.Fprint(outw, bytecodevm.Disassemble(target))
		return 0
	}
	dumpInspectProto(outw, "<main>", proto, 0)
	return 0
}

type inspectFileDirective struct {
	Kind   string   `json:"kind"`
	Args   []string `json:"args,omitempty"`
	Text   string   `json:"text,omitempty"`
	Line   int      `json:"line"`
	Column int      `json:"column"`
}

func runInspectDirectivesCommand(args []string, outw, errw io.Writer) int {
	fs := flag.NewFlagSet("inspect directives", flag.ContinueOnError)
	fs.SetOutput(errw)
	jsonOut := fs.Bool("json", false, "print directives as JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	paths := fs.Args()
	if len(paths) != 1 {
		fmt.Fprintln(errw, "usage: leia inspect directives [--json] <file.leia>")
		return 2
	}
	directives, err := parseFileDirectivesForInspect(paths[0])
	if err != nil {
		fmt.Fprintf(errw, "%s: %v\n", paths[0], err)
		return 1
	}
	if *jsonOut {
		enc := json.NewEncoder(outw)
		enc.SetIndent("", "  ")
		if err := enc.Encode(inspectDirectiveRows(directives)); err != nil {
			fmt.Fprintf(errw, "leia inspect directives: write json: %v\n", err)
			return 1
		}
		return 0
	}
	for _, directive := range directives {
		if directive.Text == "" {
			fmt.Fprintf(outw, "%d:%d %s\n", directive.P.Line, directive.P.Column, directive.Kind)
			continue
		}
		fmt.Fprintf(outw, "%d:%d %s %s\n", directive.P.Line, directive.P.Column, directive.Kind, directive.Text)
	}
	return 0
}

func inspectDirectiveRows(directives []ast.FileDirective) []inspectFileDirective {
	out := make([]inspectFileDirective, 0, len(directives))
	for _, directive := range directives {
		out = append(out, inspectFileDirective{
			Kind:   directive.Kind,
			Args:   directive.Args,
			Text:   directive.Text,
			Line:   directive.P.Line,
			Column: directive.P.Column,
		})
	}
	return out
}

func parseFileDirectivesForInspect(filename string) ([]ast.FileDirective, error) {
	src, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	tokens, err := lexer.New(string(src)).Tokenize()
	if err != nil {
		return nil, fmt.Errorf("lexer error: %w", err)
	}
	prog, err := parser.New(tokens).Parse()
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}
	return prog.FileDirectives, nil
}

func compileFileForInspect(filename string) (*bytecodevm.FuncProto, error) {
	src, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	tokens, err := lexer.New(string(src)).Tokenize()
	if err != nil {
		return nil, fmt.Errorf("lexer error: %w", err)
	}
	prog, err := parser.New(tokens).Parse()
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}
	proto, err := bytecodevm.Compile(prog)
	if err != nil {
		return nil, fmt.Errorf("compile error: %w", err)
	}
	return proto, nil
}

func findInspectProto(proto *bytecodevm.FuncProto, name string) *bytecodevm.FuncProto {
	if proto == nil {
		return nil
	}
	if name == "<main>" {
		return proto
	}
	for _, child := range proto.Protos {
		if child.Name == name {
			return child
		}
		if found := findInspectProto(child, name); found != nil {
			return found
		}
	}
	return nil
}

func dumpInspectProto(w io.Writer, name string, proto *bytecodevm.FuncProto, depth int) {
	if proto == nil {
		return
	}
	indent := strings.Repeat("  ", depth)
	tier1 := proto.MethodJITTier1CallableDecision()
	tier2 := proto.MethodJITTier2CallableDecision()
	fmt.Fprintf(w, "%s=== %s (params=%d, maxstack=%d, jit_disabled=%v, is_vararg=%v, uses_vararg=%v, tier1=%v/%s, tier2=%v/%s) ===\n",
		indent,
		name,
		proto.NumParams,
		proto.MaxStack,
		proto.JITDisabled,
		proto.IsVarArg,
		proto.UsesVarargBytecode,
		tier1.Allowed,
		tier1.Reason,
		tier2.Allowed,
		tier2.Reason,
	)
	fmt.Fprintln(w, bytecodevm.Disassemble(proto))
	for _, child := range proto.Protos {
		dumpInspectProto(w, child.Name, child, depth+1)
	}
}
