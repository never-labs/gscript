package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	leia "github.com/never-labs/leia"
)

const (
	defaultPlaygroundAddr           = "127.0.0.1:8080"
	defaultPlaygroundTimeout        = 3 * time.Second
	defaultPlaygroundMaxSourceBytes = 64 << 10
	defaultPlaygroundMaxSteps       = 2_000_000
)

type playgroundOptions struct {
	Addr           string
	Timeout        time.Duration
	MaxSourceBytes int64
	MaxSteps       int64
	Executable     string
	Runner         playgroundRunner
}

type playgroundRunner func(context.Context, playgroundRunRequest, playgroundOptions) playgroundRunResponse

type playgroundRunRequest struct {
	Source string   `json:"source"`
	Mode   string   `json:"mode"`
	Args   []string `json:"args,omitempty"`
}

type playgroundRunResponse struct {
	OK         bool   `json:"ok"`
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr,omitempty"`
	Error      string `json:"error,omitempty"`
	DurationMS int64  `json:"duration_ms"`
	TimedOut   bool   `json:"timed_out,omitempty"`
	ExitCode   int    `json:"exit_code,omitempty"`
}

type playgroundExample struct {
	Name   string `json:"name"`
	Source string `json:"source"`
}

func runPlaygroundCommand(args []string, outw, errw io.Writer) int {
	fs := flag.NewFlagSet("playground", flag.ContinueOnError)
	fs.SetOutput(errw)
	addr := fs.String("addr", defaultPlaygroundAddr, "listen address")
	timeout := fs.Duration("timeout", defaultPlaygroundTimeout, "per-run timeout")
	maxSourceBytes := fs.Int64("max-source-bytes", defaultPlaygroundMaxSourceBytes, "maximum source bytes per run")
	maxSteps := fs.Int64("max-steps", defaultPlaygroundMaxSteps, "maximum interpreter or bytecode steps per run")
	if code, done := parseCLIFlags(fs, args); done {
		return code
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(errw, "usage: leia playground [--addr ADDR] [--timeout DURATION] [--max-source-bytes N] [--max-steps N]")
		return 2
	}
	if *timeout <= 0 {
		fmt.Fprintln(errw, "leia playground: --timeout must be positive")
		return 2
	}
	if *maxSourceBytes <= 0 {
		fmt.Fprintln(errw, "leia playground: --max-source-bytes must be positive")
		return 2
	}
	if *maxSteps <= 0 {
		fmt.Fprintln(errw, "leia playground: --max-steps must be positive")
		return 2
	}
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(errw, "leia playground: %v\n", err)
		return 1
	}
	opts := playgroundOptions{
		Addr:           *addr,
		Timeout:        *timeout,
		MaxSourceBytes: *maxSourceBytes,
		MaxSteps:       *maxSteps,
		Executable:     exe,
	}
	server := &http.Server{
		Addr:              opts.Addr,
		Handler:           newPlaygroundHandler(opts),
		ReadHeaderTimeout: 5 * time.Second,
	}
	listener, err := net.Listen("tcp", opts.Addr)
	if err != nil {
		fmt.Fprintf(errw, "leia playground: listen %s: %v\n", opts.Addr, err)
		return 1
	}
	fmt.Fprintf(outw, "Leia playground listening on http://%s\n", listener.Addr().String())
	if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
		fmt.Fprintf(errw, "leia playground: %v\n", err)
		return 1
	}
	return 0
}

func newPlaygroundHandler(opts playgroundOptions) http.Handler {
	if opts.Timeout <= 0 {
		opts.Timeout = defaultPlaygroundTimeout
	}
	if opts.MaxSourceBytes <= 0 {
		opts.MaxSourceBytes = defaultPlaygroundMaxSourceBytes
	}
	if opts.MaxSteps <= 0 {
		opts.MaxSteps = defaultPlaygroundMaxSteps
	}
	if opts.Runner == nil {
		opts.Runner = subprocessPlaygroundRunner
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = playgroundPage.Execute(w, struct {
			Timeout string
			MaxKB   int64
		}{
			Timeout: opts.Timeout.String(),
			MaxKB:   opts.MaxSourceBytes / 1024,
		})
	})
	mux.HandleFunc("/api/examples", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		writePlaygroundJSON(w, http.StatusOK, playgroundExamples())
	})
	mux.HandleFunc("/api/run", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		defer r.Body.Close()
		var req playgroundRunRequest
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, opts.MaxSourceBytes+4096))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			writePlaygroundJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		req.Source = strings.TrimPrefix(req.Source, "\ufeff")
		if int64(len(req.Source)) > opts.MaxSourceBytes {
			writePlaygroundJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "source exceeds max-source-bytes"})
			return
		}
		if strings.TrimSpace(req.Source) == "" {
			writePlaygroundJSON(w, http.StatusBadRequest, map[string]string{"error": "source is empty"})
			return
		}
		req.Mode = normalizePlaygroundMode(req.Mode)
		if req.Mode == "" {
			writePlaygroundJSON(w, http.StatusBadRequest, map[string]string{"error": "mode must be interpreter or bytecode"})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), opts.Timeout)
		defer cancel()
		writePlaygroundJSON(w, http.StatusOK, opts.Runner(ctx, req, opts))
	})
	return mux
}

func normalizePlaygroundMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", "interpreter", "interp", "tree":
		return "interpreter"
	case "bytecode", "vm":
		return "bytecode"
	default:
		return ""
	}
}

func subprocessPlaygroundRunner(ctx context.Context, req playgroundRunRequest, opts playgroundOptions) playgroundRunResponse {
	start := time.Now()
	dir, err := os.MkdirTemp("", "leia-playground-*")
	if err != nil {
		return playgroundRunResponse{OK: false, Error: err.Error(), DurationMS: elapsedMillis(start)}
	}
	defer os.RemoveAll(dir)
	sourcePath := filepath.Join(dir, "main.leia")
	if err := os.WriteFile(sourcePath, []byte(req.Source), 0600); err != nil {
		return playgroundRunResponse{OK: false, Error: err.Error(), DurationMS: elapsedMillis(start)}
	}
	exe := opts.Executable
	if exe == "" {
		exe, err = os.Executable()
		if err != nil {
			return playgroundRunResponse{OK: false, Error: err.Error(), DurationMS: elapsedMillis(start)}
		}
	}
	args := []string{"__playground_exec", "--mode", req.Mode, "--max-steps", fmt.Sprint(opts.MaxSteps), sourcePath}
	args = append(args, req.Args...)
	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	resp := playgroundRunResponse{
		OK:         err == nil,
		Stdout:     stdout.String(),
		Stderr:     stderr.String(),
		DurationMS: elapsedMillis(start),
	}
	if err != nil {
		resp.Error = strings.TrimSpace(err.Error())
		if ctx.Err() == context.DeadlineExceeded {
			resp.TimedOut = true
			resp.Error = "execution timed out"
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			resp.ExitCode = exitErr.ExitCode()
		}
	}
	return resp
}

func runPlaygroundExecCommand(args []string, outw, errw io.Writer) int {
	fs := flag.NewFlagSet("__playground_exec", flag.ContinueOnError)
	fs.SetOutput(errw)
	mode := fs.String("mode", "interpreter", "execution mode: interpreter or bytecode")
	maxSteps := fs.Int64("max-steps", defaultPlaygroundMaxSteps, "maximum interpreter or bytecode steps")
	if code, done := parseCLIFlags(fs, args); done {
		return code
	}
	rest := fs.Args()
	if len(rest) == 0 {
		fmt.Fprintln(errw, "usage: leia __playground_exec [--mode interpreter|bytecode] [--max-steps N] <file.leia> [args...]")
		return 2
	}
	normalizedMode := normalizePlaygroundMode(*mode)
	if normalizedMode == "" {
		fmt.Fprintf(errw, "leia playground: invalid mode %q\n", *mode)
		return 2
	}
	var printBuf bytes.Buffer
	opts := []leia.Option{
		leia.SecuritySandbox(),
		leia.WithMaxSteps(*maxSteps),
		leia.WithMaxNativeCalls(100_000),
		leia.WithMaxCallDepth(1024),
		leia.WithMaxGoroutines(128),
		leia.WithMaxChannelCapacity(1024),
		leia.WithMaxHostResultBytes(1 << 20),
		leia.WithPrint(func(args ...interface{}) {
			parts := make([]string, len(args))
			for i, arg := range args {
				parts[i] = fmt.Sprint(arg)
			}
			fmt.Fprintln(&printBuf, strings.Join(parts, "\t"))
		}),
	}
	if normalizedMode == "bytecode" {
		opts = append(opts, leia.WithVM())
	}
	opts = append(opts, leia.WithArgs(rest[0], rest[1:]...))
	vm := leia.New(opts...)
	if err := vm.ExecFile(rest[0]); err != nil {
		_, _ = io.Copy(outw, &printBuf)
		fmt.Fprintf(errw, "error: %v\n", err)
		return 1
	}
	_, _ = io.Copy(outw, &printBuf)
	return 0
}

func writePlaygroundJSON(w http.ResponseWriter, status int, value interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func elapsedMillis(start time.Time) int64 {
	ms := time.Since(start).Milliseconds()
	if ms == 0 {
		return 1
	}
	return ms
}

func playgroundExamples() []playgroundExample {
	return []playgroundExample{
		{
			Name: "Hello",
			Source: `print("hello from Leia")

name := "playground"
print("running", name)`,
		},
		{
			Name: "Functions",
			Source: `func fib(n) {
    if n < 2 {
        return n
    }
    return fib(n - 1) + fib(n - 2)
}

for i := 0; i <= 10; i = i + 1 {
    print(i, fib(i))
}`,
		},
		{
			Name: "Tables",
			Source: `scores := { ada: 98, lin: 91, ken: 87 }
total := 0
for name, score := range scores {
    print(name, score)
    total = total + score
}
print("avg", total / 3)`,
		},
		{
			Name: "Channels",
			Source: `ch := make(chan, 2)
ch <- "alpha"
ch <- "beta"

print(<-ch)
print(<-ch)`,
		},
	}
}

var playgroundPage = template.Must(template.New("playground").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Leia Playground</title>
<style>
:root {
  color-scheme: light dark;
  --bg: #f7f7f4;
  --panel: #ffffff;
  --text: #161616;
  --muted: #656565;
  --border: #d7d7d2;
  --accent: #0b6bcb;
  --accent-text: #ffffff;
  --code: #0f1720;
}
@media (prefers-color-scheme: dark) {
  :root {
    --bg: #101112;
    --panel: #181a1d;
    --text: #f2f2ef;
    --muted: #a4a4a0;
    --border: #303236;
    --accent: #69a7ff;
    --accent-text: #07111f;
    --code: #0b0d10;
  }
}
* { box-sizing: border-box; }
body {
  margin: 0;
  background: var(--bg);
  color: var(--text);
  font: 14px/1.45 ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
}
header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 14px 18px;
  border-bottom: 1px solid var(--border);
  background: var(--panel);
}
h1 {
  margin: 0;
  font-size: 18px;
  font-weight: 650;
}
.meta {
  color: var(--muted);
  font-size: 13px;
}
main {
  display: grid;
  grid-template-columns: minmax(0, 1.25fr) minmax(320px, 0.75fr);
  min-height: calc(100vh - 57px);
}
.editor, .output {
  min-width: 0;
  padding: 16px;
}
.editor {
  border-right: 1px solid var(--border);
}
.toolbar {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 12px;
  flex-wrap: wrap;
}
select, button {
  height: 34px;
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 0 10px;
  font: inherit;
  background: var(--panel);
  color: var(--text);
}
button.primary {
  border-color: var(--accent);
  background: var(--accent);
  color: var(--accent-text);
  font-weight: 650;
}
.codewrap {
  position: relative;
  width: 100%;
  min-height: calc(100vh - 130px);
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--code);
  overflow: hidden;
}
.highlight, textarea {
  margin: 0;
  padding: 14px;
  font: 13px/1.55 ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  tab-size: 4;
  white-space: pre;
  overflow: auto;
}
.highlight {
  position: absolute;
  inset: 0;
  min-height: 100%;
  border: 0;
  border-radius: 0;
  background: transparent;
  color: #f4f4f0;
  pointer-events: none;
}
textarea {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  resize: none;
  border: 0;
  outline: 0;
  background: transparent;
  color: transparent;
  -webkit-text-fill-color: transparent;
  caret-color: #f4f4f0;
  font: 13px/1.55 ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
}
textarea::selection {
  background: rgba(105, 167, 255, 0.35);
}
.tok-keyword { color: #78a8ff; font-weight: 650; }
.tok-string { color: #9bd67d; }
.tok-number { color: #e7c56e; }
.tok-comment { color: #7d8793; font-style: italic; }
.tok-builtin { color: #82d8d8; }
.tok-agent { color: #d8a8ff; font-weight: 650; }
.tok-op { color: #c8d0d9; }
.tok-ident { color: #f4f4f0; }
.tok-error { color: #ff9a9a; }
@media (prefers-color-scheme: light) {
  .tok-keyword { color: #064fa8; }
  .tok-string { color: #257a2c; }
  .tok-number { color: #875f00; }
  .tok-comment { color: #667085; }
  .tok-builtin { color: #006c82; }
  .tok-agent { color: #7a3db8; }
}
pre {
  min-height: 220px;
  margin: 0;
  overflow: auto;
  white-space: pre-wrap;
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 14px;
  background: var(--panel);
  font: 13px/1.55 ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
}
.status {
  margin: 0 0 12px;
  color: var(--muted);
}
.error {
  color: #b42318;
}
@media (max-width: 820px) {
  main { grid-template-columns: 1fr; }
  .editor { border-right: 0; border-bottom: 1px solid var(--border); }
  .codewrap { min-height: 420px; }
}
</style>
</head>
<body>
<header>
  <h1>Leia Playground</h1>
  <div class="meta">Backend execution, timeout {{.Timeout}}, max source {{.MaxKB}} KB</div>
</header>
<main>
  <section class="editor">
    <div class="toolbar">
      <select id="examples" aria-label="Examples"></select>
      <select id="mode" aria-label="Execution mode">
        <option value="interpreter">Interpreter</option>
        <option value="bytecode">Bytecode VM</option>
      </select>
      <button class="primary" id="run">Run</button>
    </div>
    <div class="codewrap">
      <pre class="highlight" id="highlight" aria-hidden="true"></pre>
      <textarea id="source" spellcheck="false" autocomplete="off" autocapitalize="off"></textarea>
    </div>
  </section>
  <section class="output">
    <p class="status" id="status">Ready.</p>
    <pre id="output"></pre>
  </section>
</main>
<script>
const source = document.getElementById("source");
const highlight = document.getElementById("highlight");
const output = document.getElementById("output");
const statusLine = document.getElementById("status");
const examples = document.getElementById("examples");
const mode = document.getElementById("mode");

const leiaKeywords = new Set([
  "agent", "and", "break", "continue", "defer", "do", "else", "elseif", "end",
  "evaluate", "false", "flow", "for", "func", "go", "if", "in", "local", "nil",
  "not", "or", "return", "then", "tool", "true", "turn", "while"
]);
const leiaBuiltins = new Set([
  "print", "pairs", "ipairs", "range", "len", "type", "tostring", "tonumber",
  "error", "pcall", "xpcall", "require", "assert", "make", "close", "sleep"
]);
const leiaAgentFields = new Set([
  "model", "tools", "system", "user", "output", "example", "history", "messages",
  "budget", "parallel", "metric", "fail_if", "record_to", "filter", "seed"
]);

function escapeHTML(text) {
  return text.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
}

function span(cls, text) {
  return "<span class=\"" + cls + "\">" + escapeHTML(text) + "</span>";
}

function highlightLeia(text) {
  let out = "";
  let i = 0;
  while (i < text.length) {
    const ch = text[i];
    const next = text[i + 1] || "";
    if (ch === "-" && next === "-") {
      let j = i + 2;
      while (j < text.length && text[j] !== "\n") j++;
      out += span("tok-comment", text.slice(i, j));
      i = j;
      continue;
    }
    if (ch === "\"" || ch === "'") {
      const quote = ch;
      let j = i + 1;
      while (j < text.length) {
        if (text[j] === "\\") {
          j += 2;
          continue;
        }
        if (text[j] === quote) {
          j++;
          break;
        }
        j++;
      }
      out += span("tok-string", text.slice(i, j));
      i = j;
      continue;
    }
    if (/[0-9]/.test(ch)) {
      let j = i + 1;
      while (j < text.length && /[0-9_]/.test(text[j])) j++;
      if (text[j] === "." && /[0-9]/.test(text[j + 1] || "")) {
        j++;
        while (j < text.length && /[0-9_]/.test(text[j])) j++;
      }
      out += span("tok-number", text.slice(i, j));
      i = j;
      continue;
    }
    if (/[A-Za-z_]/.test(ch)) {
      let j = i + 1;
      while (j < text.length && /[A-Za-z0-9_]/.test(text[j])) j++;
      const word = text.slice(i, j);
      const k = j;
      while (j < text.length && /[ \t]/.test(text[j])) j++;
      const isField = leiaAgentFields.has(word) && text[j] === ":";
      if (isField) out += span("tok-agent", word);
      else if (leiaKeywords.has(word)) out += span("tok-keyword", word);
      else if (leiaBuiltins.has(word)) out += span("tok-builtin", word);
      else out += span("tok-ident", word);
      i = k;
      continue;
    }
    if ("+-*/%=<>!&|.#:;,.(){}[]".includes(ch)) {
      out += span("tok-op", ch);
      i++;
      continue;
    }
    out += escapeHTML(ch);
    i++;
  }
  return out.endsWith("\n") ? out + " " : out;
}

function refreshHighlight() {
  highlight.innerHTML = highlightLeia(source.value);
  highlight.scrollTop = source.scrollTop;
  highlight.scrollLeft = source.scrollLeft;
}

async function loadExamples() {
  const res = await fetch("/api/examples");
  const data = await res.json();
  for (const item of data) {
    const opt = document.createElement("option");
    opt.value = item.name;
    opt.textContent = item.name;
    examples.appendChild(opt);
  }
  examples._items = data;
  if (data.length) {
    source.value = data[0].source;
    refreshHighlight();
  }
}

examples.addEventListener("change", () => {
  const item = (examples._items || []).find(x => x.name === examples.value);
  if (item) {
    source.value = item.source;
    refreshHighlight();
  }
});

async function run() {
  statusLine.textContent = "Running...";
  statusLine.className = "status";
  output.textContent = "";
  try {
    const res = await fetch("/api/run", {
      method: "POST",
      headers: {"Content-Type": "application/json"},
      body: JSON.stringify({source: source.value, mode: mode.value})
    });
    const data = await res.json();
    const parts = [];
    if (data.stdout) parts.push(data.stdout);
    if (data.stderr) parts.push(data.stderr);
    if (data.error && !data.stderr) parts.push(data.error);
    output.textContent = parts.join(parts.length > 1 ? "\n" : "");
    statusLine.textContent = data.ok ? ("OK in " + data.duration_ms + " ms") : ("Failed in " + (data.duration_ms || 0) + " ms");
    statusLine.className = data.ok ? "status" : "status error";
  } catch (err) {
    statusLine.textContent = "Request failed.";
    statusLine.className = "status error";
    output.textContent = String(err);
  }
}

document.getElementById("run").addEventListener("click", run);
source.addEventListener("input", refreshHighlight);
source.addEventListener("scroll", () => {
  highlight.scrollTop = source.scrollTop;
  highlight.scrollLeft = source.scrollLeft;
});
source.addEventListener("keydown", (event) => {
  if ((event.metaKey || event.ctrlKey) && event.key === "Enter") {
    run();
    return;
  }
  if (event.key === "Tab") {
    event.preventDefault();
    const start = source.selectionStart;
    const end = source.selectionEnd;
    source.value = source.value.slice(0, start) + "    " + source.value.slice(end);
    source.selectionStart = source.selectionEnd = start + 4;
    refreshHighlight();
  }
});
loadExamples();
</script>
</body>
</html>`))
