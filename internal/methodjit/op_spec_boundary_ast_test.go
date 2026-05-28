package methodjit

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
)

func fileHasSelectorCall(path, selector string) (bool, error) {
	fileSet := token.NewFileSet()
	parsed, err := parser.ParseFile(fileSet, path, nil, 0)
	if err != nil {
		return false, err
	}
	found := false
	ast.Inspect(parsed, func(node ast.Node) bool {
		if found {
			return false
		}
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if ok && sel.Sel.Name == selector {
			found = true
			return false
		}
		return true
	})
	return found, nil
}

func funcHasIdentIndexSuffix(path, funcName, suffix string) (bool, error) {
	fileSet := token.NewFileSet()
	parsed, err := parser.ParseFile(fileSet, path, nil, 0)
	if err != nil {
		return false, err
	}
	for _, decl := range parsed.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != funcName {
			continue
		}
		return blockHasIdentIndexSuffix(fn.Body, suffix), nil
	}
	return false, os.ErrNotExist
}

func blockHasIdentIndexSuffix(body *ast.BlockStmt, suffix string) bool {
	found := false
	ast.Inspect(body, func(node ast.Node) bool {
		if found {
			return false
		}
		index, ok := node.(*ast.IndexExpr)
		if !ok {
			return true
		}
		ident, ok := index.X.(*ast.Ident)
		if ok && strings.HasSuffix(ident.Name, suffix) {
			found = true
			return false
		}
		return true
	})
	return found
}

func countLines(src []byte) int {
	if len(src) == 0 {
		return 0
	}
	lines := strings.Count(string(src), "\n")
	if src[len(src)-1] != '\n' {
		lines++
	}
	return lines
}
