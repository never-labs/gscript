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
