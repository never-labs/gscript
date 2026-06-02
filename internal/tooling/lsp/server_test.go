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

func TestDidOpenPublishesAINativeDiagnostics(t *testing.T) {
	input := mustEncodeMessages(t,
		map[string]any{
			"jsonrpc": "2.0",
			"method":  "textDocument/didOpen",
			"params": map[string]any{
				"textDocument": map[string]any{
					"uri":        "file:///tmp/ai-diagnostic.leia",
					"languageId": "leia",
					"version":    1,
					"text":       "tool missing_caps() { return nil, nil }\n",
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
		t.Fatalf("expected one diagnostic notification, got %d: %#v", len(msgs), msgs)
	}
	params := msgs[0]["params"].(map[string]any)
	diagnostics := params["diagnostics"].([]any)
	if len(diagnostics) != 1 {
		t.Fatalf("expected one diagnostic, got %#v", diagnostics)
	}
	diag := diagnostics[0].(map[string]any)
	if diag["code"] != "LEIA2001" || !strings.Contains(diag["message"].(string), "tool missing_caps") {
		t.Fatalf("unexpected AI-native diagnostic: %#v", diag)
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

func TestCompletionReturnsLeiaKeywords(t *testing.T) {
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
	for _, want := range []string{"func", "return", "agent", "tool", "evaluate"} {
		if !labels[want] {
			t.Fatalf("completion labels missing %q: %#v", want, labels)
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
						"// Calculator tool.",
						"// leia:requires: cap.none",
						"tool calculate(input) {",
						"    return input",
						"}",
						"agent helper {",
						"    model: \"test\"",
						"}",
						"evaluate \"slug baseline\" {",
						"    func normalize(name) {",
						"        return name",
						"    }",
						"    assert(normalize(\"Leia\") == \"Leia\")",
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
	if len(symbols) != 5 {
		t.Fatalf("expected 5 symbols, got %#v", symbols)
	}
	got := map[string]map[string]any{}
	for _, raw := range symbols {
		sym := raw.(map[string]any)
		got[sym["name"].(string)] = sym
	}
	for _, name := range []string{"add", "calculate", "helper", "slug baseline", "normalize"} {
		if got[name] == nil {
			t.Fatalf("missing symbol %q: %#v", name, got)
		}
	}
	if got["add"]["kind"] != float64(12) {
		t.Fatalf("add kind = %#v, want function", got["add"]["kind"])
	}
	if got["helper"]["kind"] != float64(5) {
		t.Fatalf("helper kind = %#v, want class", got["helper"]["kind"])
	}
	if got["slug baseline"]["kind"] != float64(24) {
		t.Fatalf("slug baseline kind = %#v, want event", got["slug baseline"]["kind"])
	}
	selection := got["calculate"]["selectionRange"].(map[string]any)
	start := selection["start"].(map[string]any)
	if start["line"] != float64(5) || start["character"] != float64(5) {
		t.Fatalf("calculate selection start = %#v, want 5:5", start)
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
						"agent beta_agent {",
						"    model: \"mock\"",
						"}",
						"evaluate \"beta baseline\" {",
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
		t.Fatalf("workspace symbols = %#v, want beta_agent and beta baseline", symbols)
	}
	got := map[string]map[string]any{}
	for _, raw := range symbols {
		sym := raw.(map[string]any)
		got[sym["name"].(string)] = sym
	}
	for _, name := range []string{"beta_agent", "beta baseline"} {
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

func TestCodeLensReturnsAgentAndEvaluateActions(t *testing.T) {
	src := strings.Join([]string{
		"agent helper {",
		"    model: \"mock\"",
		"}",
		"evaluate \"helper baseline\" {",
		"    assert(true)",
		"}",
		"",
	}, "\n")
	input := mustEncodeMessages(t,
		map[string]any{
			"jsonrpc": "2.0",
			"method":  "textDocument/didOpen",
			"params": map[string]any{
				"textDocument": map[string]any{
					"uri":        "file:///tmp/lens.leia",
					"languageId": "leia",
					"version":    1,
					"text":       src,
				},
			},
		},
		map[string]any{
			"jsonrpc": "2.0",
			"id":      30,
			"method":  "textDocument/codeLens",
			"params": map[string]any{
				"textDocument": map[string]any{"uri": "file:///tmp/lens.leia"},
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
		t.Fatalf("expected diagnostics notification and codeLens response, got %d: %#v", len(msgs), msgs)
	}
	lenses := msgs[1]["result"].([]any)
	if len(lenses) != 2 {
		t.Fatalf("expected two code lenses, got %#v", lenses)
	}
	commands := map[string]string{}
	for _, raw := range lenses {
		lens := raw.(map[string]any)
		cmd := lens["command"].(map[string]any)
		commands[cmd["command"].(string)] = cmd["title"].(string)
	}
	if commands["leia.agent.run"] != "Run agent" {
		t.Fatalf("missing agent code lens: %#v", commands)
	}
	if commands["leia.evaluate.case"] != "Run evaluate case" {
		t.Fatalf("missing evaluate code lens: %#v", commands)
	}
}

func TestInlayHintReturnsAgentAndEvaluateHints(t *testing.T) {
	src := strings.Join([]string{
		"agent helper {",
		"    model: \"mock\"",
		"}",
		"evaluate \"helper baseline\" {",
		"    assert(true)",
		"}",
		"",
	}, "\n")
	input := mustEncodeMessages(t,
		map[string]any{
			"jsonrpc": "2.0",
			"method":  "textDocument/didOpen",
			"params": map[string]any{
				"textDocument": map[string]any{
					"uri":        "file:///tmp/hints.leia",
					"languageId": "leia",
					"version":    1,
					"text":       src,
				},
			},
		},
		map[string]any{
			"jsonrpc": "2.0",
			"id":      31,
			"method":  "textDocument/inlayHint",
			"params": map[string]any{
				"textDocument": map[string]any{"uri": "file:///tmp/hints.leia"},
				"range": map[string]any{
					"start": map[string]any{"line": 0, "character": 0},
					"end":   map[string]any{"line": 6, "character": 0},
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
	if len(msgs) != 2 {
		t.Fatalf("expected diagnostics notification and inlayHint response, got %d: %#v", len(msgs), msgs)
	}
	hints := msgs[1]["result"].([]any)
	if len(hints) != 2 {
		t.Fatalf("expected two inlay hints, got %#v", hints)
	}
	labels := map[string]bool{}
	for _, raw := range hints {
		hint := raw.(map[string]any)
		labels[hint["label"].(string)] = true
	}
	if !labels[" agent"] || !labels[" eval"] {
		t.Fatalf("missing expected hint labels: %#v", labels)
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
