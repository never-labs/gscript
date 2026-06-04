package parser

import (
	"testing"

	"github.com/never-labs/leia/internal/ast"
	"github.com/never-labs/leia/internal/lexer"
)

func TestTaggedDialectInterpolationParses(t *testing.T) {
	prog := mustParse(t, "shell := $`printf hello-${name}`")
	decl := prog.Stmts[0].(*ast.DeclareStmt)
	tagged := decl.Values[0].(*ast.TaggedStringExpr)
	if tagged.Tag != "sh" {
		t.Fatalf("tag = %q, want sh", tagged.Tag)
	}
	interp := tagged.Body.(*ast.InterpolatedStringExpr)
	if len(interp.Parts) != 2 {
		t.Fatalf("parts = %#v, want text + expr", interp.Parts)
	}
	if interp.Parts[0].Text != "printf hello-" {
		t.Fatalf("text part = %q", interp.Parts[0].Text)
	}
	if ident, ok := interp.Parts[1].Expr.(*ast.IdentExpr); !ok || ident.Name != "name" {
		t.Fatalf("expr part = %#v, want name ident", interp.Parts[1].Expr)
	}
}

func TestTaggedDialectStringFormsParse(t *testing.T) {
	prog := mustParse(t, "pattern := re!`^[a-z]+-${suffix}$`\nencoded := base64`${name}`")
	if len(prog.Stmts) != 2 {
		t.Fatalf("stmt count = %d, want 2", len(prog.Stmts))
	}

	pattern := prog.Stmts[0].(*ast.DeclareStmt).Values[0].(*ast.TaggedStringExpr)
	if pattern.Tag != "re" || !pattern.FailFast {
		t.Fatalf("pattern tag/failFast = %q/%v, want re/true", pattern.Tag, pattern.FailFast)
	}
	body, ok := pattern.Body.(*ast.InterpolatedStringExpr)
	if !ok || len(body.Parts) != 3 {
		t.Fatalf("pattern body = %#v, want interpolated text/expr/text", pattern.Body)
	}
	if body.Parts[0].Text != "^[a-z]+-" || body.Parts[2].Text != "$" {
		t.Fatalf("pattern text parts = %#v", body.Parts)
	}
	if ident, ok := body.Parts[1].Expr.(*ast.IdentExpr); !ok || ident.Name != "suffix" {
		t.Fatalf("pattern expr part = %#v, want suffix ident", body.Parts[1].Expr)
	}

	encoded := prog.Stmts[1].(*ast.DeclareStmt).Values[0].(*ast.TaggedStringExpr)
	if encoded.Tag != "base64" || encoded.FailFast {
		t.Fatalf("encoded tag/failFast = %q/%v, want base64/false", encoded.Tag, encoded.FailFast)
	}
}

func TestTaggedDialectBlocksParse(t *testing.T) {
	prog := mustParse(t, `
msg := prompt {
    role: "system"
    text: "Hello"
}
quoted := quote {
    x := 1
    x += 2
}
`)
	if len(prog.Stmts) != 2 {
		t.Fatalf("stmt count = %d, want 2", len(prog.Stmts))
	}

	promptBlock := prog.Stmts[0].(*ast.DeclareStmt).Values[0].(*ast.TaggedBlockExpr)
	if promptBlock.Tag != "prompt" || promptBlock.Body != nil || len(promptBlock.Config) != 2 {
		t.Fatalf("prompt block = %#v, want field block with 2 fields", promptBlock)
	}
	roleKey, ok := promptBlock.Config[0].Key.(*ast.StringLit)
	if !ok || roleKey.Value != "role" {
		t.Fatalf("prompt first key = %#v, want role string key", promptBlock.Config[0].Key)
	}
	textValue, ok := promptBlock.Config[1].Value.(*ast.StringLit)
	if !ok || textValue.Value != "Hello" {
		t.Fatalf("prompt text value = %#v, want string value", promptBlock.Config[1].Value)
	}

	quoteBlock := prog.Stmts[1].(*ast.DeclareStmt).Values[0].(*ast.TaggedBlockExpr)
	if quoteBlock.Tag != "quote" || quoteBlock.Body == nil || len(quoteBlock.Body.Stmts) != 2 || len(quoteBlock.Config) != 0 {
		t.Fatalf("quote block = %#v, want raw block with 2 statements", quoteBlock)
	}
}

func TestGoStyleImportParses(t *testing.T) {
	src := `
import "json"
import p "path"
import (
    "regexp"
    fs "fs"
)
`
	tokens, err := lexer.New(src).Tokenize()
	if err != nil {
		t.Fatalf("Tokenize: %v", err)
	}
	prog, err := New(tokens).Parse()
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	block, ok := prog.Stmts[2].(*ast.BlockStmt)
	if !ok {
		t.Fatalf("third stmt = %T, want block import decl", prog.Stmts[2])
	}
	if len(prog.Stmts) != 3 || len(block.Stmts) != 2 {
		t.Fatalf("stmt counts = top %d block %d", len(prog.Stmts), len(block.Stmts))
	}
	want := []string{"json", "p", "regexp", "fs"}
	got := []string{
		prog.Stmts[0].(*ast.DeclareStmt).Names[0],
		prog.Stmts[1].(*ast.DeclareStmt).Names[0],
		block.Stmts[0].(*ast.DeclareStmt).Names[0],
		block.Stmts[1].(*ast.DeclareStmt).Names[0],
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("import aliases = %#v, want %#v", got, want)
		}
	}
}
