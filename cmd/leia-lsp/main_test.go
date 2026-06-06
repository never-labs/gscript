package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestHelpDoesNotStartServer(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"--help"}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run help code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "usage: leia-lsp") {
		t.Fatalf("stdout = %q, want usage", stdout.String())
	}
}

func TestUnknownArgumentReportsUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"--bad"}, strings.NewReader(""), &stdout, &stderr)
	if code != 2 {
		t.Fatalf("run unknown code = %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "usage: leia-lsp") {
		t.Fatalf("stderr = %q, want usage", stderr.String())
	}
}

func TestBinaryStdioSmoke(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping binary stdio smoke in short mode")
	}

	bin := filepath.Join(t.TempDir(), "leia-lsp")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	build := exec.Command("go", "build", "-o", bin, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build leia-lsp failed: %v\n%s", err, out)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start leia-lsp: %v", err)
	}
	t.Cleanup(func() {
		if cmd.ProcessState == nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	})

	reader := bufio.NewReader(stdout)
	writeLSPMessage(t, stdin, map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params":  map[string]any{},
	})
	initialize := readLSPMessage(t, reader)
	if got := initialize["id"]; got != float64(1) {
		t.Fatalf("initialize id = %#v, want 1; stderr=%q", got, stderr.String())
	}
	result, ok := initialize["result"].(map[string]any)
	if !ok {
		t.Fatalf("initialize missing result: %#v; stderr=%q", initialize, stderr.String())
	}
	caps, ok := result["capabilities"].(map[string]any)
	if !ok || caps["textDocumentSync"] == nil {
		t.Fatalf("initialize missing capabilities: %#v; stderr=%q", result, stderr.String())
	}

	writeLSPMessage(t, stdin, map[string]any{"jsonrpc": "2.0", "method": "initialized"})
	writeLSPMessage(t, stdin, map[string]any{
		"jsonrpc": "2.0",
		"method":  "textDocument/didOpen",
		"params": map[string]any{
			"textDocument": map[string]any{
				"uri":        "file:///tmp/leia-lsp-smoke-bad.leia",
				"languageId": "leia",
				"version":    1,
				"text":       "x := \n",
			},
		},
	})
	diagnostics := readLSPMessage(t, reader)
	if got := diagnostics["method"]; got != "textDocument/publishDiagnostics" {
		t.Fatalf("method = %#v, want publishDiagnostics; msg=%#v; stderr=%q", got, diagnostics, stderr.String())
	}
	params, ok := diagnostics["params"].(map[string]any)
	if !ok {
		t.Fatalf("diagnostics missing params: %#v; stderr=%q", diagnostics, stderr.String())
	}
	items, ok := params["diagnostics"].([]any)
	if !ok || len(items) == 0 {
		t.Fatalf("expected diagnostics, got %#v; stderr=%q", params["diagnostics"], stderr.String())
	}

	writeLSPMessage(t, stdin, map[string]any{"jsonrpc": "2.0", "id": 2, "method": "shutdown"})
	shutdown := readLSPMessage(t, reader)
	if got := shutdown["id"]; got != float64(2) {
		t.Fatalf("shutdown id = %#v, want 2; stderr=%q", got, stderr.String())
	}
	writeLSPMessage(t, stdin, map[string]any{"jsonrpc": "2.0", "method": "exit"})
	if err := stdin.Close(); err != nil {
		t.Fatalf("close stdin: %v", err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("leia-lsp exited with error: %v; stderr=%q", err, stderr.String())
	}
}

func writeLSPMessage(t *testing.T, w io.Writer, msg map[string]any) {
	t.Helper()
	payload, err := json.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fmt.Fprintf(w, "Content-Length: %d\r\n\r\n%s", len(payload), payload); err != nil {
		t.Fatalf("write LSP message: %v", err)
	}
}

func readLSPMessage(t *testing.T, r *bufio.Reader) map[string]any {
	t.Helper()
	contentLength := -1
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			t.Fatalf("read LSP header: %v", err)
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		name, value, ok := strings.Cut(line, ":")
		if !ok {
			t.Fatalf("malformed LSP header %q", line)
		}
		if strings.EqualFold(strings.TrimSpace(name), "Content-Length") {
			if _, err := fmt.Sscanf(strings.TrimSpace(value), "%d", &contentLength); err != nil {
				t.Fatalf("invalid Content-Length %q: %v", value, err)
			}
		}
	}
	if contentLength < 0 {
		t.Fatal("missing Content-Length")
	}
	payload := make([]byte, contentLength)
	if _, err := io.ReadFull(r, payload); err != nil {
		t.Fatalf("read LSP payload: %v", err)
	}
	var msg map[string]any
	if err := json.Unmarshal(payload, &msg); err != nil {
		t.Fatalf("decode LSP payload %q: %v", payload, err)
	}
	return msg
}
