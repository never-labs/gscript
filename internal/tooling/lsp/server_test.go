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
