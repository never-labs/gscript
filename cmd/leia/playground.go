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
	case "ai", "llm":
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
	if provider, ok := playgroundGLMProviderFromEnv(); ok {
		opts = append(opts, leia.WithLLMProvider(provider))
	}
	return opts
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
			Title:   "Welcome",
			Section: "Basics",
			Summary: "Start with print, local bindings, strings, and arithmetic.",
			Concepts: []string{
				"`:=` creates a local binding.",
				"`print` writes tab-separated values to stdout.",
				"Numbers and strings are ordinary first-class values.",
			},
			Source: `print("hello from Leia")

name := "playground"
year := 2026
print("running", name, year)
print("score", 40 + 2)`,
			Runnable: true,
		},
		{
			ID:      "control-flow",
			Title:   "Control Flow",
			Section: "Basics",
			Summary: "Use if statements and loops for predictable imperative code.",
			Concepts: []string{
				"`if` conditions use blocks with braces.",
				"`for` can be a counted loop.",
				"`continue` and `break` work inside loops.",
			},
			Source: `total := 0

for i := 1; i <= 10; i = i + 1 {
    if i % 2 == 1 {
        continue
    }
    total = total + i
}

if total > 20 {
    print("large", total)
} else {
    print("small", total)
}`,
			Runnable: true,
		},
		{
			ID:      "functions",
			Title:   "Functions",
			Section: "Functions",
			Summary: "Functions are values, can call other functions, and may return multiple values.",
			Concepts: []string{
				"`func name(args) { ... }` declares a function.",
				"Multiple return values are adjusted at assignment sites.",
				"Recursion works in the interpreter and bytecode VM.",
			},
			Source: `func divmod(a, b) {
    return a / b, a % b
}

func fib(n) {
    if n < 2 {
        return n
    }
    return fib(n - 1) + fib(n - 2)
}

q, r := divmod(17, 5)
print("divmod", q, r)
print("fib", fib(10))`,
			Runnable: true,
		},
		{
			ID:      "tables",
			Title:   "Tables",
			Section: "Data",
			Summary: "Tables provide array-like and map-like data in one dynamic structure.",
			Concepts: []string{
				"Array indexes are one-based.",
				"Named fields and computed fields share the same table value.",
				"`range` iterates key/value pairs.",
			},
			Source: `scores := { ada: 98, lin: 91, ken: 87 }
scores["grace"] = 95

total := 0
count := 0
for name, score := range scores {
    print(name, score)
    total = total + score
    count = count + 1
}
print("avg", total / count)`,
			Runnable: true,
		},
		{
			ID:      "strings-stdlib",
			Title:   "Strings And Stdlib",
			Section: "Standard Library",
			Summary: "The sandboxed playground exposes safe standard-library modules such as string, math, json, and sort.",
			Concepts: []string{
				"Standard libraries are available as global module tables.",
				"Safe modules work without host filesystem or network access.",
				"JSON and string helpers are useful for scripts and agent data.",
			},
			Source: `text := "Leia Playground"
print(string.lower(text))
print(string.sub(text, 1, 4))

payload := json.encode({ language: "Leia", ok: true, count: 3 })
print(payload)

print("sqrt", math.sqrt(81))`,
			Runnable: true,
		},
		{
			ID:      "errors",
			Title:   "Errors",
			Section: "Robustness",
			Summary: "Use protected calls when a script should recover from a runtime error.",
			Concepts: []string{
				"`error` raises a runtime error.",
				"`pcall` returns a status plus values instead of aborting.",
				"Recovered errors can be printed, inspected, or converted into fallback values.",
			},
			Source: `func risky(n) {
    if n == 0 {
        error("division by zero")
    }
    return 10 / n
}

ok, value := pcall(risky, 2)
print(ok, value)

ok, value = pcall(risky, 0)
print(ok, value)`,
			Runnable: true,
		},
		{
			ID:      "channels",
			Title:   "Channels",
			Section: "Concurrency",
			Summary: "Leia borrows Go-style channels for simple message passing.",
			Concepts: []string{
				"`make(chan, n)` creates a buffered channel.",
				"`ch <- value` sends; `<-ch` receives.",
				"Receives currently return value and ok, matching Go-style checked receive.",
			},
			Source: `ch := make(chan, 2)
ch <- "alpha"
ch <- "beta"

print(<-ch)
print(<-ch)`,
			Runnable: true,
		},
		{
			ID:      "data-oriented",
			Title:   "Data-Oriented Arrays",
			Section: "Data",
			Summary: "Dense arrays are the foundation for data-oriented workloads and later JIT optimizations.",
			Concepts: []string{
				"`array.i64({...})` converts a table into a dense integer array.",
				"Loops over dense arrays keep hot numeric code straightforward.",
				"This is the user-facing shape that SoA and matrix work build on.",
			},
			Source: `xs := array.i64({ 1, 4, 9, 16, 25 })
print("sum", array.sum(xs))`,
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
				"`turn` sends one request to the selected default model.",
				"The result carries text, usage, history, and tool-call metadata.",
				"This example is intentionally one line; configure a model provider before running.",
			},
			Source: `result, err := turn { user: "Reply exactly: LEIA_GLM_OK" }
if err != nil { print(err.message); return }
print(result.text)`,
		},
		{
			ID:       "ai-agent-shape",
			Title:    "Agent Shape",
			Section:  "AI Basics",
			Summary:  "Agents package prompt, input, model choice, and output shape as a callable value.",
			Runnable: true,
			Concepts: []string{
				"`agent` declarations compile in the playground.",
				"Actual model calls require a provider configured by the embedding host.",
				"Use this shape for prompt and output design before wiring live credentials.",
			},
			Source: `agent summarize(topic) {
    system: "Summarize in one sentence."
    user: topic
    output: { summary: "short answer" }
}

print("agent declared")
print("live turns need an LLM provider")`,
		},
		{
			ID:       "ai-models",
			Title:    "Named Models",
			Section:  "Configuration",
			Summary:  "Model blocks let scripts name provider configurations without a separate config file.",
			Runnable: true,
			Requires: "environment-backed model credentials",
			Concepts: []string{
				"`models` defines project-local model aliases.",
				"Environment variables keep API keys out of source files.",
				"Agents can select a model by its alias.",
			},
			Source: `models {
    default: "glm"
    glm: {
        provider: "glm"
        protocol: "anthropic_compatible"
        base_url: __leia_playground_env_first("LEIA_GLM_BASE_URL", "ANTHROPIC_BASE_URL", "", "https://open.bigmodel.cn/api/anthropic")
        api_key: __leia_playground_env_first("LEIA_GLM_API_KEY", "SENTINEL_GLM_API_KEY", "GLM_API_KEY", "")
        provider_model: __leia_playground_env_first("LEIA_GLM_MODEL", "GLM_MODEL", "ANTHROPIC_MODEL", "GLM-5.1")
    }
}

result, err := turn {
    model: "glm"
    user: "Reply exactly: MODEL_ALIAS_OK"
    max_tokens: 16
    temperature: 0
}
if err != nil { print(err.message); return }
print(result.text)`,
		},
		{
			ID:       "ai-tool",
			Title:    "Tool Use",
			Section:  "Tools",
			Summary:  "Tools are ordinary Leia functions with descriptions and capability annotations.",
			Runnable: true,
			Concepts: []string{
				"`tool` declares a callable function that an agent can expose to a model.",
				"`//leia:requires` documents required host capabilities.",
				"Tool bodies can be tested without a live model.",
			},
			Source: `//leia:requires docs.read
//leia:param query search query
tool lookup(query) {
    return "doc:" .. query, nil
}

print("tool lookup declared")
print("agents can expose lookup to a model")`,
		},
		{
			ID:       "ai-memory",
			Title:    "Multi-Turn Memory",
			Section:  "Conversation",
			Summary:  "Separate history construction from each new user input for multi-turn agents.",
			Runnable: true,
			Requires: "LLM provider",
			Concepts: []string{
				"`messages` builds an ordered conversation history.",
				"Append assistant and user messages between turns.",
				"Record/replay can later turn this into deterministic regression data.",
			},
			Source: `history := messages {
    system: "Remember facts exactly."
    user: "Store: project=ORCHID, owner=ADA. Reply MEMORY_STORED."
}

stored, err := turn { messages: history, max_tokens: 32 }
if err != nil { return nil, err }

history[#history + 1] = msg.assistant(stored.text)
history[#history + 1] = msg.user("Recall the project and owner.")

recalled, err := turn { messages: history, max_tokens: 64 }
print(recalled.text)`,
		},
		{
			ID:       "ai-agent-tool",
			Title:    "Agent As Tool",
			Section:  "Tools",
			Summary:  "An agent can be exposed as a tool to a supervisor agent for delegation.",
			Runnable: true,
			Requires: "LLM provider",
			Concepts: []string{
				"Specialized agents can become tools.",
				"The supervisor controls when to delegate.",
				"Output shapes make tool results predictable.",
			},
			Source: `agent extract_memory(note) {
    system: "Extract project and owner. Return compact JSON."
    user: note
    output: { project: "ORCHID", owner: "ADA" }
}

agent supervisor(question) {
    system: "Call extract_memory before answering."
    user: question
    tools: [extract_memory]
}

result, err := supervisor("project is ORCHID and owner is ADA")
if err != nil { print(err.message); return }
print(result.text)`,
		},
		{
			ID:       "ai-coding-agent",
			Title:    "Coding Agent Shape",
			Section:  "Agent Workflows",
			Summary:  "A coding agent combines read-only tools, patch proposal, trace recording, and a strict output contract.",
			Runnable: true,
			Concepts: []string{
				"Keep dangerous host actions behind explicit tools and capabilities.",
				"Return a structured patch plan instead of mutating files implicitly.",
				"Use evaluate blocks later to regression-test prompts and tool choices.",
			},
			Source: `//leia:requires fs.read
//leia:param path source file path
tool read_file(path) {
    return "file:" .. path .. ": contents omitted in demo", nil
}

agent coding_agent(task) {
    system: "Inspect files, then propose a small patch plan. Do not mutate files."
    user: task
    tools: [read_file]
    output: {
        summary: "what changed"
        files: ["path"]
        risk: "low"
    }
}

print("coding agent declared")
print("wire read_file to a safe host binding before live execution")`,
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
	return !strings.Contains(path, "/llm/") &&
		!strings.Contains(path, "/web/") &&
		!strings.Contains(path, "/game_engine/")
}

func repositoryExampleRequires(path string) string {
	switch {
	case strings.Contains(path, "/llm/"):
		return "LLM provider"
	case strings.Contains(path, "/web/"):
		return "network/server host access"
	case strings.Contains(path, "/game_engine/"):
		return "game/window host access"
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
