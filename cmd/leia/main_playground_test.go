package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestPlaygroundRunAPI(t *testing.T) {
	handler := newPlaygroundHandler(playgroundOptions{
		Timeout:        time.Second,
		MaxSourceBytes: 1024,
		MaxSteps:       1000,
		Runner: func(ctx context.Context, req playgroundRunRequest, opts playgroundOptions) playgroundRunResponse {
			if req.Mode != "bytecode" {
				t.Fatalf("mode = %q, want bytecode", req.Mode)
			}
			if req.Source != `print("hi")` {
				t.Fatalf("source = %q", req.Source)
			}
			return playgroundRunResponse{OK: true, Stdout: "hi\n", DurationMS: 2}
		},
	})
	body := strings.NewReader(`{"source":"print(\"hi\")","mode":"vm"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/run", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var resp playgroundRunResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp.OK || resp.Stdout != "hi\n" || resp.DurationMS != 2 {
		t.Fatalf("response = %#v", resp)
	}
}

func TestPlaygroundRunAPIRejectsOversizedSource(t *testing.T) {
	handler := newPlaygroundHandler(playgroundOptions{
		Timeout:        time.Second,
		MaxSourceBytes: 8,
		MaxSteps:       1000,
		Runner: func(context.Context, playgroundRunRequest, playgroundOptions) playgroundRunResponse {
			t.Fatal("runner should not be called")
			return playgroundRunResponse{}
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/run", strings.NewReader(`{"source":"123456789","mode":"interpreter"}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestPlaygroundExamplesAPI(t *testing.T) {
	handler := newPlaygroundHandler(playgroundOptions{
		Runner: func(context.Context, playgroundRunRequest, playgroundOptions) playgroundRunResponse {
			return playgroundRunResponse{}
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/examples", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var examples []playgroundExample
	if err := json.Unmarshal(rec.Body.Bytes(), &examples); err != nil {
		t.Fatalf("decode examples: %v", err)
	}
	if len(examples) < 3 {
		t.Fatalf("examples = %d, want at least 3", len(examples))
	}
}

func TestPlaygroundExamplesExecute(t *testing.T) {
	for _, example := range playgroundExamples() {
		t.Run(example.Name, func(t *testing.T) {
			dir := t.TempDir()
			path := dir + "/main.leia"
			if err := os.WriteFile(path, []byte(example.Source), 0600); err != nil {
				t.Fatal(err)
			}
			var stdout, stderr bytes.Buffer
			code := runPlaygroundExecCommand([]string{"--max-steps=2000000", path}, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("example failed with code %d\nstdout:\n%s\nstderr:\n%s\nsource:\n%s", code, stdout.String(), stderr.String(), example.Source)
			}
			if strings.TrimSpace(stdout.String()) == "" {
				t.Fatalf("example produced no stdout\nsource:\n%s", example.Source)
			}
		})
	}
}

func TestPlaygroundCommandRejectsBadFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runPlaygroundCommand([]string{"--timeout=0s"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "--timeout must be positive") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestPlaygroundExecRunsSandboxedSource(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/main.leia"
	if err := os.WriteFile(path, []byte(`print("ok")`), 0600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := runPlaygroundExecCommand([]string{"--max-steps=1000", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code = %d, stderr = %s", code, stderr.String())
	}
	if stdout.String() != "ok\n" {
		t.Fatalf("stdout = %q", stdout.String())
	}
}
