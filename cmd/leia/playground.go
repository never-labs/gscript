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
	"github.com/never-labs/leia/llm"
	"github.com/never-labs/leia/llm/anthropic"
	"github.com/never-labs/leia/llm/openai"
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
	Source  string   `json:"source"`
	Mode    string   `json:"mode"`
	Profile string   `json:"profile,omitempty"`
	Args    []string `json:"args,omitempty"`
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
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Section  string   `json:"section"`
	Summary  string   `json:"summary"`
	Concepts []string `json:"concepts,omitempty"`
	Source   string   `json:"source"`
	Runnable bool     `json:"runnable"`
	Requires string   `json:"requires,omitempty"`
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
	mux.HandleFunc("/api/tour", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		writePlaygroundJSON(w, http.StatusOK, playgroundTourLessons())
	})
	mux.HandleFunc("/api/ai", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		writePlaygroundJSON(w, http.StatusOK, playgroundAIExamples())
	})
	mux.HandleFunc("/api/examples", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		examples, err := playgroundRepositoryExamples(playgroundExamplesRoot())
		if err != nil {
			writePlaygroundJSON(w, http.StatusOK, []playgroundExample{})
			return
		}
		writePlaygroundJSON(w, http.StatusOK, examples)
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
		timeout := opts.Timeout
		if normalizePlaygroundProfile(req.Profile) == "ai" && timeout < 60*time.Second {
			timeout = 60 * time.Second
		}
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
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

func normalizePlaygroundProfile(profile string) string {
	switch strings.ToLower(strings.TrimSpace(profile)) {
	case "", "sandbox":
		return "sandbox"
	case "ai", "llm", "glm":
		return "ai"
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
	args := []string{"__playground_exec", "--mode", req.Mode, "--profile", normalizePlaygroundProfile(req.Profile), "--max-steps", fmt.Sprint(opts.MaxSteps), sourcePath}
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
	profile := fs.String("profile", "sandbox", "execution profile: sandbox or ai")
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
	normalizedProfile := normalizePlaygroundProfile(*profile)
	if normalizedProfile == "" {
		fmt.Fprintf(errw, "leia playground: invalid profile %q\n", *profile)
		return 2
	}
	var printBuf bytes.Buffer
	sourcePath := rest[0]
	if normalizedProfile == "ai" {
		prefixed, err := playgroundAIProfileSource(sourcePath)
		if err != nil {
			fmt.Fprintf(errw, "error: %v\n", err)
			return 1
		}
		sourcePath = prefixed
		defer os.Remove(prefixed)
	}
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
	if normalizedProfile == "ai" {
		opts = append(opts, playgroundAIProfileOptions()...)
	}
	opts = append(opts, leia.WithArgs(sourcePath, rest[1:]...))
	vm := leia.New(opts...)
	if err := vm.ExecFile(sourcePath); err != nil {
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

func playgroundAIProfileOptions() []leia.Option {
	opts := []leia.Option{
		leia.WithLibs(leia.LibSafe | leia.LibOS | leia.LibLLM),
		leia.WithCapabilities(leia.CapEnvironmentRead),
		leia.WithEnvironmentRead(true),
		leia.WithEnvironmentAllowlist(
			"LEIA_GLM_BASE_URL",
			"LEIA_GLM_API_KEY",
			"LEIA_GLM_MODEL",
			"SENTINEL_GLM_API_KEY",
			"GLM_API_KEY",
			"GLM_MODEL",
			"ANTHROPIC_BASE_URL",
			"ANTHROPIC_AUTH_TOKEN",
			"ANTHROPIC_MODEL",
		),
		leia.WithNetworkAccess(true),
		leia.WithLLMProviderFactory(playgroundLLMProviderFactory),
	}
	if os.Getenv("LEIA_PLAYGROUND_MOCK_LLM") != "" {
		opts = append(opts, leia.WithLLMProvider(playgroundMockLLMProvider{}))
		return opts
	}
	if provider, ok := playgroundGLMProviderFromEnv(); ok {
		opts = append(opts, leia.WithLLMProvider(provider))
	}
	return opts
}

type playgroundMockLLMProvider struct{}

func (playgroundMockLLMProvider) Turn(_ context.Context, req llm.TurnRequest) (llm.TurnResult, error) {
	if wantsJSONObject(req.ResponseFormat) {
		if requestContains(req, "slugify") {
			return llm.TurnResult{Status: "final_answer", Text: playgroundMockSlugifyPatch()}, nil
		}
		if requestContains(req, "structured research handoff") || requestContains(req, "Research ") {
			return llm.TurnResult{Status: "final_answer", Text: `{"summary":"delegated research ok","confidence":1}`}, nil
		}
		if requestContains(req, "codename") || requestContains(req, "direct-agent-tool") {
			return llm.TurnResult{Status: "final_answer", Text: `{"project":"ORCHID","owner":"ADA","remembered":true,"source":"direct-agent-tool"}`}, nil
		}
		if requestContains(req, "project") || requestContains(req, "ORCHID") || requestContains(req, "memory") {
			return llm.TurnResult{Status: "final_answer", Text: `{"project":"ORCHID","owner":"ADA","risk":"LOW","remembered":true,"meta":{"source":"mock"}}`}, nil
		}
		return llm.TurnResult{Status: "final_answer", Text: `{"product":"playground","severity":"low","action":"improve_demos"}`}, nil
	}
	if len(req.Messages) > 0 {
		last := req.Messages[len(req.Messages)-1]
		if last.Role == "tool" {
			return llm.TurnResult{Status: "final_answer", Text: "MOCK_TOOL_RESULT " + fmt.Sprint(last.Value)}, nil
		}
	}
	if len(req.Tools) > 0 {
		tool := req.Tools[0].Name
		switch tool {
		case "extract_memory":
			return llm.TurnResult{Status: "tool_calls", Calls: []llm.ToolCall{{
				ID: "call_extract_1", Tool: tool, Args: map[string]any{"note": lastUserText(req)},
			}}}, nil
		case "search_runbook":
			return llm.TurnResult{Status: "tool_calls", Calls: []llm.ToolCall{{
				ID: "call_runbook_1", Tool: tool, Args: map[string]any{"service": "checkout", "symptom": "p95 latency spike"},
			}}}, nil
		case "get_metrics":
			return llm.TurnResult{Status: "tool_calls", Calls: []llm.ToolCall{{
				ID: "call_metrics_1", Tool: tool, Args: map[string]any{"service": "checkout"},
			}}}, nil
		case "lookup_order":
			return llm.TurnResult{Status: "tool_calls", Calls: []llm.ToolCall{{
				ID: "call_order_1", Tool: tool, Args: map[string]any{"id": "A100"},
			}}}, nil
		default:
			args := map[string]any{"query": "memory"}
			if len(req.Tools[0].Params) > 0 && req.Tools[0].Params[0] != "query" {
				args = map[string]any{req.Tools[0].Params[0]: lastUserText(req)}
			}
			return llm.TurnResult{Status: "tool_calls", Calls: []llm.ToolCall{{
				ID: "call_tool_1", Tool: tool, Args: args,
			}}}, nil
		}
	}
	if requestContains(req, "PASS") {
		return llm.TurnResult{Status: "final_answer", Text: "PASS"}, nil
	}
	if requestContains(req, "project=ORCHID") {
		return llm.TurnResult{Status: "final_answer", Text: "project=ORCHID;owner=ADA;risk=LOW"}, nil
	}
	if requestContains(req, "MEMORY_STORED") {
		return llm.TurnResult{Status: "final_answer", Text: "MEMORY_STORED"}, nil
	}
	if requestContains(req, "LEIA_GLM_OK") {
		return llm.TurnResult{Status: "final_answer", Text: "LEIA_GLM_OK"}, nil
	}
	return llm.TurnResult{Status: "final_answer", Text: "MOCK_AI_OK"}, nil
}

func (p playgroundMockLLMProvider) StreamTurn(ctx context.Context, req llm.TurnRequest, sink llm.StreamSink) (llm.TurnResult, error) {
	res, err := p.Turn(ctx, req)
	if err != nil || sink == nil || res.Text == "" {
		return res, err
	}
	for i, token := range strings.Fields(res.Text) {
		if i > 0 {
			if err := sink(llm.StreamEvent{Type: "token", Token: " ", Text: " "}); err != nil {
				return llm.TurnResult{}, err
			}
		}
		if err := sink(llm.StreamEvent{Type: "token", Token: token, Text: token}); err != nil {
			return llm.TurnResult{}, err
		}
	}
	return res, nil
}

func wantsJSONObject(format any) bool {
	if format == nil {
		return false
	}
	if m, ok := format.(map[string]any); ok {
		return fmt.Sprint(m["type"]) == "json_object"
	}
	return strings.Contains(fmt.Sprint(format), "json_object")
}

func requestContains(req llm.TurnRequest, needle string) bool {
	haystack := strings.ToLower(needle)
	for _, msg := range req.Messages {
		if strings.Contains(strings.ToLower(msg.Text), haystack) || strings.Contains(strings.ToLower(fmt.Sprint(msg.Value)), haystack) {
			return true
		}
	}
	return strings.Contains(strings.ToLower(req.Model), haystack)
}

func lastUserText(req llm.TurnRequest) string {
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "user" {
			return req.Messages[i].Text
		}
	}
	return ""
}

func playgroundMockSlugifyPatch() string {
	code := `func slugify(title) {
    value := string.lower(string.trim(title))
    value, _ = string.gsub(value, "%s+", "-")
    return value
}

	assert(slugify("Hello Leia") == "hello-leia")
	assert(slugify("AI Native Script") == "ai-native-script")`
	return jsonEncodeObject(map[string]string{
		"code": code,
		"risk": "mock proposal only",
	})
}

func jsonEncodeObject(value map[string]string) string {
	var buf bytes.Buffer
	_ = json.NewEncoder(&buf).Encode(value)
	return strings.TrimSpace(buf.String())
}

func playgroundGLMProviderFromEnv() (llm.Provider, bool) {
	key := firstNonEmptyEnv("LEIA_GLM_API_KEY", "SENTINEL_GLM_API_KEY", "GLM_API_KEY", "ANTHROPIC_AUTH_TOKEN")
	if key == "" {
		return nil, false
	}
	return anthropic.Provider{
		Endpoint: firstNonEmptyEnvOr("https://open.bigmodel.cn/api/anthropic", "LEIA_GLM_BASE_URL", "ANTHROPIC_BASE_URL"),
		APIKey:   key,
		Model:    firstNonEmptyEnvOr("GLM-5.1", "LEIA_GLM_MODEL", "GLM_MODEL", "ANTHROPIC_MODEL"),
		Timeout:  60 * time.Second,
	}, true
}

func firstNonEmptyEnv(names ...string) string {
	for _, name := range names {
		if value := os.Getenv(name); value != "" {
			return value
		}
	}
	return ""
}

func firstNonEmptyEnvOr(fallback string, names ...string) string {
	if value := firstNonEmptyEnv(names...); value != "" {
		return value
	}
	return fallback
}

func playgroundLLMProviderFactory(cfg llm.ProviderConfig) (llm.Provider, error) {
	protocol := strings.ToLower(strings.ReplaceAll(cfg.Protocol, "_", "-"))
	switch protocol {
	case "openai", "openai-compatible", "openai-compat", "chat-completions":
		return openai.Provider{
			Endpoint: cfg.BaseURL,
			APIKey:   cfg.APIKey,
			Model:    cfg.ProviderModel,
		}, nil
	case "anthropic", "anthropic-compatible", "anthropic-compat", "messages":
		return anthropic.Provider{
			Endpoint: cfg.BaseURL,
			APIKey:   cfg.APIKey,
			Model:    cfg.ProviderModel,
			Timeout:  defaultPlaygroundTimeout,
		}, nil
	default:
		if cfg.Protocol == "" {
			return nil, fmt.Errorf("llm provider protocol not configured for model %q", cfg.Name)
		}
		return nil, fmt.Errorf("unsupported llm provider protocol %q for model %q", cfg.Protocol, cfg.Name)
	}
}

func playgroundAIProfileSource(path string) (string, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	out := filepath.Join(filepath.Dir(path), "main_ai.leia")
	if err := os.WriteFile(out, append([]byte(playgroundAIEnvHelper()), src...), 0600); err != nil {
		return "", err
	}
	return out, nil
}

func playgroundAIEnvHelper() string {
	return `func __leia_playground_env_first(a, b, c, fallback) {
    v := os.getenv(a)
    if v != "" { return v }
    v = os.getenv(b)
    if v != "" { return v }
    v = os.getenv(c)
    if v != "" { return v }
    return fallback
}

`
}

func elapsedMillis(start time.Time) int64 {
	ms := time.Since(start).Milliseconds()
	if ms == 0 {
		return 1
	}
	return ms
}

func playgroundTourLessons() []playgroundExample {
	return []playgroundExample{
		{
			ID:      "welcome",
			Title:   "A Useful First Script",
			Section: "Basics",
			Summary: "Start with a tiny data-cleanup script instead of isolated print statements.",
			Concepts: []string{
				"`:=` creates a local binding.",
				"Tables can hold records with named fields.",
				"Standard library calls compose with ordinary script code.",
			},
			Source: `raw := {
    { name: "Ada", email: " ADA@EXAMPLE.COM ", active: true },
    { name: "Lin", email: " lin@example.com ", active: false },
    { name: "Grace", email: " Grace@Example.com ", active: true },
}

clean := {}
for _, user := range raw {
    if !user.active {
        continue
    }
    clean[#clean + 1] = {
        name: user.name,
        email: string.lower(string.trim(user.email)),
    }
}

print(json.encode(clean))`,
			Runnable: true,
		},
		{
			ID:      "control-flow",
			Title:   "Control Flow And Scope",
			Section: "Basics",
			Summary: "Use ordinary imperative control flow with lexical block scope.",
			Concepts: []string{
				"`if` conditions use blocks with braces.",
				"Only `nil` and `false` are false.",
				"Loop bodies can build up tables incrementally.",
			},
			Source: `events := {
    { type: "click", value: 1 },
    { type: "view", value: 5 },
    { type: "click", value: 2 },
    { type: "error", value: 99 },
}

counts := {}
for _, event := range events {
    if event.type == "error" {
        continue
    }
    if counts[event.type] == nil {
        counts[event.type] = 0
    }
    counts[event.type] = counts[event.type] + event.value
}

for kind, count := range counts {
    print(kind, count)
}`,
			Runnable: true,
		},
		{
			ID:      "functions",
			Title:   "Functions And Multiple Returns",
			Section: "Functions",
			Summary: "Functions are values and can return status plus data without special syntax.",
			Concepts: []string{
				"`func name(args) { ... }` declares a function.",
				"Multiple returns make result/error-style APIs natural.",
				"Closures can capture local state.",
			},
			Source: `func parse_score(row) {
    parts := string.split(row, ":")
    if #parts != 2 {
        return nil, "bad row: " .. row
    }
    return { name: string.trim(parts[1]), score: tonumber(parts[2]) }, nil
}

func counter() {
    n := 0
    return func() {
        n = n + 1
        return n
    }
}

next_id := counter()
for _, row := range { "Ada:98", "bad", "Grace:95" } {
    item, err := parse_score(row)
    if err != nil {
        print("skip", err)
        continue
    }
    print(next_id(), item.name, item.score)
}`,
			Runnable: true,
		},
		{
			ID:      "tables",
			Title:   "Tables As Records And Maps",
			Section: "Data",
			Summary: "Tables cover records, arrays, dictionaries, and nested data structures.",
			Concepts: []string{
				"Array indexes are one-based.",
				"Named fields and computed fields share the same table value.",
				"`pairs` and `range` work well for grouping and aggregation.",
			},
			Source: `orders := {
    { region: "apac", amount: 12 },
    { region: "emea", amount: 20 },
    { region: "apac", amount: 30 },
}

by_region := {}
for _, order := range orders {
    bucket := by_region[order.region]
    if bucket == nil {
        bucket = { total: 0, count: 0 }
        by_region[order.region] = bucket
    }
    bucket.total = bucket.total + order.amount
    bucket.count = bucket.count + 1
}

print(json.encode(by_region))`,
			Runnable: true,
		},
		{
			ID:      "strings-stdlib",
			Title:   "Standard Library Pipeline",
			Section: "Standard Library",
			Summary: "Safe standard-library modules are available without filesystem or network access.",
			Concepts: []string{
				"Standard libraries are available as global module tables.",
				"JSON, string, table, and math helpers are enough for many host scripts.",
				"Structured output stays easy to pass back to Go.",
			},
			Source: `items := { "alpha", "beta", "alpha", "gamma" }
seen := {}
unique := {}
for _, item := range items {
    key := string.upper(item)
    if seen[key] == nil {
        seen[key] = true
        unique[#unique + 1] = key
    }
}

table.sort(unique)
print(table.concat(unique, ","))
print("distance", math.sqrt(3 * 3 + 4 * 4))`,
			Runnable: true,
		},
		{
			ID:      "errors",
			Title:   "Errors And Recovery",
			Section: "Robustness",
			Summary: "Protected calls let host-facing scripts recover and return useful fallback data.",
			Concepts: []string{
				"`error` raises a runtime error.",
				"`pcall` returns a status plus values instead of aborting.",
				"`assert` is useful in tests and evaluate blocks.",
			},
			Source: `func require_field(row, name) {
    value := row[name]
    if value == nil {
        error("missing field: " .. name)
    }
    return value
}

rows := { { name: "Ada" }, { email: "lin@example.com" } }
for _, row := range rows {
    ok, value := pcall(require_field, row, "name")
    if ok {
        print("name", value)
    } else {
        print("recover", value)
    }
}`,
			Runnable: true,
		},
		{
			ID:      "concurrency",
			Title:   "Go-Style Concurrency",
			Section: "Concurrency",
			Summary: "Use goroutines, channels, and wait groups for simple parallel coordination.",
			Concepts: []string{
				"`go func() { ... }()` starts concurrent work.",
				"Channels pass results back to the coordinator.",
				"`sync.waitgroup()` waits for a fixed set of tasks.",
			},
			Source: `jobs := { 3, 5, 8, 13 }
out := make(chan, #jobs)
wg := sync.waitgroup()

for _, n := range jobs {
    wg.add(1)
    go func(value) {
        out <- value * value
        wg.done()
    }(n)
}

wg.wait()
sum := 0
for i := 1; i <= #jobs; i++ {
    sum = sum + (<-out)
}
print("sum of squares", sum)`,
			Runnable: true,
		},
		{
			ID:      "data-oriented",
			Title:   "Data-Oriented Arrays",
			Section: "Data",
			Summary: "Dense arrays and SoA tables give numeric scripts a clear, optimizable shape.",
			Concepts: []string{
				"`[]f64{...}` and `[]i64{...}` create dense arrays.",
				"`soa.zip` groups columns without losing column access.",
				"Column kernels are the public shape for future JIT wins.",
			},
			Source: `particles := soa.zip({
    x: []f64{0, 1, 2, 3},
    vx: []f64{0.5, 0.25, -0.5, 1.0},
    id: []i64{101, 102, 103, 104},
})

soa.addScaled(particles, "x", "vx", 2.0)
fast := soa.mask(particles, "vx", ">=", 0.5)

print("positions", json.encode(soa.unzip(particles).x))
print("fast count", soa.countWhere(particles, fast))
print("fast ids", json.encode(soa.select(particles, fast, "id", 0)))`,
			Runnable: true,
		},
		{
			ID:      "dialects",
			Title:   "Built-In Dialects",
			Section: "Shell And Data",
			Summary: "Tagged strings and blocks let scripts use JSON, regexp, HTTP, template, and prompt-shaped data without leaving Leia.",
			Concepts: []string{
				"`json`...`` decodes JSON text into ordinary Leia values.",
				"`re`...`` builds a regular expression object.",
				"`httpmsg { ... }` and `mime` help build protocol fixtures without network access.",
				"`template { ... }` renders small deterministic text artifacts.",
				"`prompt { ... }` creates structured prompt data.",
				"Dialect tags are explicit, so they stay inspectable by tooling.",
			},
			Source: `name := "Leia"
doc := json` + "`{\"project\":\"leia\",\"kind\":\"dialect\"}`" + `
rx := re` + "`^Lei`" + `
media := dialect.eval("mime", "application/json; charset=utf-8")
headers := {}
headers["content-type"] = media.type
request := httpmsg {
    method: "GET",
    target: "/health",
    headers: headers,
}
summary := template {
    text: "{{.project}} serves {{.media}}",
    data: {project: doc.project, media: media.type},
}
task := prompt {
    system: "Be brief."
    user: "Explain " .. doc.kind
}

print(doc.project, doc.kind)
print(rx.match(name))
print(summary)
print(request)
print(task.body.user)`,
			Runnable: true,
		},
	}
}

func playgroundAIExamples() []playgroundExample {
	return []playgroundExample{
		{
			ID:       "ai-one-line",
			Title:    "One-Line Turn",
			Section:  "AI Basics",
			Summary:  "The smallest useful AI call is one turn with a user message.",
			Runnable: true,
			Requires: "LLM provider",
			Concepts: []string{
				"`llm.turn` sends one request to the selected default model.",
				"The result carries text, usage, history, and tool-call metadata.",
				"This example is intentionally one line; configure a model provider before running.",
			},
			Source: `result, err := llm.turn({ messages: {llm.user("Reply exactly: LEIA_GLM_OK")} })
if err != nil { print(err.message); return }
print(result.text)`,
		},
		{
			ID:       "ai-agent-shape",
			Title:    "Reusable Agent",
			Section:  "AI Basics",
			Summary:  "An agent is a callable prompt capsule with defaults, input, and ordinary return values.",
			Runnable: true,
			Requires: "LLM provider",
			Concepts: []string{
				"`llm.agent` creates callable AI values.",
				"The host-provided GLM profile supplies the default model.",
				"Use plain text for the simplest live agent path.",
			},
			Source: `summarize := llm.agent("summarize", func(topic) {
    return {
        messages: {
            llm.system("Summarize in one sentence for an engineer evaluating Leia. Return plain text only."),
            llm.user(topic),
        }
    }
}, nil, {params: {"topic"}})

result, err := summarize("Leia is a Go-native, hot-reloadable scripting language with AI-native agents.")
if err != nil { print(err.message); return }
print(result.text)`,
		},
		{
			ID:       "ai-structured-output",
			Title:    "Structured Output",
			Section:  "AI Basics",
			Summary:  "Agents can ask for a concrete output shape and receive validated JSON-like data.",
			Runnable: true,
			Requires: "LLM provider",
			Concepts: []string{
				"`output` declares the expected result shape.",
				"The runtime validates model text before returning it as `result.value`.",
				"Hosts can consume the value without reparsing ad-hoc prose.",
			},
			Source: `result, err := llm.turn({
    messages: {
        llm.system("Return raw JSON only. Do not use markdown, prose, or code fences."),
        llm.user("Return exactly this object with no extra keys: {\"product\":\"playground\",\"severity\":\"low\",\"action\":\"improve_demos\"}"),
    }
    response_format: {type: "json_object"}
    max_tokens: 64
    temperature: 0
})
if err != nil { print(err.message); return }
ticket := json.decode(result.text)
print(json.encode(ticket))`,
		},
		{
			ID:       "ai-streaming",
			Title:    "Streaming Tokens",
			Section:  "AI Basics",
			Summary:  "Use a turn callback to update UI state while still receiving the final provider result.",
			Runnable: true,
			Requires: "LLM provider",
			Concepts: []string{
				"`on_stream` receives provider token events as ordinary tables.",
				"Providing `on_stream` automatically requests streaming.",
				"The final `result.text` remains the authoritative complete answer.",
			},
			Source: `streamed := ""
result, err := llm.turn({
    messages: {llm.user("Reply exactly: LEIA_GLM_OK")}
    on_stream: func(event) {
        streamed = streamed .. event.token
    }
})
if err != nil { print(err.message); return }
print("streamed", streamed)
print("final", result.text)`,
		},
		{
			ID:       "ai-tool",
			Title:    "Tool Use With Local Data",
			Section:  "Tools",
			Summary:  "Expose a narrow local function to the model instead of giving it broad host access.",
			Runnable: true,
			Requires: "LLM provider",
			Concepts: []string{
				"`llm.tool` wraps a callable function that an agent can expose to a model.",
				"`//leia:requires` documents required host capabilities.",
				"Agents receive tools as an ordinary list.",
			},
			Source: `lookup := llm.tool("lookup", func(query) {
    docs := {
        memory: "Use an ordered list of llm.system/user messages, then append msg.assistant and msg.user.",
        tools: "Declare tool functions and pass them in an agent tools list.",
        dialect: "Use tagged strings such as json and shell dialects for compact host workflows."
    }
    return docs[query] || "No local doc for " .. query, nil
}, {
    params: {"query"}
    requires: {"docs.read"}
    param_docs: {query: "search query"}
})

answer_with_lookup := llm.agent("answer_with_lookup", func(question) {
    return {
        messages: {
            llm.system("Use lookup with one of: memory, tools, dialect. Reply in two short bullets."),
            llm.user(question),
        }
        tools: {lookup}
    }
}, nil, {params: {"question"}})

result, err := answer_with_lookup("How should I keep multi-turn memory in Leia?")
if err != nil { print(err.message); return }
print(result.text)`,
		},
		{
			ID:       "ai-memory",
			Title:    "Multi-Turn Memory",
			Section:  "Conversation",
			Summary:  "Build history explicitly, then add assistant and user turns as the conversation evolves.",
			Runnable: true,
			Requires: "LLM provider",
			Concepts: []string{
				"`messages` builds an ordered conversation history.",
				"Append assistant and user messages between turns.",
				"Record/replay can later turn this into deterministic regression data.",
			},
			Source: `history := {
    llm.system("Remember facts exactly."),
    llm.user("Store these facts: project=ORCHID, owner=ADA, risk=LOW. Reply MEMORY_STORED."),
}

stored, err := llm.turn({messages: history, max_tokens: 32})
if err != nil { return nil, err }

history[#history + 1] = msg.assistant(stored.text)
history[#history + 1] = msg.user("Recall project, owner, and risk as key=value pairs.")

recalled, err := llm.turn({messages: history, max_tokens: 64})
if err != nil { print(err.message); return }
print(recalled.text)`,
		},
		{
			ID:       "ai-agent-tool",
			Title:    "Agent As Tool",
			Section:  "Tools",
			Summary:  "Delegate focused extraction work to a specialist agent from a supervisor.",
			Runnable: true,
			Requires: "LLM provider",
			Concepts: []string{
				"Specialized agents can become tools.",
				"The supervisor controls when to delegate.",
				"Output shapes make tool results predictable.",
			},
			Source: `extract_memory := llm.agent("extract_memory", func(note) {
    return {
        output: { project: "ORCHID", owner: "ADA", risk: "LOW" }
    }
}, func(note) {
    lower := string.lower(note)
    if string.find(lower, "orchid") == nil || string.find(lower, "ada") == nil {
        return nil, {kind: "validation", message: "memory note must mention ORCHID and ADA"}
    }
    risk := "LOW"
    if string.find(lower, "high") != nil {
        risk = "HIGH"
    }
    return { project: "ORCHID", owner: "ADA", risk: risk }, nil
}, {
    params: {"note"}
    output: {project: "ORCHID", owner: "ADA", risk: "LOW"}
})

supervisor := llm.agent("supervisor", func(question) {
    return {
        messages: {
            llm.system("Call extract_memory before answering. Summarize the extracted fields."),
            llm.user(question),
        }
        tools: {extract_memory}
    }
}, nil, {params: {"question"}})

result, err := supervisor("project is ORCHID, owner is ADA, launch risk is LOW")
if err != nil { print(err.message); return }
print(result.text)`,
		},
		{
			ID:       "ai-support-triage",
			Title:    "Support Triage",
			Section:  "Agent Workflows",
			Summary:  "A support agent combines policy text, a safe lookup tool, and a concise customer-facing answer.",
			Runnable: true,
			Requires: "LLM provider",
			Concepts: []string{
				"Business tools are explicit and capability-scoped.",
				"The agent decides whether the tool is useful for the current turn.",
				"The result is ordinary text that a host can log, review, or replay.",
			},
			Source: `lookup_order := llm.tool("lookup_order", func(id) {
    return { id: id, status: "delivered", total: 42, refundable: true }, nil
}, {
    params: {"id"}
    requires: {"orders.read"}
    param_docs: {id: "order id"}
})

support_triage := llm.agent("support_triage", func(message) {
    return {
        messages: {
            llm.system("You are a concise support assistant. Use lookup_order for order status. Mention whether refund is possible."),
            llm.user(message),
        }
        tools: {lookup_order}
    }
}, nil, {params: {"message"}})

result, err := support_triage("Customer asks: order A100 arrived damaged. Can I get a refund?")
if err != nil { print(err.message); return }
print(result.text)`,
		},
		{
			ID:       "ai-draft-review",
			Title:    "Draft And Review",
			Section:  "Agent Workflows",
			Summary:  "Use one agent to draft an answer and a second turn to review it against a checklist.",
			Runnable: true,
			Requires: "LLM provider",
			Concepts: []string{
				"Agent outputs can feed a later review turn.",
				"History separates system instructions, draft content, and review request.",
				"This pattern is useful for lightweight agent quality gates.",
			},
			Source: `draft_release_note := llm.agent("draft_release_note", func(change) {
    return {
        messages: {
            llm.system("Write a release note in two short bullets."),
            llm.user(change),
        }
    }
}, nil, {params: {"change"}})

draft, err := draft_release_note("Playground now has runnable Tour, Examples, and AI demos.")
if err != nil { print(err.message); return }

review := {
    llm.system("Review the draft. Reply PASS if it is concise and user-facing; otherwise explain one fix."),
    llm.user(draft.text),
}

checked, err := llm.turn({messages: review, max_tokens: 64})
if err != nil { print(err.message); return }
print("draft:")
print(draft.text)
print("review:")
print(checked.text)`,
		},
		{
			ID:       "ai-coding-agent",
			Title:    "Coding Agent With Safe Tools",
			Section:  "Agent Workflows",
			Summary:  "A coding agent gathers context, checks tests, iterates on feedback, and returns a bounded program patch.",
			Runnable: true,
			Requires: "LLM provider",
			Concepts: []string{
				"Multiple tools expose narrow read/search/test capabilities.",
				"A custom `flow` can call the same safe implementation functions that back those tools.",
				"The agent can revise after test feedback and still return a reviewable result.",
			},
			Source: `func read_file_impl(path) {
    if path == "README.md" {
        return "Build a small Leia program. Prefer pure functions and self-checking assertions.", nil
    }
    if path == "src/main.leia" {
        return "func slugify(title) { return title }", nil
    }
    if path == "tests/slugify_cases.txt" {
        return "Hello Leia => hello-leia; AI Native Script => ai-native-script", nil
    }
    return "missing file: " .. path, nil
}

func search_repo_impl(query) {
    if string.find(query, "slug") != nil {
        return {"src/main.leia", "tests/slugify_cases.txt"}, nil
    }
    return {"README.md"}, nil
}

func read_docs_impl(topic) {
    docs := {
        strings: "Use string.lower, string.trim, string.gsub, and string.split for text cleanup.",
        test_doc: "Use assert(...) in executable examples for lightweight regression checks.",
        style: "Keep host-changing operations behind explicit tools."
    }
    return docs[topic] || "No docs for " .. topic, nil
}

func run_tests_impl(patch) {
    lower := string.lower(patch)
    has_function := string.find(lower, "func slugify") != nil
    has_asserts := string.find(lower, "assert") != nil
    has_cases := string.find(lower, "hello%-leia") != nil && string.find(lower, "ai%-native%-script") != nil
    uses_supported_string_api := string.find(lower, "string.lower") != nil && string.find(lower, "string.trim") != nil && string.find(lower, "string.gsub") != nil
    has_leia_binding := string.find(lower, ":=") != nil
    uses_invalid_syntax := string.find(lower, "var ") != nil ||
        string.find(lower, "let ") != nil ||
        string.find(lower, "while ") != nil ||
        string.find(lower, "import ") != nil ||
        string.find(lower, "\n#") != nil ||
        string.find(lower, " null") != nil ||
        string.find(lower, "string.at") != nil ||
        string.find(lower, "string.length") != nil ||
        string.find(lower, "string.substring") != nil
    if has_function && has_asserts && has_cases && uses_supported_string_api && has_leia_binding && !uses_invalid_syntax {
        return { ok: true, output: "2 slugify cases passed" }, nil
    }
    return { ok: false, output: "patch must be raw Leia code only. Use := locals, // comments, nil, func slugify, string.lower/trim/gsub, and assert checks for hello-leia and ai-native-script. Do not use markdown, import, var, let, while, # comments, null, string.at, string.length, or string.substring" }, nil
}

func propose_file_impl(path, body) {
    return { path: path, body: body, mode: "proposal_only" }, nil
}

read_file := llm.tool("read_file", func(path) {
    return read_file_impl(path)
}, {
    params: {"path"}
    requires: {"fs.read"}
    param_docs: {path: "source file path"}
})

search_repo := llm.tool("search_repo", func(query) {
    return search_repo_impl(query)
}, {
    params: {"query"}
    requires: {"repo.search"}
    param_docs: {query: "search query"}
})

read_docs := llm.tool("read_docs", func(topic) {
    return read_docs_impl(topic)
}, {
    params: {"topic"}
    requires: {"docs.read"}
    param_docs: {topic: "language topic"}
})

run_tests := llm.tool("run_tests", func(patch) {
    return run_tests_impl(patch)
}, {
    params: {"patch"}
    requires: {"tests.run"}
    param_docs: {patch: "proposed patch text"}
})

propose_file := llm.tool("propose_file", func(path, body) {
    return propose_file_impl(path, body)
}, {
    params: {"path", "body"}
    requires: {"patch.write"}
    param_docs: {
        path: "target path"
        body: "proposed file content"
    }
})

func coding_agent_config(task) {
    return {
        messages: {
            llm.system("You are a careful coding agent for Leia. Produce a complete Leia patch proposal. Include code, tests, and risk notes. Do not claim to have written files."),
            llm.user(task),
        }
        tools: {search_repo, read_file, read_docs, run_tests, propose_file}
        budget: {turns: 4, calls: 8, tokens: 3200}
    }
}

coding_agent := llm.agent("coding_agent", coding_agent_config, func(task) {
    cfg := coding_agent_config(task)
    system_message := cfg.messages[1].text
    hits, _ := search_repo_impl("slugify implementation and tests")
    readme, _ := read_file_impl("README.md")
    current, _ := read_file_impl("src/main.leia")
    cases, _ := read_file_impl("tests/slugify_cases.txt")
    string_docs, _ := read_docs_impl("strings")
	    test_docs, _ := read_docs_impl("test_doc")

    prompt := task ..
        "\n\nsearch: " .. json.encode(hits) ..
        "\nreadme: " .. readme ..
        "\ncurrent: " .. current ..
        "\ncases: " .. cases ..
	        "\nstring_docs: " .. string_docs ..
	        "\ntest_docs: " .. test_docs ..
	        "\n\nReturn raw JSON only with exactly two string fields: code and risk. code must be raw Leia source, not markdown. Include func slugify(title), use string.lower/string.trim/string.gsub, and include assert checks for the two required cases. Use := locals, nil, and // comments. Do not use import, var, let, while, null, # comments, markdown, string.at, string.length, or string.substring."

    last := nil
    for attempt := 1; attempt <= 3; attempt++ {
        draft, err := llm.turn({
            messages: {
                llm.system(system_message),
                llm.user(prompt),
            }
            tools: {}
            response_format: {type: "json_object"}
            max_tokens: 700
        })
        if err != nil {
            return nil, err
        }
        decoded, patch := pcall(json.decode, draft.text)
        if !decoded || patch == nil || patch.code == nil {
            fence := string.char(96, 96, 96)
            cleaned, _ := string.gsub(draft.text, fence .. "json", "")
            cleaned, _ = string.gsub(cleaned, fence, "")
            decoded, patch = pcall(json.decode, cleaned)
        }
        if !decoded || patch == nil || patch.code == nil {
            patch = {
                code: draft.text,
                risk: "model did not return the requested JSON shape",
            }
        }
        last = patch

        report, test_err := run_tests_impl(patch.code)
        if test_err != nil {
            return nil, test_err
        }
        if report.ok {
            proposal, _ := propose_file_impl("src/main.leia", patch.code)
            return {
                text: patch.code,
                attempts: attempt,
                tests: report.output,
                risk: patch.risk,
                tools: {"search_repo", "read_file", "read_docs", "run_tests", "propose_file"},
                proposal: proposal,
            }, nil
        }

        prompt = prompt .. "\n\nPrevious code:\n" .. patch.code .. "\n\nTest feedback: " .. report.output .. ". Return raw JSON only with fixed code and risk."
    }

    return {
        text: last.code,
        attempts: 3,
        tests: "needs human review after three attempts",
        risk: last.risk,
    }, nil
}, {params: {"task"}})

result, err := coding_agent("Implement slugify(title) for blog URLs and include evaluate regression cases.")
if err != nil { print(err.message); return }
print("attempts", result.attempts)
print("tests", result.tests)
print("risk", result.risk)
print("tools", table.concat(result.tools, ","))
print(result.text)`,
		},
		{
			ID:       "ai-self-check-loop",
			Title:    "Agent Self-Check",
			Section:  "Evaluation",
			Summary:  "Keep local behavior checks near agent helpers with executable assertions.",
			Runnable: true,
			Concepts: []string{
				"Plain Leia code can still test prompt-adjacent helper logic.",
				"`assert` makes examples fail fast in CI and in the playground.",
				"Richer agent evaluation can build on this executable style.",
			},
			Source: `func classify_local(text) {
    if string.find(string.lower(text), "refund") != nil {
        return "refund"
    }
    return "other"
}

assert(classify_local("customer asks for a refund") == "refund")
assert(classify_local("customer says hello") == "other")
print("local classifier checks passed")`,
		},
	}
}

func playgroundRepositoryExamples(root string) ([]playgroundExample, error) {
	var out []playgroundExample
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".leia" {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(root, path)
		if err != nil {
			relPath = path
		}
		rel := "examples/" + filepath.ToSlash(relPath)
		section := "Examples"
		if parts := strings.Split(rel, "/"); len(parts) >= 2 {
			section = strings.ReplaceAll(parts[1], "_", " ")
		}
		id := strings.TrimSuffix(strings.TrimPrefix(rel, "examples/"), ".leia")
		id = strings.ReplaceAll(id, "/", "-")
		title := strings.TrimSuffix(filepath.Base(path), ".leia")
		title = strings.ReplaceAll(title, "_", " ")
		out = append(out, playgroundExample{
			ID:       "repo-" + id,
			Title:    title,
			Section:  strings.Title(section),
			Summary:  rel,
			Source:   string(src),
			Runnable: repositoryExampleRunnable(rel),
			Requires: repositoryExampleRequires(rel),
		})
		return nil
	})
	return out, err
}

func playgroundExamplesRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return "examples"
	}
	for {
		candidate := filepath.Join(wd, "examples")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			return "examples"
		}
		wd = parent
	}
}

func repositoryExampleRunnable(path string) bool {
	return !strings.Contains(path, "/evaluate/") &&
		!strings.Contains(path, "/llm/") &&
		!strings.Contains(path, "/testing/") &&
		!strings.Contains(path, "/workflow/support_triage_replay.leia") &&
		!strings.Contains(path, "/web/") &&
		!strings.Contains(path, "/database/package_managed/") &&
		!strings.Contains(path, "/dialects/shell_filesystem.leia") &&
		!strings.Contains(path, "/macos/package_managed/") &&
		!strings.Contains(path, "/ui/package_managed/") &&
		!strings.Contains(path, "/game_engine/") &&
		!strings.Contains(path, "/concurrency/context_process.leia") &&
		!strings.Contains(path, "/concurrency/goroutine_errors.leia") &&
		!strings.Contains(path, "/data_processing/data_oriented/particle_integration.leia")
}

func repositoryExampleRequires(path string) string {
	switch {
	case strings.Contains(path, "/evaluate/"):
		return "leia evaluate CLI"
	case strings.Contains(path, "/llm/"):
		return "LLM provider"
	case strings.Contains(path, "/testing/"):
		return "leia test CLI"
	case strings.Contains(path, "/workflow/support_triage_replay.leia"):
		return "LLM replay fixture or provider"
	case strings.Contains(path, "/web/"):
		return "network/server host access"
	case strings.Contains(path, "/database/package_managed/"):
		return "package-managed database runtime and native SQL driver"
	case strings.Contains(path, "/dialects/shell_filesystem.leia"):
		return "process shell and filesystem host access"
	case strings.Contains(path, "/macos/package_managed/"):
		return "package-managed macOS automation runtime and process host access"
	case strings.Contains(path, "/ui/package_managed/"):
		return "package-managed UI runtime and native window host access"
	case strings.Contains(path, "/game_engine/"):
		return "game/window host access"
	case strings.Contains(path, "/concurrency/context_process.leia"):
		return "process host access"
	case strings.Contains(path, "/concurrency/goroutine_errors.leia"):
		return "debug event sink host access"
	case strings.Contains(path, "/data_processing/data_oriented/particle_integration.leia"):
		return "higher playground step budget"
	default:
		return ""
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
.tabs {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}
.tab-button {
  height: 32px;
  border-radius: 999px;
  background: transparent;
}
.tab-button.active {
  border-color: var(--accent);
  color: var(--accent);
  background: color-mix(in srgb, var(--accent) 10%, transparent);
}
.meta {
  color: var(--muted);
  font-size: 13px;
}
main {
  display: grid;
  grid-template-columns: 300px minmax(0, 1.25fr) minmax(320px, 0.75fr);
  min-height: calc(100vh - 57px);
}
.tour, .editor, .output {
  min-width: 0;
  padding: 16px;
}
.tour {
  border-right: 1px solid var(--border);
  background: color-mix(in srgb, var(--panel) 72%, var(--bg));
  overflow: auto;
}
.editor {
  border-right: 1px solid var(--border);
}
.tour h2 {
  margin: 0 0 8px;
  font-size: 16px;
}
.tour-note {
  margin: 0 0 14px;
  color: var(--muted);
  font-size: 13px;
}
.lesson-group {
  margin: 16px 0 7px;
  color: var(--muted);
  font-size: 12px;
  font-weight: 700;
  text-transform: uppercase;
}
.lesson-button {
  display: block;
  width: 100%;
  min-height: 36px;
  margin: 0 0 6px;
  text-align: left;
  border-radius: 6px;
  background: transparent;
}
.lesson-button.active {
  border-color: var(--accent);
  color: var(--accent);
  background: color-mix(in srgb, var(--accent) 10%, transparent);
}
.lesson-button small {
  display: block;
  margin-top: 2px;
  color: var(--muted);
  font-size: 11px;
}
.lesson-summary {
  margin: 0 0 12px;
  color: var(--muted);
}
.concepts {
  margin: 0 0 12px;
  padding-left: 18px;
  color: var(--muted);
}
.concepts li {
  margin: 4px 0;
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
button:disabled {
  cursor: not-allowed;
  opacity: 0.55;
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
  .tour, .editor { border-right: 0; border-bottom: 1px solid var(--border); }
  .codewrap { min-height: 420px; }
}
</style>
</head>
<body>
<header>
  <h1>Leia Playground</h1>
  <nav class="tabs" aria-label="Playground sections">
    <button class="tab-button active" data-tab="playground" type="button">Playground</button>
    <button class="tab-button" data-tab="tour" type="button">Tour</button>
    <button class="tab-button" data-tab="examples" type="button">Examples</button>
    <button class="tab-button" data-tab="ai" type="button">AI</button>
  </nav>
  <div class="meta">Backend execution, timeout {{.Timeout}}, max source {{.MaxKB}} KB</div>
</header>
<main>
  <aside class="tour">
    <h2 id="panelTitle">Playground</h2>
    <p class="tour-note" id="panelNote">Write or paste Leia code, then run it on the backend.</p>
    <div id="items"></div>
  </aside>
  <section class="editor">
    <div class="toolbar">
      <select id="examples" aria-label="Examples"></select>
      <select id="mode" aria-label="Execution mode">
        <option value="interpreter">Interpreter</option>
        <option value="bytecode">Bytecode VM</option>
      </select>
      <button class="primary" id="run">Run</button>
    </div>
    <p class="lesson-summary" id="lessonSummary"></p>
    <ul class="concepts" id="concepts"></ul>
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
const items = document.getElementById("items");
const panelTitle = document.getElementById("panelTitle");
const panelNote = document.getElementById("panelNote");
const lessonSummary = document.getElementById("lessonSummary");
const concepts = document.getElementById("concepts");
const mode = document.getElementById("mode");
const runButton = document.getElementById("run");
const tabButtons = document.querySelectorAll(".tab-button");
let activeItemID = "";
let activeTab = "playground";
let currentItems = [];

const leiaKeywords = new Set([
  "and", "break", "case", "const", "continue", "default",
  "defer", "do", "else", "elseif", "end", "false", "for",
  "func", "go", "if", "import", "in", "local", "nil",
  "not", "or", "return", "select", "then", "true"
]);
const leiaBuiltins = new Set([
  "print", "pairs", "ipairs", "range", "len", "type", "tostring", "tonumber",
  "error", "pcall", "xpcall", "require", "assert", "make", "close", "sleep"
]);
const leiaAgentFields = new Set([
  "model", "tools", "system", "user", "output", "example", "history", "messages",
  "temperature", "max_tokens", "stream", "on_stream", "response_format"
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
    if ((ch === "-" && next === "-") || (ch === "/" && next === "/")) {
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

const tabConfig = {
  playground: {
    title: "Playground",
    note: "A blank backend-powered editor. This tab does not load any preset code.",
    emptySource: "// Write Leia code here.\nprint(\"hello\")"
  },
  tour: {
    title: "Tour of Leia",
    note: "A compact tour of the main language topics. Each lesson is runnable.",
    url: "/api/tour"
  },
  examples: {
    title: "Repository Examples",
    note: "Examples loaded from the repository. Some require host capabilities or provider setup.",
    url: "/api/examples"
  },
  ai: {
    title: "AI-Native Leia",
    note: "From a one-line turn to tool-using and coding-agent shapes.",
    url: "/api/ai"
  }
};

async function loadTab(tab) {
  activeTab = tab;
  for (const button of tabButtons) {
    button.classList.toggle("active", button.dataset.tab === tab);
  }
  const cfg = tabConfig[tab];
  panelTitle.textContent = cfg.title;
  panelNote.textContent = cfg.note;
  items.innerHTML = "";
  examples.innerHTML = "";
  currentItems = [];
  concepts.innerHTML = "";
  statusLine.textContent = "Ready.";
  statusLine.className = "status";

  if (tab === "playground") {
    lessonSummary.textContent = "Scratch code runs in a sandboxed backend process.";
    source.value = cfg.emptySource;
    runButton.disabled = false;
    refreshHighlight();
    return;
  }

  const res = await fetch(cfg.url);
  const data = await res.json();
  currentItems = data;
  let currentSection = "";
  for (const item of data) {
    if (item.section !== currentSection) {
      currentSection = item.section;
      const group = document.createElement("div");
      group.className = "lesson-group";
      group.textContent = currentSection;
      items.appendChild(group);
    }
    const button = document.createElement("button");
    button.type = "button";
    button.className = "lesson-button";
    button.dataset.id = item.id;
    button.textContent = item.title;
    if (!item.runnable && item.requires) {
      const small = document.createElement("small");
      small.textContent = "Requires: " + item.requires;
      button.appendChild(small);
    }
    button.addEventListener("click", () => setActiveItem(item.id));
    items.appendChild(button);

    const opt = document.createElement("option");
    opt.value = item.id;
    opt.textContent = item.section + " / " + item.title;
    examples.appendChild(opt);
  }
  if (data.length) {
    setActiveItem(data[0].id);
  } else {
    lessonSummary.textContent = "No examples found.";
    source.value = "";
    runButton.disabled = true;
    refreshHighlight();
  }
}

examples.addEventListener("change", () => {
  setActiveItem(examples.value);
});

function setActiveItem(id) {
  const item = currentItems.find(x => x.id === id);
  if (item) {
    activeItemID = item.id;
    examples.value = item.id;
    for (const button of items.querySelectorAll(".lesson-button")) {
      button.classList.toggle("active", button.dataset.id === item.id);
    }
    lessonSummary.textContent = item.summary || "";
    concepts.innerHTML = "";
    if (!item.runnable && item.requires) {
      const li = document.createElement("li");
      li.textContent = "Requires: " + item.requires + ". This example is shown for syntax and design review.";
      concepts.appendChild(li);
    }
    for (const concept of item.concepts || []) {
      const li = document.createElement("li");
      li.innerHTML = escapeHTML(concept).replace(new RegExp("\\x60([^\\x60]+)\\x60", "g"), "<code>$1</code>");
      concepts.appendChild(li);
    }
    source.value = item.source;
    runButton.disabled = !item.runnable;
    refreshHighlight();
  }
}

async function run() {
  if (runButton.disabled) return;
  statusLine.textContent = "Running...";
  statusLine.className = "status";
  output.textContent = "";
  try {
    const res = await fetch("/api/run", {
      method: "POST",
      headers: {"Content-Type": "application/json"},
      body: JSON.stringify({source: source.value, mode: mode.value, profile: activeTab === "ai" ? "ai" : "sandbox"})
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

runButton.addEventListener("click", run);
for (const button of tabButtons) {
  button.addEventListener("click", () => loadTab(button.dataset.tab));
}
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
loadTab("playground");
</script>
</body>
</html>`))
