package lsp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
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
	for _, want := range []string{"func", "return", "agent", "tool"} {
		if !labels[want] {
			t.Fatalf("completion labels missing %q: %#v", want, labels)
		}
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
