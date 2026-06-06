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

func TestShellShortcutFailFastParses(t *testing.T) {
	prog := mustParse(t, "shell := $!`printf hello`")
	decl := prog.Stmts[0].(*ast.DeclareStmt)
	tagged := decl.Values[0].(*ast.TaggedStringExpr)
	if tagged.Tag != "sh" || !tagged.FailFast {
		t.Fatalf("tag/failFast = %q/%v, want sh/true", tagged.Tag, tagged.FailFast)
	}
}

func TestTaggedDialectInterpolationParsesNestedExpressionStrings(t *testing.T) {
	prog := mustParse(t, "msg := prompt`value=${choose({right: \"}\", left: \"{\"})}`")
	decl := prog.Stmts[0].(*ast.DeclareStmt)
	tagged := decl.Values[0].(*ast.TaggedStringExpr)
	interp := tagged.Body.(*ast.InterpolatedStringExpr)
	if len(interp.Parts) != 2 {
		t.Fatalf("parts = %#v, want text + expr", interp.Parts)
	}
	if interp.Parts[0].Text != "value=" {
		t.Fatalf("text part = %q, want value=", interp.Parts[0].Text)
	}
	call, ok := interp.Parts[1].Expr.(*ast.CallExpr)
	if !ok {
		t.Fatalf("expr part = %T, want call", interp.Parts[1].Expr)
	}
	if ident, ok := call.Func.(*ast.IdentExpr); !ok || ident.Name != "choose" {
		t.Fatalf("call func = %#v, want choose ident", call.Func)
	}
	if len(call.Args) != 1 {
		t.Fatalf("call args = %d, want 1", len(call.Args))
	}
	table, ok := call.Args[0].(*ast.TableLitExpr)
	if !ok || len(table.Fields) != 2 {
		t.Fatalf("call arg = %#v, want table with two fields", call.Args[0])
	}
}

func TestTaggedDialectStringsRequireRawStringLiteral(t *testing.T) {
	for _, src := range []string{
		`msg := prompt"hello"`,
		`msg := prompt!"hello"`,
		`shell := $"printf hello"`,
		`shell := $!"printf hello"`,
	} {
		t.Run(src, func(t *testing.T) {
			mustFail(t, src)
		})
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

func TestTaggedDialectFailFastRawBlockParses(t *testing.T) {
	prog := mustParse(t, "quoted := quote! {\n    x := 1\n    x += 2\n}\n")
	decl := prog.Stmts[0].(*ast.DeclareStmt)
	tagged := decl.Values[0].(*ast.TaggedBlockExpr)
	if tagged.Tag != "quote" || !tagged.FailFast {
		t.Fatalf("tag/failFast = %q/%v, want quote/true", tagged.Tag, tagged.FailFast)
	}
	if tagged.Body == nil || len(tagged.Body.Stmts) != 2 || len(tagged.Config) != 0 {
		t.Fatalf("tagged block = %#v, want fail-fast raw block with 2 statements", tagged)
	}
}

func TestTaggedDialectExpressionStatementsLowerToDialectCalls(t *testing.T) {
	prog := mustParse(t, "prompt`hello ${name}`\nprompt {\n    role: \"system\"\n}\nquote! {\n    x := 1\n}\n")
	if len(prog.Stmts) != 3 {
		t.Fatalf("stmt count = %d, want 3", len(prog.Stmts))
	}
	want := []struct {
		method   string
		tag      string
		failFast bool
	}{
		{method: "eval", tag: "prompt"},
		{method: "eval_block", tag: "prompt"},
		{method: "eval_raw", tag: "quote", failFast: true},
	}
	for i, tt := range want {
		callStmt, ok := prog.Stmts[i].(*ast.CallStmt)
		if !ok {
			t.Fatalf("stmt[%d] = %T, want CallStmt", i, prog.Stmts[i])
		}
		call, ok := callStmt.Call.(*ast.CallExpr)
		if !ok {
			t.Fatalf("stmt[%d] call = %T, want CallExpr", i, callStmt.Call)
		}
		field, ok := call.Func.(*ast.FieldExpr)
		if !ok {
			t.Fatalf("stmt[%d] func = %T, want FieldExpr", i, call.Func)
		}
		if recv, ok := field.Table.(*ast.IdentExpr); !ok || recv.Name != "dialect" || field.Field != tt.method {
			t.Fatalf("stmt[%d] call func = %#v, want dialect.%s", i, call.Func, tt.method)
		}
		tag, ok := call.Args[0].(*ast.StringLit)
		if !ok || tag.Value != tt.tag {
			t.Fatalf("stmt[%d] tag arg = %#v, want %q", i, call.Args[0], tt.tag)
		}
		opts, ok := call.Args[2].(*ast.TableLitExpr)
		if !ok || len(opts.Fields) != 1 {
			t.Fatalf("stmt[%d] opts = %#v, want fail_fast option", i, call.Args[2])
		}
		failFast, ok := opts.Fields[0].Value.(*ast.BoolLit)
		if !ok || failFast.Value != tt.failFast {
			t.Fatalf("stmt[%d] fail_fast = %#v, want %v", i, opts.Fields[0].Value, tt.failFast)
		}
	}
	rawCall, ok := prog.Stmts[2].(*ast.CallStmt).Call.(*ast.CallExpr)
	if !ok {
		t.Fatalf("raw block call = %T, want CallExpr", prog.Stmts[2].(*ast.CallStmt).Call)
	}
	if _, ok := rawCall.Args[1].(*ast.FuncLitExpr); !ok {
		t.Fatalf("raw block body = %T, want FuncLitExpr", rawCall.Args[1])
	}
}

func TestEvaluateStatementParses(t *testing.T) {
	prog := mustParse(t, "evaluate \"replay fixture\" {\n    assert(true)\n}\n")
	if len(prog.Stmts) != 1 {
		t.Fatalf("stmt count = %d, want 1", len(prog.Stmts))
	}
	eval, ok := prog.Stmts[0].(*ast.EvaluateStmt)
	if !ok {
		t.Fatalf("stmt = %T, want EvaluateStmt", prog.Stmts[0])
	}
	if eval.Name != "replay fixture" || eval.Body == nil || len(eval.Body.Stmts) != 1 {
		t.Fatalf("evaluate stmt = %#v, want named body with one assertion", eval)
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
