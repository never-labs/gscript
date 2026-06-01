package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	leia "github.com/never-labs/leia"
	"github.com/never-labs/leia/internal/lexer"
	"github.com/never-labs/leia/internal/parser"
	"github.com/never-labs/leia/internal/runtime"
	bytecodevm "github.com/never-labs/leia/internal/vm"
	"github.com/never-labs/leia/llm"
	"github.com/never-labs/leia/llm/anthropic"
	"github.com/never-labs/leia/llm/openai"
)

func processExit(err error) (*runtime.ProcessExitError, bool) {
	var exit *runtime.ProcessExitError
	if errors.As(err, &exit) {
		return exit, true
	}
	return nil, false
}

func processExitCode(err error) (int, bool) {
	if exit, ok := processExit(err); ok {
		return exit.Code, true
	}
	var leiaExit *leia.ExitError
	if errors.As(err, &leiaExit) {
		return leiaExit.Code, true
	}
	return 0, false
}

func runScriptFile(interp *runtime.Interpreter, filename string, args []string, opts cliRunOptions) error {
	installCLILLMProviderFactory(interp)
	interp.SetArgs(filename, args)

	// Set script directory for require.
	absPath, _ := filepath.Abs(filename)
	interp.SetScriptDir(filepath.Dir(absPath))

	if opts.UseVM {
		return runFileVM(interp, filename, opts.UseJIT, opts.ShowJITStats, opts.JIT)
	}
	return runFile(interp, filename)
}

func installCLILLMProviderFactory(interp *runtime.Interpreter) {
	interp.SetLLMProviderFactory(cliDefaultLLMProviderFactory)
}

func cliDefaultLLMProviderFactory(cfg runtime.LLMProviderConfig) (runtime.LLMProvider, error) {
	protocol := strings.ToLower(strings.ReplaceAll(cfg.Protocol, "_", "-"))
	switch protocol {
	case "openai", "openai-compatible", "openai-compat", "chat-completions":
		return cliLLMProviderAdapter{provider: openai.Provider{
			Endpoint: cliOpenAIChatCompletionsEndpoint(cfg.BaseURL),
			APIKey:   cfg.APIKey,
			Model:    cfg.ProviderModel,
		}}, nil
	case "anthropic", "anthropic-compatible", "anthropic-compat", "messages":
		return cliLLMProviderAdapter{provider: anthropic.Provider{
			Endpoint: cfg.BaseURL,
			APIKey:   cfg.APIKey,
			Model:    cfg.ProviderModel,
		}}, nil
	default:
		if cfg.Protocol == "" {
			return nil, fmt.Errorf("llm provider protocol not configured for model %q", cfg.Name)
		}
		return nil, fmt.Errorf("unsupported llm provider protocol %q for model %q", cfg.Protocol, cfg.Name)
	}
}

type cliLLMProviderAdapter struct {
	provider llm.Provider
}

func (a cliLLMProviderAdapter) Turn(ctx context.Context, req runtime.LLMTurnRequest) (runtime.LLMTurnResult, error) {
	res, err := a.provider.Turn(ctx, cliPublicLLMTurnRequest(req))
	return cliRuntimeLLMTurnResult(res), err
}

func cliPublicLLMTurnRequest(req runtime.LLMTurnRequest) llm.TurnRequest {
	out := llm.TurnRequest{
		Model:          req.Model,
		ForceTool:      req.ForceTool,
		MaxTokens:      req.MaxTokens,
		Temperature:    cloneFloat64Ptr(req.Temperature),
		TopP:           cloneFloat64Ptr(req.TopP),
		ResponseFormat: req.ResponseFormat,
		Stream:         req.Stream,
		Stop:           append([]string(nil), req.Stop...),
		Metadata:       cloneStringMap(req.Metadata),
	}
	if len(req.Messages) > 0 {
		out.Messages = make([]llm.Message, len(req.Messages))
		for i := range req.Messages {
			out.Messages[i] = cliPublicLLMMessage(req.Messages[i])
		}
	}
	if len(req.Tools) > 0 {
		out.Tools = make([]llm.Tool, len(req.Tools))
		for i := range req.Tools {
			out.Tools[i] = cliPublicLLMTool(req.Tools[i])
		}
	}
	return out
}

func cliPublicLLMMessage(msg runtime.LLMMessage) llm.Message {
	out := llm.Message{
		Role:      msg.Role,
		Text:      msg.Text,
		ToolUseID: msg.ToolUseID,
		Value:     msg.Value,
		Error:     msg.Error,
	}
	if msg.ToolCall != nil {
		call := cliPublicLLMToolCall(*msg.ToolCall)
		out.ToolCall = &call
	}
	return out
}

func cliPublicLLMTool(tool runtime.LLMTool) llm.Tool {
	return llm.Tool{
		Name:        tool.Name,
		Description: tool.Description,
		Params:      append([]string(nil), tool.Params...),
		Requires:    append([]string(nil), tool.Requires...),
		Schema:      tool.Schema,
	}
}

func cliPublicLLMToolCall(call runtime.LLMToolCall) llm.ToolCall {
	return llm.ToolCall{
		ID:   call.ID,
		Tool: call.Tool,
		Args: cloneAnyMap(call.Args),
	}
}

func cliRuntimeLLMTurnResult(res llm.TurnResult) runtime.LLMTurnResult {
	out := runtime.LLMTurnResult{
		Status: res.Status,
		Text:   res.Text,
		Reason: res.Reason,
		Usage: runtime.LLMTurnUsage{
			InputTokens:  res.Usage.InputTokens,
			OutputTokens: res.Usage.OutputTokens,
			Cost:         res.Usage.Cost,
			LatencyMS:    res.Usage.LatencyMS,
		},
	}
	if len(res.Calls) > 0 {
		out.Calls = make([]runtime.LLMToolCall, len(res.Calls))
		for i := range res.Calls {
			out.Calls[i] = runtime.LLMToolCall{
				ID:   res.Calls[i].ID,
				Tool: res.Calls[i].Tool,
				Args: cloneAnyMap(res.Calls[i].Args),
			}
		}
	}
	return out
}

func cloneStringMap(src map[string]string) map[string]string {
	if src == nil {
		return nil
	}
	out := make(map[string]string, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

func cloneAnyMap(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}
	out := make(map[string]any, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

func cloneFloat64Ptr(src *float64) *float64 {
	if src == nil {
		return nil
	}
	v := *src
	return &v
}

func cliOpenAIChatCompletionsEndpoint(base string) string {
	base = strings.TrimSpace(base)
	if base == "" {
		return "https://api.openai.com/v1/chat/completions"
	}
	trimmed := strings.TrimRight(base, "/")
	if strings.HasSuffix(trimmed, "/chat/completions") {
		return trimmed
	}
	return trimmed + "/chat/completions"
}

func runFile(interp *runtime.Interpreter, filename string) error {
	src, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	return runString(interp, string(src))
}

func runString(interp *runtime.Interpreter, src string) error {
	tokens, err := lexer.New(src).Tokenize()
	if err != nil {
		return fmt.Errorf("lexer error: %w", err)
	}

	prog, err := parser.New(tokens).Parse()
	if err != nil {
		return fmt.Errorf("parse error: %w", err)
	}

	return interp.Exec(prog)
}

func runFileVM(interp *runtime.Interpreter, filename string, jit bool, showJITStats bool, jitOpts jitCLIOptions) error {
	src, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	return runStringVMWithSource(interp, string(src), filename, jit, showJITStats, jitOpts)
}

func runStringVM(interp *runtime.Interpreter, src string, jit bool, showJITStats bool, jitOpts jitCLIOptions) error {
	return runStringVMWithSource(interp, src, "", jit, showJITStats, jitOpts)
}

func runStringVMWithSource(interp *runtime.Interpreter, src, sourceName string, jit bool, showJITStats bool, jitOpts jitCLIOptions) error {
	tokens, err := lexer.New(src).Tokenize()
	if err != nil {
		return fmt.Errorf("lexer error: %w", err)
	}
	prog, err := parser.New(tokens).Parse()
	if err != nil {
		return fmt.Errorf("parse error: %w", err)
	}
	proto, err := bytecodevm.Compile(prog)
	if err != nil {
		return fmt.Errorf("compile error: %w", err)
	}
	setProtoSource(proto, sourceName)
	globals := interp.ExportGlobals()
	bvm := bytecodevm.New(globals)
	bvm.SetScriptDir(interp.ScriptDir())
	bvm.SetLLMProviderFactory(cliDefaultLLMProviderFactory)
	if jitOpts.ShowCoroutineStats {
		bvm.EnableCoroutineStats()
	}
	var pathStats *runtime.RuntimePathStats
	if jitOpts.ShowPathStats || jitOpts.ShowPathStatsJSON {
		pathStats = runtime.EnableRuntimePathStats()
		defer runtime.DisableRuntimePathStats()
	}
	var reporter jitStatsReporter
	if !jit && jitOpts.TimelinePath != "" {
		return fmt.Errorf("JIT timeline requires JIT to be enabled")
	}
	if !jit && jitOpts.WarmDumpDir != "" {
		return fmt.Errorf("-jit-dump-warm requires -jit")
	}
	var warmDumper jitWarmDumpController
	if jit {
		reporter, err = cliEnableJIT(bvm, jitOpts)
		if err != nil {
			return err
		}
		if reporter != nil {
			warmDumper, _ = reporter.(jitWarmDumpController)
		}
	}
	if jitOpts.WarmDumpDir != "" {
		if warmDumper == nil {
			return fmt.Errorf("-jit-dump-warm requires the darwin/arm64 method JIT")
		}
		if err := warmDumper.EnableWarmDump(jitOpts.WarmDumpDir, jitOpts.WarmDumpProto); err != nil {
			return err
		}
	}
	_, err = bvm.Execute(proto)
	var dumpErr error
	if jitOpts.WarmDumpDir != "" && warmDumper != nil {
		dumpErr = warmDumper.WriteWarmDump(proto)
	}
	var closeErr error
	if reporter != nil {
		closeErr = reporter.Close()
	}
	if showJITStats {
		if reporter != nil {
			reporter.PrintStats(os.Stderr)
		} else {
			fmt.Fprintln(os.Stderr, "JIT Statistics: JIT disabled or unavailable on this platform")
		}
	}
	var statsErr error
	if jitOpts.ShowExitStats {
		if reporter != nil {
			reporter.PrintExitStats(os.Stderr)
		} else {
			fmt.Fprintln(os.Stderr, "Tier 2 Exit Profile: JIT disabled or unavailable on this platform")
		}
	}
	if jitOpts.ShowExitStatsJSON {
		if reporter != nil {
			statsErr = reporter.PrintExitStatsJSON(os.Stderr)
		} else {
			fmt.Fprintln(os.Stderr, `{"error":"JIT disabled or unavailable on this platform"}`)
		}
	}
	if jitOpts.ShowTier2PerfStats {
		if reporter != nil {
			reporter.PrintTier2PerfStats(os.Stderr)
		} else {
			fmt.Fprintln(os.Stderr, "Tier 2 Performance Diagnostics: JIT disabled or unavailable on this platform")
		}
	}
	if jitOpts.ShowTier2PerfStatsJSON {
		if reporter != nil {
			statsErr = reporter.PrintTier2PerfStatsJSON(os.Stderr)
		} else {
			fmt.Fprintln(os.Stderr, `{"error":"JIT disabled or unavailable on this platform"}`)
		}
	}
	if jitOpts.ShowTier2SpecStateJSON {
		if reporter != nil {
			statsErr = reporter.PrintTier2SpeculationStateJSON(os.Stderr)
		} else {
			fmt.Fprintln(os.Stderr, `{"error":"JIT disabled or unavailable on this platform"}`)
		}
	}
	if jitOpts.ShowTier2SpecWorklistJSON {
		if reporter != nil {
			statsErr = reporter.PrintTier2SpeculationWorklistJSON(os.Stderr)
		} else {
			fmt.Fprintln(os.Stderr, `{"error":"JIT disabled or unavailable on this platform"}`)
		}
	}
	if jitOpts.ShowCoroutineStats {
		printCoroutineStats(os.Stderr, bvm.CoroutineStats())
	}
	if jitOpts.ShowPathStats && pathStats != nil {
		pathStats.WriteText(os.Stderr)
	}
	if jitOpts.ShowPathStatsJSON && pathStats != nil {
		if err := pathStats.WriteJSON(os.Stderr); err != nil && statsErr == nil {
			statsErr = err
		}
	}
	if err != nil {
		if dumpErr != nil {
			return fmt.Errorf("%w; warm dump failed: %v", err, dumpErr)
		}
		if closeErr != nil {
			return fmt.Errorf("%w; JIT close failed: %v", err, closeErr)
		}
		if statsErr != nil {
			return fmt.Errorf("%w; exit stats failed: %v", err, statsErr)
		}
		return err
	}
	if dumpErr != nil {
		return dumpErr
	}
	if closeErr != nil {
		return closeErr
	}
	if statsErr != nil {
		return statsErr
	}
	return nil
}

func setProtoSource(proto *bytecodevm.FuncProto, sourceName string) {
	if proto == nil || sourceName == "" {
		return
	}
	proto.Source = sourceName
	for _, sub := range proto.Protos {
		setProtoSource(sub, sourceName)
	}
}

func printCoroutineStats(w *os.File, stats bytecodevm.CoroutineStatsSnapshot) {
	fmt.Fprintln(w, "Coroutine Statistics:")
	fmt.Fprintf(w, "  created: %d\n", stats.Created)
	fmt.Fprintf(w, "  wrapped: %d\n", stats.Wrapped)
	fmt.Fprintf(w, "  resumes: %d\n", stats.ResumeCalls)
	fmt.Fprintf(w, "  yields: %d\n", stats.YieldCalls)
	fmt.Fprintf(w, "  leaf fast path: %d\n", stats.LeafFastPath)
	fmt.Fprintf(w, "  leaf fallbacks: %d\n", stats.LeafFallbacks)
	fmt.Fprintf(w, "  wrapped generator fast path: %d\n", stats.WrappedGenerator)
	fmt.Fprintf(w, "  goroutine starts: %d\n", stats.GoroutineStarts)
	fmt.Fprintf(w, "  jit continuations: %d\n", stats.JITContinuations)
	fmt.Fprintf(w, "  jit native resumes: %d\n", stats.JITNativeResumes)
	fmt.Fprintf(w, "  jit native yields: %d\n", stats.JITNativeYields)
	fmt.Fprintf(w, "  jit native misses: %d\n", stats.JITNativeMisses)
	fmt.Fprintf(w, "  completed: %d\n", stats.Completed)
	fmt.Fprintf(w, "  resume errors: %d\n", stats.ResumeErrors)
}
