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
		fmt.Fprintln(errw, "usage: leia inspect bytecode [--json] [--proto NAME] <file.leia>")
		fmt.Fprintln(errw, "       leia inspect directives [--json] <file.leia>")
		return 2
	}
	switch args[0] {
	case "bytecode":
		return runInspectBytecodeCommand(args[1:], outw, errw)
	case "directives":
		return runInspectDirectivesCommand(args[1:], outw, errw)
	case "help", "-h", "--help":
		fmt.Fprintln(outw, "usage: leia inspect bytecode [--json] [--proto NAME] <file.leia>")
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
	jsonOut := fs.Bool("json", false, "print bytecode metadata as JSON")
	protoName := fs.String("proto", "", "dump only a named proto")
	if code, done := parseCLIFlags(fs, args); done {
		return code
	}
	paths := fs.Args()
	if len(paths) != 1 {
		fmt.Fprintln(errw, "usage: leia inspect bytecode [--json] [--proto NAME] <file.leia>")
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
		if *jsonOut {
			if err := writeInspectBytecodeJSON(outw, paths[0], *protoName, target, false); err != nil {
				fmt.Fprintf(errw, "leia inspect bytecode: write json: %v\n", err)
				return 1
			}
			return 0
		}
		fmt.Fprint(outw, bytecodevm.Disassemble(target))
		return 0
	}
	if *jsonOut {
		if err := writeInspectBytecodeJSON(outw, paths[0], "<main>", proto, true); err != nil {
			fmt.Fprintf(errw, "leia inspect bytecode: write json: %v\n", err)
			return 1
		}
		return 0
	}
	dumpInspectProto(outw, "<main>", proto, 0)
	return 0
}

type inspectBytecodeReport struct {
	SchemaVersion int                `json:"schema_version"`
	OK            bool               `json:"ok"`
	Status        string             `json:"status"`
	Source        string             `json:"source"`
	SelectedProto string             `json:"selected_proto"`
	Recursive     bool               `json:"recursive"`
	ProtoCount    int                `json:"proto_count"`
	Proto         inspectProtoReport `json:"proto"`
}

type inspectProtoReport struct {
	Name               string                   `json:"name"`
	DisplayName        string                   `json:"display_name"`
	Source             string                   `json:"source,omitempty"`
	LineDefined        int                      `json:"line_defined"`
	NumParams          int                      `json:"num_params"`
	MaxStack           int                      `json:"max_stack"`
	InstructionCount   int                      `json:"instruction_count"`
	ConstantCount      int                      `json:"constant_count"`
	UpvalueCount       int                      `json:"upvalue_count"`
	ChildProtoCount    int                      `json:"child_proto_count"`
	JITDisabled        bool                     `json:"jit_disabled"`
	IsVarArg           bool                     `json:"is_vararg"`
	UsesVarargBytecode bool                     `json:"uses_vararg_bytecode"`
	LeafNoCall         bool                     `json:"leaf_no_call"`
	NoGlobalOps        bool                     `json:"no_global_ops"`
	Tier1              inspectJITDecisionReport `json:"tier1"`
	Tier2              inspectJITDecisionReport `json:"tier2"`
	Disassembly        string                   `json:"disassembly"`
	Children           []inspectProtoReport     `json:"children,omitempty"`
}

type inspectJITDecisionReport struct {
	Allowed            bool   `json:"allowed"`
	Reason             string `json:"reason"`
	IsVarArg           bool   `json:"is_vararg"`
	UsesVarargBytecode bool   `json:"uses_vararg_bytecode"`
}

func writeInspectBytecodeJSON(w io.Writer, source, selected string, proto *bytecodevm.FuncProto, recursive bool) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(inspectBytecodeReport{
		SchemaVersion: 1,
		OK:            true,
		Status:        "pass",
		Source:        source,
		SelectedProto: selected,
		Recursive:     recursive,
		ProtoCount:    inspectProtoCount(proto, recursive),
		Proto:         inspectProtoRow(selected, proto, recursive),
	})
}

func inspectProtoCount(proto *bytecodevm.FuncProto, recursive bool) int {
	if proto == nil {
		return 0
	}
	if !recursive {
		return 1
	}
	count := 1
	for _, child := range proto.Protos {
		count += inspectProtoCount(child, true)
	}
	return count
}

func inspectProtoRow(displayName string, proto *bytecodevm.FuncProto, recursive bool) inspectProtoReport {
	if proto == nil {
		return inspectProtoReport{DisplayName: displayName}
	}
	tier1 := proto.MethodJITTier1CallableDecision()
	tier2 := proto.MethodJITTier2CallableDecision()
	row := inspectProtoReport{
		Name:               proto.Name,
		DisplayName:        displayName,
		Source:             proto.Source,
		LineDefined:        proto.LineDefined,
		NumParams:          proto.NumParams,
		MaxStack:           proto.MaxStack,
		InstructionCount:   len(proto.Code),
		ConstantCount:      len(proto.Constants),
		UpvalueCount:       len(proto.Upvalues),
		ChildProtoCount:    len(proto.Protos),
		JITDisabled:        proto.JITDisabled,
		IsVarArg:           proto.IsVarArg,
		UsesVarargBytecode: proto.UsesVarargBytecode,
		LeafNoCall:         proto.LeafNoCall,
		NoGlobalOps:        proto.NoGlobalOps,
		Tier1:              inspectJITDecisionRow(tier1),
		Tier2:              inspectJITDecisionRow(tier2),
		Disassembly:        bytecodevm.Disassemble(proto),
	}
	if recursive {
		row.Children = make([]inspectProtoReport, 0, len(proto.Protos))
		for _, child := range proto.Protos {
			row.Children = append(row.Children, inspectProtoRow(child.Name, child, true))
		}
	}
	return row
}

func inspectJITDecisionRow(decision bytecodevm.MethodJITCallableDecision) inspectJITDecisionReport {
	return inspectJITDecisionReport{
		Allowed:            decision.Allowed,
		Reason:             decision.Reason,
		IsVarArg:           decision.IsVarArg,
		UsesVarargBytecode: decision.UsesVarargBytecode,
	}
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
	if code, done := parseCLIFlags(fs, args); done {
		return code
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
		rows := inspectDirectiveRows(directives)
		if err := enc.Encode(inspectDirectivesReport{
			SchemaVersion:  1,
			OK:             true,
			Status:         "pass",
			Source:         paths[0],
			DirectiveCount: len(rows),
			Directives:     rows,
		}); err != nil {
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

type inspectDirectivesReport struct {
	SchemaVersion  int                    `json:"schema_version"`
	OK             bool                   `json:"ok"`
	Status         string                 `json:"status"`
	Source         string                 `json:"source"`
	DirectiveCount int                    `json:"directive_count"`
	Directives     []inspectFileDirective `json:"directives"`
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
