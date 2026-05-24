package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/gscript/gscript/internal/lexer"
	"github.com/gscript/gscript/internal/parser"
	"github.com/gscript/gscript/internal/vm"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: dump_bytecode <file.gs>")
		os.Exit(1)
	}
	src, _ := os.ReadFile(os.Args[1])
	tokens, _ := lexer.New(string(src)).Tokenize()
	prog, _ := parser.New(tokens).Parse()
	proto, _ := vm.Compile(prog)
	dumpProto("<main>", proto, 0)
}

func dumpProto(name string, proto *vm.FuncProto, depth int) {
	if proto == nil {
		return
	}
	indent := strings.Repeat("  ", depth)
	tier1 := proto.MethodJITTier1CallableDecision()
	tier2 := proto.MethodJITTier2CallableDecision()
	fmt.Printf("%s=== %s (params=%d, maxstack=%d, jit_disabled=%v, is_vararg=%v, uses_vararg=%v, tier1=%v/%s, tier2=%v/%s) ===\n",
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
	fmt.Println(vm.Disassemble(proto))
	for _, child := range proto.Protos {
		dumpProto(child.Name, child, depth+1)
	}
}
