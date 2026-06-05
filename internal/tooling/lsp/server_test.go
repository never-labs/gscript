package lsp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
)

func TestServerInitializeShutdown(t *testing.T) {
	input := mustEncodeMessages(t,
		map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{}},
		map[string]any{"jsonrpc": "2.0", "method": "initialized"},
		map[string]any{"jsonrpc": "2.0", "id": 2, "method": "shutdown"},
	)
	var out bytes.Buffer
	err := NewServer().Run(context.Background(), bytes.NewReader(input), &out)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	msgs := readOutputMessages(t, out.Bytes())
	if len(msgs) != 2 {
		t.Fatalf("expected 2 responses, got %d: %#v", len(msgs), msgs)
	}
	if got := msgs[0]["id"]; got != float64(1) {
		t.Fatalf("initialize id = %#v, want 1", got)
	}
	result := msgs[0]["result"].(map[string]any)
	caps := result["capabilities"].(map[string]any)
	if caps["textDocumentSync"] == nil {
		t.Fatalf("initialize result missing textDocumentSync: %#v", result)
	}
	if caps["documentFormattingProvider"] != true {
		t.Fatalf("initialize result missing formatting capability: %#v", caps)
	}
	if caps["completionProvider"] == nil {
		t.Fatalf("initialize result missing completion capability: %#v", caps)
	}
	if caps["hoverProvider"] != true {
		t.Fatalf("initialize result missing hover capability: %#v", caps)
	}
	if caps["documentSymbolProvider"] != true {
		t.Fatalf("initialize result missing document symbol capability: %#v", caps)
	}
	if caps["documentLinkProvider"] == nil {
		t.Fatalf("initialize result missing document link capability: %#v", caps)
	}
	semanticProvider, ok := caps["semanticTokensProvider"].(map[string]any)
	if !ok || semanticProvider["full"] != true {
		t.Fatalf("initialize result missing semantic tokens capability: %#v", caps)
	}
	legend := semanticProvider["legend"].(map[string]any)
	tokenTypes := legend["tokenTypes"].([]any)
	if len(tokenTypes) != len(semanticTokenTypes) || tokenTypes[0] != "keyword" || tokenTypes[len(tokenTypes)-1] != "namespace" {
		t.Fatalf("semantic token legend = %#v", legend)
	}
	tokenModifiers := legend["tokenModifiers"].([]any)
	if len(tokenModifiers) != len(semanticTokenModifiers) || tokenModifiers[0] != "declaration" || tokenModifiers[len(tokenModifiers)-1] != "dialect" {
		t.Fatalf("semantic token modifiers = %#v", legend)
	}
	if caps["workspaceSymbolProvider"] != true {
		t.Fatalf("initialize result missing workspace symbol capability: %#v", caps)
	}
	if caps["codeLensProvider"] == nil {
		t.Fatalf("initialize result missing code lens capability: %#v", caps)
	}
	if caps["inlayHintProvider"] != true {
		t.Fatalf("initialize result missing inlay hint capability: %#v", caps)
	}
	renameProvider, ok := caps["renameProvider"].(map[string]any)
	if caps["definitionProvider"] != true || caps["referencesProvider"] != true || !ok || renameProvider["prepareProvider"] != true {
		t.Fatalf("initialize result missing navigation capabilities: %#v", caps)
	}
	if got := msgs[1]["id"]; got != float64(2) {
		t.Fatalf("shutdown id = %#v, want 2", got)
	}
	if _, ok := msgs[1]["error"]; ok {
		t.Fatalf("shutdown returned error: %#v", msgs[1])
	}
}

func TestDidOpenPublishesSyntaxDiagnostics(t *testing.T) {
	input := mustEncodeMessages(t,
		map[string]any{
			"jsonrpc": "2.0",
			"method":  "textDocument/didOpen",
			"params": map[string]any{
				"textDocument": map[string]any{
					"uri":        "file:///tmp/bad.leia",
					"languageId": "leia",
					"version":    1,
					"text":       "x := \n",
				},
			},
		},
	)
	var out bytes.Buffer
	err := NewServer().Run(context.Background(), bytes.NewReader(input), &out)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	msgs := readOutputMessages(t, out.Bytes())
	if len(msgs) != 1 {
		t.Fatalf("expected 1 notification, got %d: %#v", len(msgs), msgs)
	}
	if got := msgs[0]["method"]; got != "textDocument/publishDiagnostics" {
		t.Fatalf("method = %#v, want publishDiagnostics", got)
	}
	params := msgs[0]["params"].(map[string]any)
	diagnostics := params["diagnostics"].([]any)
	if len(diagnostics) != 1 {
		t.Fatalf("expected one diagnostic, got %#v", diagnostics)
	}
	diag := diagnostics[0].(map[string]any)
	if diag["source"] != "leia" || diag["severity"] != float64(1) {
		t.Fatalf("unexpected diagnostic metadata: %#v", diag)
	}
	if diag["message"] == "" {
		t.Fatalf("diagnostic missing message: %#v", diag)
	}
}

func TestDidChangeClearsSyntaxDiagnostics(t *testing.T) {
	input := mustEncodeMessages(t,
		map[string]any{
			"jsonrpc": "2.0",
			"method":  "textDocument/didChange",
			"params": map[string]any{
				"textDocument": map[string]any{"uri": "file:///tmp/good.leia", "version": 2},
				"contentChanges": []map[string]any{
					{"text": "x := 1\n"},
				},
			},
		},
	)
	var out bytes.Buffer
	err := NewServer().Run(context.Background(), bytes.NewReader(input), &out)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	msgs := readOutputMessages(t, out.Bytes())
	params := msgs[0]["params"].(map[string]any)
	diagnostics := params["diagnostics"].([]any)
	if len(diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got %#v", diagnostics)
	}
}

func TestFormattingReturnsWholeDocumentEdit(t *testing.T) {
	input := mustEncodeMessages(t,
		map[string]any{
			"jsonrpc": "2.0",
			"method":  "textDocument/didOpen",
			"params": map[string]any{
				"textDocument": map[string]any{
					"uri":        "file:///tmp/format.leia",
					"languageId": "leia",
					"version":    1,
					"text":       "if true {\nreturn 1  \n}\n\n",
				},
			},
		},
		map[string]any{
			"jsonrpc": "2.0",
			"id":      7,
			"method":  "textDocument/formatting",
			"params": map[string]any{
				"textDocument": map[string]any{"uri": "file:///tmp/format.leia"},
				"options":      map[string]any{"tabSize": 4, "insertSpaces": true},
			},
		},
	)
	var out bytes.Buffer
	err := NewServer().Run(context.Background(), bytes.NewReader(input), &out)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	msgs := readOutputMessages(t, out.Bytes())
	if len(msgs) != 2 {
		t.Fatalf("expected diagnostics notification and formatting response, got %d: %#v", len(msgs), msgs)
	}
	resp := msgs[1]
	if got := resp["id"]; got != float64(7) {
		t.Fatalf("formatting response id = %#v, want 7", got)
	}
	edits := resp["result"].([]any)
	if len(edits) != 1 {
		t.Fatalf("expected one formatting edit, got %#v", edits)
	}
	edit := edits[0].(map[string]any)
	if got, want := edit["newText"], "if true {\n    return 1\n}\n"; got != want {
		t.Fatalf("formatted text = %q, want %q", got, want)
	}
}

func TestCompletionReturnsCurrentLeiaSyntaxAndStdlib(t *testing.T) {
	input := mustEncodeMessages(t,
		map[string]any{
			"jsonrpc": "2.0",
			"id":      3,
			"method":  "textDocument/completion",
			"params": map[string]any{
				"textDocument": map[string]any{"uri": "file:///tmp/completion.leia"},
				"position":     map[string]any{"line": 0, "character": 0},
			},
		},
	)
	var out bytes.Buffer
	err := NewServer().Run(context.Background(), bytes.NewReader(input), &out)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	msgs := readOutputMessages(t, out.Bytes())
	if len(msgs) != 1 {
		t.Fatalf("expected one completion response, got %d: %#v", len(msgs), msgs)
	}
	items := msgs[0]["result"].([]any)
	if len(items) == 0 {
		t.Fatalf("expected completion items")
	}
	labels := map[string]bool{}
	for _, item := range items {
		labels[item.(map[string]any)["label"].(string)] = true
	}
	for _, want := range []string{"func", "return", "import", "select", "llm", "llm.turn", "llm.tool", "llm.register_models"} {
		if !labels[want] {
			t.Fatalf("completion labels missing %q: %#v", want, labels)
		}
	}
	for _, old := range []string{"agent", "tool", "evaluate"} {
		if labels[old] {
			t.Fatalf("completion labels unexpectedly include old AI keyword %q: %#v", old, labels)
		}
	}
}

func TestDocumentSymbolReturnsDeclarations(t *testing.T) {
	input := mustEncodeMessages(t,
		map[string]any{
			"jsonrpc": "2.0",
			"method":  "textDocument/didOpen",
			"params": map[string]any{
				"textDocument": map[string]any{
					"uri":        "file:///tmp/symbols.leia",
					"languageId": "leia",
					"version":    1,
					"text": strings.Join([]string{
						"func add(a, b) {",
						"    return a + b",
						"}",
						"func calculate(input) {",
						"    return input",
						"}",
						"func normalize(name) {",
						"    return name",
						"}",
						"",
					}, "\n"),
				},
			},
		},
		map[string]any{
			"jsonrpc": "2.0",
			"id":      9,
			"method":  "textDocument/documentSymbol",
			"params": map[string]any{
				"textDocument": map[string]any{"uri": "file:///tmp/symbols.leia"},
			},
		},
	)
	var out bytes.Buffer
	err := NewServer().Run(context.Background(), bytes.NewReader(input), &out)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	msgs := readOutputMessages(t, out.Bytes())
	if len(msgs) != 2 {
		t.Fatalf("expected diagnostics notification and documentSymbol response, got %d: %#v", len(msgs), msgs)
	}
	symbols := msgs[1]["result"].([]any)
	if len(symbols) != 3 {
		t.Fatalf("expected 3 symbols, got %#v", symbols)
	}
	got := map[string]map[string]any{}
	for _, raw := range symbols {
		sym := raw.(map[string]any)
		got[sym["name"].(string)] = sym
	}
	for _, name := range []string{"add", "calculate", "normalize"} {
		if got[name] == nil {
			t.Fatalf("missing symbol %q: %#v", name, got)
		}
	}
	if got["add"]["kind"] != float64(12) {
		t.Fatalf("add kind = %#v, want function", got["add"]["kind"])
	}
	selection := got["calculate"]["selectionRange"].(map[string]any)
	start := selection["start"].(map[string]any)
	if start["line"] != float64(3) || start["character"] != float64(5) {
		t.Fatalf("calculate selection start = %#v, want 3:5", start)
	}
}

func TestDocumentLinkReturnsLocalModuleLinks(t *testing.T) {
	src := strings.Join([]string{
		`helper := require("./helper")`,
		`json := require("json")`,
		`import "./tools/other" as other`,
		`import local "./pkg/local"`,
		`import (`,
		`    "./grouped"`,
		`    named "./named"`,
		`)`,
		`import "go:strings" as strings`,
		"",
	}, "\n")
	input := mustEncodeMessages(t,
		map[string]any{
			"jsonrpc": "2.0",
			"method":  "textDocument/didOpen",
			"params": map[string]any{
				"textDocument": map[string]any{
					"uri":        "file:///tmp/project/main.leia",
					"languageId": "leia",
					"version":    1,
					"text":       src,
				},
			},
		},
		map[string]any{
			"jsonrpc": "2.0",
			"id":      31,
			"method":  "textDocument/documentLink",
			"params": map[string]any{
				"textDocument": map[string]any{"uri": "file:///tmp/project/main.leia"},
			},
		},
	)
	var out bytes.Buffer
	err := NewServer().Run(context.Background(), bytes.NewReader(input), &out)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	msgs := readOutputMessages(t, out.Bytes())
	if len(msgs) != 2 {
		t.Fatalf("expected diagnostics notification and documentLink response, got %d: %#v", len(msgs), msgs)
	}
	links := msgs[1]["result"].([]any)
	if len(links) != 5 {
		t.Fatalf("document links = %#v, want helper, other, local, grouped, and named only", links)
	}
	targets := map[string]bool{}
	for _, raw := range links {
		link := raw.(map[string]any)
		targets[link["target"].(string)] = true
		if link["tooltip"] == "" {
			t.Fatalf("document link missing tooltip: %#v", link)
		}
	}
	for _, want := range []string{
		"file:///tmp/project/helper.leia",
		"file:///tmp/project/tools/other.leia",
		"file:///tmp/project/pkg/local.leia",
		"file:///tmp/project/grouped.leia",
		"file:///tmp/project/named.leia",
	} {
		if !targets[want] {
			t.Fatalf("document link targets missing %q: %#v", want, targets)
		}
	}
}

func TestSemanticTokensFullReturnsEncodedTokens(t *testing.T) {
	src := strings.Join([]string{
		"func add(a, b) {",
		"    return a + b",
		"}",
		`answer := add(40, 2)`,
		"",
	}, "\n")
	input := mustEncodeMessages(t,
		map[string]any{
			"jsonrpc": "2.0",
			"method":  "textDocument/didOpen",
			"params": map[string]any{
				"textDocument": map[string]any{
					"uri":        "file:///tmp/semantic.leia",
					"languageId": "leia",
					"version":    1,
					"text":       src,
				},
			},
		},
		map[string]any{
			"jsonrpc": "2.0",
			"id":      32,
			"method":  "textDocument/semanticTokens/full",
			"params": map[string]any{
				"textDocument": map[string]any{"uri": "file:///tmp/semantic.leia"},
			},
		},
	)
	var out bytes.Buffer
	err := NewServer().Run(context.Background(), bytes.NewReader(input), &out)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	msgs := readOutputMessages(t, out.Bytes())
	if len(msgs) != 2 {
		t.Fatalf("expected diagnostics notification and semantic tokens response, got %d: %#v", len(msgs), msgs)
	}
	result := msgs[1]["result"].(map[string]any)
	data := result["data"].([]any)
	if len(data) == 0 || len(data)%5 != 0 {
		t.Fatalf("semantic token data = %#v, want non-empty 5-tuples", data)
	}
	foundDeclaration := false
	for i := 0; i+4 < len(data); i += 5 {
		if data[i+3] == float64(semanticFunction) && data[i+4] == float64(semanticDeclarationModifier) {
			foundDeclaration = true
			break
		}
	}
	if !foundDeclaration {
		t.Fatalf("semantic token data = %#v, want function declaration token", data)
	}
}

func TestSemanticTokensStringEscapesStayWithinSourceLine(t *testing.T) {
	src := "value := \"a\\nb\"\nraw := `multi\nline`\n"
	input := mustEncodeMessages(t,
		map[string]any{
			"jsonrpc": "2.0",
			"method":  "textDocument/didOpen",
			"params": map[string]any{
				"textDocument": map[string]any{
					"uri":        "file:///tmp/semantic-strings.leia",
					"languageId": "leia",
					"version":    1,
					"text":       src,
				},
			},
		},
		map[string]any{
			"jsonrpc": "2.0",
			"id":      33,
			"method":  "textDocument/semanticTokens/full",
			"params": map[string]any{
				"textDocument": map[string]any{"uri": "file:///tmp/semantic-strings.leia"},
			},
		},
	)
	var out bytes.Buffer
	err := NewServer().Run(context.Background(), bytes.NewReader(input), &out)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	msgs := readOutputMessages(t, out.Bytes())
	if len(msgs) != 2 {
		t.Fatalf("expected diagnostics notification and semantic tokens response, got %d: %#v", len(msgs), msgs)
	}
	data := msgs[1]["result"].(map[string]any)["data"].([]any)
	lines := strings.Split(src, "\n")
	line, char := 0, 0
	for i := 0; i+4 < len(data); i += 5 {
		deltaLine := int(data[i].(float64))
		deltaStart := int(data[i+1].(float64))
		length := int(data[i+2].(float64))
		line += deltaLine
		if deltaLine == 0 {
			char += deltaStart
		} else {
			char = deltaStart
		}
		if line < 0 || line >= len(lines) || char+length > len(lines[line]) {
			t.Fatalf("semantic token tuple %v extends past source line %d %q", data[i:i+5], line, lines[line])
		}
	}
}

func TestSemanticTokensLexerErrorReturnsEmptyData(t *testing.T) {
	input := mustEncodeMessages(t,
		map[string]any{
			"jsonrpc": "2.0",
			"method":  "textDocument/didOpen",
			"params": map[string]any{
				"textDocument": map[string]any{
					"uri":        "file:///tmp/semantic-bad.leia",
					"languageId": "leia",
					"version":    1,
					"text":       "value := \"unterminated\n",
				},
			},
		},
		map[string]any{
			"jsonrpc": "2.0",
			"id":      34,
			"method":  "textDocument/semanticTokens/full",
			"params": map[string]any{
				"textDocument": map[string]any{"uri": "file:///tmp/semantic-bad.leia"},
			},
		},
	)
	var out bytes.Buffer
	err := NewServer().Run(context.Background(), bytes.NewReader(input), &out)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	msgs := readOutputMessages(t, out.Bytes())
	if len(msgs) != 2 {
		t.Fatalf("expected diagnostics notification and semantic tokens response, got %d: %#v", len(msgs), msgs)
	}
	data := msgs[1]["result"].(map[string]any)["data"].([]any)
	if len(data) != 0 {
		t.Fatalf("semantic token data = %#v, want empty data for lexer error", data)
	}
}

func TestSemanticTokensClassifyLeiaSemanticRoles(t *testing.T) {
	src := strings.Join([]string{
		`import "go:strings" as strings`,
		`import local "./pkg/local"`,
		`import (`,
		`    "./grouped"`,
		`    named "./named"`,
		`)`,
		`rows := csv!` + "`a,b\\n1,2\\n`",
		`out := $!` + "`printf ok`",
		`prompt! { role: "system" }`,
		`select {`,
		`case value := <-ch:`,
		`    return value`,
		`default:`,
		`    return nil`,
		`}`,
		`func add(a, b) {`,
		`    return math.floor(a + b)`,
		`}`,
		`object:run()`,
		"",
	}, "\n")
	tokens := decodedSemanticTokens(src)
	assertSemanticToken(t, tokens, "import", semanticKeyword, semanticImportModifier)
	assertSemanticToken(t, tokens, "go:strings", semanticString, semanticImportModifier)
	assertSemanticToken(t, tokens, "as", semanticKeyword, semanticImportModifier)
	assertSemanticToken(t, tokens, "strings", semanticNamespace, semanticDeclarationModifier|semanticImportModifier)
	assertSemanticToken(t, tokens, "local", semanticNamespace, semanticDeclarationModifier|semanticImportModifier)
	assertSemanticToken(t, tokens, "./pkg/local", semanticString, semanticImportModifier)
	assertSemanticToken(t, tokens, "./grouped", semanticString, semanticImportModifier)
	assertSemanticToken(t, tokens, "named", semanticNamespace, semanticDeclarationModifier|semanticImportModifier)
	assertSemanticToken(t, tokens, "./named", semanticString, semanticImportModifier)
	assertSemanticToken(t, tokens, "csv", semanticNamespace, semanticDialectModifier)
	assertSemanticToken(t, tokens, "!", semanticOperator, semanticDialectModifier)
	assertSemanticToken(t, tokens, `a,b\n1,2\n`, semanticString, semanticDialectModifier)
	assertSemanticToken(t, tokens, "$", semanticNamespace, semanticDialectModifier)
	assertSemanticToken(t, tokens, "printf ok", semanticString, semanticDialectModifier)
	assertSemanticTokenSequence(t, tokens,
		semanticTokenForTest{Text: "$", TokenType: semanticNamespace, Modifier: semanticDialectModifier},
		semanticTokenForTest{Text: "!", TokenType: semanticOperator, Modifier: semanticDialectModifier},
		semanticTokenForTest{Text: "printf ok", TokenType: semanticString, Modifier: semanticDialectModifier},
	)
	assertSemanticToken(t, tokens, "prompt", semanticNamespace, semanticDialectModifier)
	assertSemanticToken(t, tokens, "select", semanticKeyword, 0)
	assertSemanticToken(t, tokens, "case", semanticKeyword, 0)
	assertSemanticToken(t, tokens, "default", semanticKeyword, 0)
	assertSemanticToken(t, tokens, "add", semanticFunction, semanticDeclarationModifier)
	assertSemanticToken(t, tokens, "a", semanticParameter, semanticDeclarationModifier)
	assertSemanticToken(t, tokens, "math", semanticNamespace, semanticDefaultLibraryModifier)
	assertSemanticToken(t, tokens, "floor", semanticProperty, 0)
	assertSemanticToken(t, tokens, "run", semanticMethod, 0)
}

func TestSemanticTokensTreatLegacyAIWordsAsVariables(t *testing.T) {
	src := strings.Join([]string{
		`agent := tool + evaluate`,
		`models := messages + budget`,
		"",
	}, "\n")
	tokens := decodedSemanticTokens(src)
	for _, name := range []string{"agent", "tool", "evaluate", "models", "messages", "budget"} {
		assertSemanticToken(t, tokens, name, semanticVariable, 0)
	}
}

func TestWorkspaceSymbolReturnsOpenDocumentDeclarations(t *testing.T) {
	input := mustEncodeMessages(t,
		map[string]any{
			"jsonrpc": "2.0",
			"method":  "textDocument/didOpen",
			"params": map[string]any{
				"textDocument": map[string]any{
					"uri":        "file:///tmp/alpha.leia",
					"languageId": "leia",
					"version":    1,
					"text":       "func alpha_tool() { return 1 }\n",
				},
			},
		},
		map[string]any{
			"jsonrpc": "2.0",
			"method":  "textDocument/didOpen",
			"params": map[string]any{
				"textDocument": map[string]any{
					"uri":        "file:///tmp/beta.leia",
					"languageId": "leia",
					"version":    1,
					"text": strings.Join([]string{
						"func beta_agent() {",
						"    return true",
						"}",
						"func beta_baseline() {",
						"    assert(true)",
						"}",
						"",
					}, "\n"),
				},
			},
		},
		map[string]any{
			"jsonrpc": "2.0",
			"id":      40,
			"method":  "workspace/symbol",
			"params":  map[string]any{"query": "beta"},
		},
	)
	var out bytes.Buffer
	err := NewServer().Run(context.Background(), bytes.NewReader(input), &out)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	msgs := readOutputMessages(t, out.Bytes())
	if len(msgs) != 3 {
		t.Fatalf("expected two diagnostics notifications and workspace/symbol response, got %d: %#v", len(msgs), msgs)
	}
	symbols := msgs[2]["result"].([]any)
	if len(symbols) != 2 {
		t.Fatalf("workspace symbols = %#v, want beta_agent and beta_baseline", symbols)
	}
	got := map[string]map[string]any{}
	for _, raw := range symbols {
		sym := raw.(map[string]any)
		got[sym["name"].(string)] = sym
	}
	for _, name := range []string{"beta_agent", "beta_baseline"} {
		if got[name] == nil {
			t.Fatalf("missing workspace symbol %q: %#v", name, got)
		}
		loc := got[name]["location"].(map[string]any)
		if loc["uri"] != "file:///tmp/beta.leia" {
			t.Fatalf("symbol %s location = %#v", name, loc)
		}
	}
	if got["alpha_tool"] != nil {
		t.Fatalf("query unexpectedly returned alpha_tool: %#v", got)
	}
}

func TestHoverReturnsKeywordStdlibAndDeclaration(t *testing.T) {
	src := strings.Join([]string{
		"func add(a, b) {",
		"    return a + b",
		"}",
		"math.floor(1.2)",
		"",
	}, "\n")
	input := mustEncodeMessages(t,
		map[string]any{
			"jsonrpc": "2.0",
			"method":  "textDocument/didOpen",
			"params": map[string]any{
				"textDocument": map[string]any{
					"uri":        "file:///tmp/hover.leia",
					"languageId": "leia",
					"version":    1,
					"text":       src,
				},
			},
		},
		map[string]any{
			"jsonrpc": "2.0",
			"id":      10,
			"method":  "textDocument/hover",
			"params": map[string]any{
				"textDocument": map[string]any{"uri": "file:///tmp/hover.leia"},
				"position":     map[string]any{"line": 1, "character": 5},
			},
		},
		map[string]any{
			"jsonrpc": "2.0",
			"id":      11,
			"method":  "textDocument/hover",
			"params": map[string]any{
				"textDocument": map[string]any{"uri": "file:///tmp/hover.leia"},
				"position":     map[string]any{"line": 3, "character": 1},
			},
		},
		map[string]any{
			"jsonrpc": "2.0",
			"id":      12,
			"method":  "textDocument/hover",
			"params": map[string]any{
				"textDocument": map[string]any{"uri": "file:///tmp/hover.leia"},
				"position":     map[string]any{"line": 0, "character": 6},
			},
		},
	)
	var out bytes.Buffer
	err := NewServer().Run(context.Background(), bytes.NewReader(input), &out)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	msgs := readOutputMessages(t, out.Bytes())
	if len(msgs) != 4 {
		t.Fatalf("expected diagnostics notification and 3 hover responses, got %d: %#v", len(msgs), msgs)
	}
	assertHoverContains(t, msgs[1], "**return**")
	assertHoverContains(t, msgs[2], "stdlib module `math`")
	assertHoverContains(t, msgs[3], "func `add(a, b)`")
}

func TestHoverDoesNotTreatLegacyAIWordsAsKeywords(t *testing.T) {
	if got := hoverText("agent := 1\n", "agent"); got != "" {
		t.Fatalf("legacy AI hover = %q, want empty", got)
	}
}

func TestCodeLensAndInlayHintDoNotReturnLegacyAIActions(t *testing.T) {
	src := strings.Join([]string{
		"agent := 1",
		"tool := 2",
		"evaluate := agent + tool",
		"func current() {",
		"    return llm.turn({})",
		"}",
		"",
	}, "\n")
	if lenses := collectCodeLens("file:///tmp/lens.leia", src); len(lenses) != 0 {
		t.Fatalf("legacy AI code lenses = %#v, want none", lenses)
	}
	if hints := collectInlayHints(src, lspRange{}); len(hints) != 0 {
		t.Fatalf("legacy AI inlay hints = %#v, want none", hints)
	}
}

func TestCodeLensReturnsEvaluateCaseActions(t *testing.T) {
	src := strings.Join([]string{
		`evaluate "basic assertion" {`,
		`    assert(true)`,
		`}`,
		`evaluate "second case" {`,
		`    assert(true)`,
		`}`,
		"",
	}, "\n")
	lenses := collectCodeLens("file:///tmp/eval.leia", src)
	if len(lenses) != 2 {
		t.Fatalf("code lenses = %#v, want two evaluate cases", lenses)
	}
	if lenses[0].Range.Start.Line != 0 || lenses[0].Command.Command != "leia.evaluate.case" || lenses[0].Command.Title != "Run evaluate: basic assertion" {
		t.Fatalf("first lens = %#v", lenses[0])
	}
	if len(lenses[0].Command.Arguments) != 2 || lenses[0].Command.Arguments[0] != "file:///tmp/eval.leia" || lenses[0].Command.Arguments[1] != "basic assertion" {
		t.Fatalf("first lens args = %#v", lenses[0].Command.Arguments)
	}
	if lenses[1].Range.Start.Line != 3 || lenses[1].Command.Arguments[1] != "second case" {
		t.Fatalf("second lens = %#v", lenses[1])
	}
}

func TestInlayHintsReturnLocalCallParametersAndStdlibImports(t *testing.T) {
	src := strings.Join([]string{
		`json := require("json")`,
		`import "math"`,
		`func add(left, right) {`,
		`    return left + right`,
		`}`,
		`result := add(1, 2)`,
		`again := add(left, 3)`,
		"",
	}, "\n")
	hints := collectInlayHints(src, fullDocumentRange(src))
	labelsByLine := map[int][]string{}
	tooltipsByLabel := map[string]string{}
	for _, hint := range hints {
		labelsByLine[hint.Position.Line] = append(labelsByLine[hint.Position.Line], hint.Label)
		tooltipsByLabel[hint.Label] = hint.Tooltip
	}
	for _, want := range []struct {
		line  int
		label string
	}{
		{0, ": stdlib base"},
		{1, ": stdlib base"},
		{5, "left:"},
		{5, "right:"},
		{6, "right:"},
	} {
		if !containsString(labelsByLine[want.line], want.label) {
			t.Fatalf("hints by line = %#v, missing %q on line %d", labelsByLine, want.label, want.line)
		}
	}
	if containsString(labelsByLine[6], "left:") {
		t.Fatalf("line 6 hints = %#v, should skip argument already named left", labelsByLine[6])
	}
	if !strings.Contains(tooltipsByLabel[": stdlib base"], "JSON") && !strings.Contains(tooltipsByLabel[": stdlib base"], "Numeric") {
		t.Fatalf("stdlib tooltip = %q, want catalog description", tooltipsByLabel[": stdlib base"])
	}
}

func TestInlayHintsHonorRequestedRange(t *testing.T) {
	src := strings.Join([]string{
		`func add(left, right) { return left + right }`,
		`first := add(1, 2)`,
		`second := add(3, 4)`,
		"",
	}, "\n")
	hints := collectInlayHints(src, lspRange{
		Start: position{Line: 2, Character: 0},
		End:   position{Line: 2, Character: 80},
	})
	if len(hints) != 2 {
		t.Fatalf("range hints = %#v, want two hints for second call only", hints)
	}
	for _, hint := range hints {
		if hint.Position.Line != 2 {
			t.Fatalf("hint outside requested range: %#v", hint)
		}
	}
}

func TestDefinitionReferencesAndRename(t *testing.T) {
	src := strings.Join([]string{
		"func add(a, b) {",
		"    return a + b",
		"}",
		"result := add(1, 2)",
		"again := add(result, 3)",
		"",
	}, "\n")
	input := mustEncodeMessages(t,
		map[string]any{
			"jsonrpc": "2.0",
			"method":  "textDocument/didOpen",
			"params": map[string]any{
				"textDocument": map[string]any{
					"uri":        "file:///tmp/nav.leia",
					"languageId": "leia",
					"version":    1,
					"text":       src,
				},
			},
		},
		map[string]any{
			"jsonrpc": "2.0",
			"id":      20,
			"method":  "textDocument/definition",
			"params": map[string]any{
				"textDocument": map[string]any{"uri": "file:///tmp/nav.leia"},
				"position":     map[string]any{"line": 3, "character": 11},
			},
		},
		map[string]any{
			"jsonrpc": "2.0",
			"id":      21,
			"method":  "textDocument/references",
			"params": map[string]any{
				"textDocument": map[string]any{"uri": "file:///tmp/nav.leia"},
				"position":     map[string]any{"line": 3, "character": 11},
				"context":      map[string]any{"includeDeclaration": true},
			},
		},
		map[string]any{
			"jsonrpc": "2.0",
			"id":      22,
			"method":  "textDocument/prepareRename",
			"params": map[string]any{
				"textDocument": map[string]any{"uri": "file:///tmp/nav.leia"},
				"position":     map[string]any{"line": 0, "character": 6},
			},
		},
		map[string]any{
			"jsonrpc": "2.0",
			"id":      23,
			"method":  "textDocument/rename",
			"params": map[string]any{
				"textDocument": map[string]any{"uri": "file:///tmp/nav.leia"},
				"position":     map[string]any{"line": 0, "character": 6},
				"newName":      "sum",
			},
		},
	)
	var out bytes.Buffer
	err := NewServer().Run(context.Background(), bytes.NewReader(input), &out)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	msgs := readOutputMessages(t, out.Bytes())
	if len(msgs) != 5 {
		t.Fatalf("expected diagnostics notification and 4 navigation responses, got %d: %#v", len(msgs), msgs)
	}
	def := msgs[1]["result"].(map[string]any)
	defRange := def["range"].(map[string]any)
	defStart := defRange["start"].(map[string]any)
	if def["uri"] != "file:///tmp/nav.leia" || defStart["line"] != float64(0) || defStart["character"] != float64(5) {
		t.Fatalf("definition = %#v, want add declaration at 0:5", def)
	}

	refs := msgs[2]["result"].([]any)
	if len(refs) != 3 {
		t.Fatalf("references = %#v, want declaration plus two calls", refs)
	}

	prep := msgs[3]["result"].(map[string]any)
	if prep["placeholder"] != "add" {
		t.Fatalf("prepareRename = %#v, want placeholder add", prep)
	}
	prepRange := prep["range"].(map[string]any)
	prepStart := prepRange["start"].(map[string]any)
	if prepStart["line"] != float64(0) || prepStart["character"] != float64(5) {
		t.Fatalf("prepareRename range = %#v, want add declaration at 0:5", prepRange)
	}

	edit := msgs[4]["result"].(map[string]any)
	changes := edit["changes"].(map[string]any)
	edits := changes["file:///tmp/nav.leia"].([]any)
	if len(edits) != 3 {
		t.Fatalf("rename edits = %#v, want three add replacements", edits)
	}
	for _, raw := range edits {
		e := raw.(map[string]any)
		if e["newText"] != "sum" {
			t.Fatalf("rename edit = %#v, want newText sum", e)
		}
	}
}

func TestRenameRejectsInvalidIdentifier(t *testing.T) {
	input := mustEncodeMessages(t,
		map[string]any{
			"jsonrpc": "2.0",
			"method":  "textDocument/didOpen",
			"params": map[string]any{
				"textDocument": map[string]any{
					"uri":        "file:///tmp/rename-bad.leia",
					"languageId": "leia",
					"version":    1,
					"text":       "func add() { return 1 }\n",
				},
			},
		},
		map[string]any{
			"jsonrpc": "2.0",
			"id":      23,
			"method":  "textDocument/rename",
			"params": map[string]any{
				"textDocument": map[string]any{"uri": "file:///tmp/rename-bad.leia"},
				"position":     map[string]any{"line": 0, "character": 6},
				"newName":      "1bad",
			},
		},
	)
	var out bytes.Buffer
	err := NewServer().Run(context.Background(), bytes.NewReader(input), &out)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	msgs := readOutputMessages(t, out.Bytes())
	if len(msgs) != 2 {
		t.Fatalf("expected diagnostics notification and rename response, got %d: %#v", len(msgs), msgs)
	}
	if msgs[1]["error"] == nil {
		t.Fatalf("rename response = %#v, want error for invalid identifier", msgs[1])
	}
}

func TestPrepareRenameReturnsNilOutsideIdentifier(t *testing.T) {
	input := mustEncodeMessages(t,
		map[string]any{
			"jsonrpc": "2.0",
			"method":  "textDocument/didOpen",
			"params": map[string]any{
				"textDocument": map[string]any{
					"uri":        "file:///tmp/prepare-empty.leia",
					"languageId": "leia",
					"version":    1,
					"text":       "func add() { return 1 }\n",
				},
			},
		},
		map[string]any{
			"jsonrpc": "2.0",
			"id":      24,
			"method":  "textDocument/prepareRename",
			"params": map[string]any{
				"textDocument": map[string]any{"uri": "file:///tmp/prepare-empty.leia"},
				"position":     map[string]any{"line": 0, "character": 4},
			},
		},
	)
	var out bytes.Buffer
	err := NewServer().Run(context.Background(), bytes.NewReader(input), &out)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	msgs := readOutputMessages(t, out.Bytes())
	if len(msgs) != 2 {
		t.Fatalf("expected diagnostics notification and prepareRename response, got %d: %#v", len(msgs), msgs)
	}
	if got, ok := msgs[1]["result"]; !ok || got != nil {
		t.Fatalf("prepareRename response = %#v, want nil result", msgs[1])
	}
}

func assertHoverContains(t *testing.T, msg map[string]any, want string) {
	t.Helper()
	result := msg["result"].(map[string]any)
	contents := result["contents"].(map[string]any)
	value := contents["value"].(string)
	if !strings.Contains(value, want) {
		t.Fatalf("hover = %q, want substring %q", value, want)
	}
}

type semanticTokenForTest struct {
	Text      string
	TokenType int
	Modifier  int
}

func decodedSemanticTokens(src string) []semanticTokenForTest {
	data := collectSemanticTokens(src)
	lines := strings.Split(src, "\n")
	var out []semanticTokenForTest
	line, char := 0, 0
	for i := 0; i+4 < len(data); i += 5 {
		deltaLine := data[i]
		deltaStart := data[i+1]
		length := data[i+2]
		line += deltaLine
		if deltaLine == 0 {
			char += deltaStart
		} else {
			char = deltaStart
		}
		text := ""
		if line >= 0 && line < len(lines) && char >= 0 && char+length <= len(lines[line]) {
			text = lines[line][char : char+length]
		}
		out = append(out, semanticTokenForTest{
			Text:      text,
			TokenType: data[i+3],
			Modifier:  data[i+4],
		})
	}
	return out
}

func assertSemanticToken(t *testing.T, tokens []semanticTokenForTest, text string, tokenType int, modifier int) {
	t.Helper()
	for _, tok := range tokens {
		if tok.Text == text && tok.TokenType == tokenType && tok.Modifier == modifier {
			return
		}
	}
	t.Fatalf("missing semantic token %q type=%d modifier=%d in %#v", text, tokenType, modifier, tokens)
}

func assertSemanticTokenSequence(t *testing.T, tokens []semanticTokenForTest, want ...semanticTokenForTest) {
	t.Helper()
	for i := 0; i+len(want) <= len(tokens); i++ {
		matched := true
		for j := range want {
			if tokens[i+j] != want[j] {
				matched = false
				break
			}
		}
		if matched {
			return
		}
	}
	t.Fatalf("missing semantic token sequence %#v in %#v", want, tokens)
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func mustEncodeMessages(t *testing.T, msgs ...any) []byte {
	t.Helper()
	var buf bytes.Buffer
	for _, msg := range msgs {
		encoded, err := encodeMessage(msg)
		if err != nil {
			t.Fatalf("encodeMessage: %v", err)
		}
		buf.Write(encoded)
	}
	return buf.Bytes()
}

func readOutputMessages(t *testing.T, data []byte) []map[string]any {
	t.Helper()
	r := bufio.NewReader(bytes.NewReader(data))
	var out []map[string]any
	for {
		payload, err := readMessage(r)
		if err != nil {
			if err == io.EOF {
				break
			}
			t.Fatalf("readMessage: %v", err)
		}
		var msg map[string]any
		if err := json.Unmarshal(payload, &msg); err != nil {
			t.Fatalf("json.Unmarshal(%q): %v", string(payload), err)
		}
		out = append(out, msg)
	}
	return out
}
