package evaluate

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/never-labs/leia/internal/runtime"
)

type evalCollector struct {
	call    runtime.ScriptFunctionCaller
	baseDir string
	llm     func() *LLMCaseRun
	llmTurn runtime.Value

	mu          sync.Mutex
	metrics     []Metric
	subcases    []Subcase
	activeStack []int
}

func newEvalCollector(call runtime.ScriptFunctionCaller, baseDir string, llm func() *LLMCaseRun, llmTurn runtime.Value) *evalCollector {
	return &evalCollector{call: call, baseDir: baseDir, llm: llm, llmTurn: llmTurn}
}

func (c *evalCollector) BuildModule() *runtime.Table {
	t := runtime.NewTable()
	t.RawSetString("metric", runtime.FunctionValue(&runtime.GoFunction{
		Name: "eval.metric",
		Fn:   c.metric,
	}))
	t.RawSetString("case", runtime.FunctionValue(&runtime.GoFunction{
		Name: "eval.case",
		Fn:   c.caseFn,
	}))
	t.RawSetString("load_jsonl", runtime.FunctionValue(&runtime.GoFunction{
		Name: "eval.load_jsonl",
		Fn:   c.loadJSONL,
	}))
	t.RawSetString("skip_if", runtime.FunctionValue(&runtime.GoFunction{
		Name: "eval.skip_if",
		Fn:   c.skipIf,
	}))
	t.RawSetString("fail_if", runtime.FunctionValue(&runtime.GoFunction{
		Name: "eval.fail_if",
		Fn:   c.failIf,
	}))
	t.RawSetString("usage", runtime.FunctionValue(&runtime.GoFunction{
		Name: "eval.usage",
		Fn:   c.usage,
	}))
	t.RawSetString("budget", runtime.FunctionValue(&runtime.GoFunction{
		Name: "eval.budget",
		Fn:   c.budget,
	}))
	t.RawSetString("judge", runtime.FunctionValue(&runtime.GoFunction{
		Name: "eval.judge",
		Fn:   c.judge,
	}))
	return t
}

func (c *evalCollector) Data() caseEvalData {
	c.mu.Lock()
	defer c.mu.Unlock()
	return caseEvalData{
		Metrics:  append([]Metric(nil), c.metrics...),
		Subcases: cloneSubcases(c.subcases),
	}
}

func (c *evalCollector) metric(args []runtime.Value) ([]runtime.Value, error) {
	if len(args) < 2 || !args[0].IsString() {
		return nil, fmt.Errorf("bad argument to 'eval.metric' (name, value expected)")
	}
	metric := metricFromValue(args[0].Str(), args[1])
	c.recordMetric(metric)
	return []runtime.Value{runtime.BoolValue(true)}, nil
}

func (c *evalCollector) caseFn(args []runtime.Value) ([]runtime.Value, error) {
	if len(args) < 2 || !args[0].IsString() || !args[1].IsFunction() {
		return nil, fmt.Errorf("bad argument to 'eval.case' (id, function expected)")
	}
	id := args[0].Str()
	start := time.Now()
	idx := c.startSubcase(id, start)
	_, err := c.call(args[1], nil)
	c.finishSubcase(idx, start, err)
	if err != nil {
		return []runtime.Value{runtime.BoolValue(false), runtime.StringValue(err.Error())}, nil
	}
	return []runtime.Value{runtime.BoolValue(true), runtime.NilValue()}, nil
}

func (c *evalCollector) loadJSONL(args []runtime.Value) ([]runtime.Value, error) {
	if len(args) < 1 || !args[0].IsString() {
		return nil, fmt.Errorf("bad argument to 'eval.load_jsonl' (path expected)")
	}
	path := c.resolvePath(args[0].Str())
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("eval.load_jsonl: %v", err)
	}
	defer f.Close()
	out := runtime.NewTable()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 4096), 1024*1024)
	index := int64(1)
	for line := 1; scanner.Scan(); line++ {
		text := scanner.Text()
		if strings.TrimSpace(text) == "" {
			continue
		}
		var decoded any
		dec := json.NewDecoder(strings.NewReader(text))
		dec.UseNumber()
		if err := dec.Decode(&decoded); err != nil {
			return nil, fmt.Errorf("eval.load_jsonl: %s:%d: %v", path, line, err)
		}
		out.RawSet(runtime.IntValue(index), runtime.JSONGoToValue(normalizeJSONNumbers(decoded)))
		index++
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("eval.load_jsonl: %v", err)
	}
	return []runtime.Value{runtime.TableValue(out)}, nil
}

func (c *evalCollector) skipIf(args []runtime.Value) ([]runtime.Value, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("bad argument to 'eval.skip_if' (condition expected)")
	}
	if !args[0].Truthy() {
		return []runtime.Value{runtime.BoolValue(false)}, nil
	}
	reason := "skipped"
	if len(args) >= 2 && args[1].IsString() {
		reason = args[1].Str()
	}
	c.markActiveSubcaseSkipped(reason)
	return []runtime.Value{runtime.BoolValue(true)}, nil
}

func (c *evalCollector) failIf(args []runtime.Value) ([]runtime.Value, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("bad argument to 'eval.fail_if' (condition expected)")
	}
	if !args[0].Truthy() {
		return []runtime.Value{runtime.BoolValue(false)}, nil
	}
	message := "eval.fail_if condition failed"
	if len(args) >= 2 && args[1].IsString() {
		message = args[1].Str()
	}
	return nil, errors.New(message)
}

func (c *evalCollector) usage(args []runtime.Value) ([]runtime.Value, error) {
	return []runtime.Value{runtime.TableValue(llmCaseUsageTable(c.currentLLM()))}, nil
}

func (c *evalCollector) budget(args []runtime.Value) ([]runtime.Value, error) {
	if len(args) < 1 || !args[0].IsTable() {
		return nil, fmt.Errorf("bad argument to 'eval.budget' (budget table expected)")
	}
	usage := c.currentLLM()
	if err := checkEvalBudget(args[0].Table(), usage); err != nil {
		return nil, err
	}
	return []runtime.Value{runtime.BoolValue(true)}, nil
}

func (c *evalCollector) judge(args []runtime.Value) ([]runtime.Value, error) {
	if len(args) < 1 || !args[0].IsTable() {
		return nil, fmt.Errorf("bad argument to 'eval.judge' (llm turn options table expected)")
	}
	if c.llmTurn.IsNil() || !c.llmTurn.IsFunction() {
		return nil, fmt.Errorf("eval.judge requires llm.turn")
	}
	opts := args[0].Table()
	if opts.RawGetString("max_tokens").IsNil() {
		opts.RawSetString("max_tokens", runtime.IntValue(200))
	}
	if opts.RawGetString("budget").IsNil() {
		budget := runtime.NewTable()
		budget.RawSetString("tokens", runtime.IntValue(512))
		budget.RawSetString("turns", runtime.IntValue(1))
		opts.RawSetString("budget", runtime.TableValue(budget))
	}
	values, err := c.call(c.llmTurn, []runtime.Value{runtime.TableValue(opts)})
	if err != nil {
		return nil, err
	}
	if len(values) > 0 && values[0].IsTable() {
		result := values[0].Table()
		if usage := result.RawGetString("usage"); usage.IsTable() {
			usageTable := usage.Table()
			input := usageTable.RawGetString("input_tokens")
			output := usageTable.RawGetString("output_tokens")
			c.recordMetric(metricFromValue("judge_cost", usageTable.RawGetString("cost")))
			c.recordMetric(metricFromValue("judge_input_tokens", input))
			c.recordMetric(metricFromValue("judge_output_tokens", output))
			if input.IsNumber() || output.IsNumber() {
				c.recordMetric(metricFromValue("judge_tokens", runtime.IntValue(evalValueInt(input)+evalValueInt(output))))
			}
		}
	}
	return values, nil
}

func (c *evalCollector) currentLLM() *LLMCaseRun {
	if c == nil || c.llm == nil {
		return nil
	}
	return c.llm()
}

func llmCaseUsageTable(usage *LLMCaseRun) *runtime.Table {
	t := runtime.NewTable()
	if usage == nil {
		t.RawSetString("turns", runtime.IntValue(0))
		t.RawSetString("stream_events", runtime.IntValue(0))
		t.RawSetString("tool_calls", runtime.IntValue(0))
		t.RawSetString("errors", runtime.IntValue(0))
		t.RawSetString("input_tokens", runtime.IntValue(0))
		t.RawSetString("output_tokens", runtime.IntValue(0))
		t.RawSetString("tokens", runtime.IntValue(0))
		t.RawSetString("latency_ms", runtime.IntValue(0))
		t.RawSetString("cost", runtime.FloatValue(0))
		return t
	}
	t.RawSetString("trace_ref", runtime.StringValue(usage.TraceRef))
	if usage.RecordPath != "" {
		t.RawSetString("record_path", runtime.StringValue(usage.RecordPath))
	}
	t.RawSetString("turns", runtime.IntValue(int64(usage.Turns)))
	t.RawSetString("stream_events", runtime.IntValue(int64(usage.StreamEvents)))
	t.RawSetString("tool_calls", runtime.IntValue(int64(usage.ToolCalls)))
	t.RawSetString("errors", runtime.IntValue(int64(usage.Errors)))
	t.RawSetString("input_tokens", runtime.IntValue(usage.InputTokens))
	t.RawSetString("output_tokens", runtime.IntValue(usage.OutputTokens))
	t.RawSetString("tokens", runtime.IntValue(usage.InputTokens+usage.OutputTokens))
	t.RawSetString("latency_ms", runtime.IntValue(usage.LatencyMS))
	t.RawSetString("cost", runtime.FloatValue(usage.Cost))
	t.RawSetString("money", runtime.FloatValue(usage.Cost))
	return t
}

func checkEvalBudget(config *runtime.Table, usage *LLMCaseRun) error {
	table := llmCaseUsageTable(usage)
	for _, key := range []string{"turns", "tokens", "input_tokens", "output_tokens", "latency_ms"} {
		limit, ok := evalBudgetInt(config, key)
		if !ok || limit <= 0 {
			continue
		}
		used := evalValueInt(table.RawGetString(key))
		if used > limit {
			return fmt.Errorf("eval.budget exceeded: %s (used %d > limit %d)", key, used, limit)
		}
	}
	for _, key := range []string{"cost", "money"} {
		limit, ok := evalBudgetFloat(config, key)
		if !ok || limit <= 0 {
			continue
		}
		used := evalValueFloat(table.RawGetString(key))
		if used > limit {
			return fmt.Errorf("eval.budget exceeded: %s (used %.6g > limit %.6g)", key, used, limit)
		}
	}
	return nil
}

func evalBudgetInt(t *runtime.Table, key string) (int64, bool) {
	if t == nil {
		return 0, false
	}
	v := t.RawGetString(key)
	if !v.IsNumber() {
		return 0, false
	}
	return evalValueInt(v), true
}

func evalBudgetFloat(t *runtime.Table, key string) (float64, bool) {
	if t == nil {
		return 0, false
	}
	v := t.RawGetString(key)
	if !v.IsNumber() {
		return 0, false
	}
	return evalValueFloat(v), true
}

func evalValueInt(v runtime.Value) int64 {
	switch {
	case v.IsInt():
		return v.Int()
	case v.IsFloat():
		return int64(v.Float())
	default:
		return 0
	}
}

func evalValueFloat(v runtime.Value) float64 {
	switch {
	case v.IsInt():
		return float64(v.Int())
	case v.IsFloat():
		return v.Float()
	default:
		return 0
	}
}

func (c *evalCollector) resolvePath(path string) string {
	if filepath.IsAbs(path) || c.baseDir == "" {
		return path
	}
	return filepath.Join(c.baseDir, path)
}

func (c *evalCollector) recordMetric(metric Metric) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.activeStack) == 0 {
		c.metrics = append(c.metrics, metric)
		return
	}
	idx := c.activeStack[len(c.activeStack)-1]
	if idx >= 0 && idx < len(c.subcases) {
		c.subcases[idx].Metrics = append(c.subcases[idx].Metrics, metric)
		return
	}
	c.metrics = append(c.metrics, metric)
}

func (c *evalCollector) startSubcase(id string, start time.Time) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	idx := len(c.subcases)
	c.subcases = append(c.subcases, Subcase{
		CaseID:    id,
		Status:    "running",
		StartedAt: start.UTC().Format(time.RFC3339),
	})
	c.activeStack = append(c.activeStack, idx)
	return idx
}

func (c *evalCollector) finishSubcase(idx int, start time.Time, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.activeStack) > 0 {
		c.activeStack = c.activeStack[:len(c.activeStack)-1]
	}
	if idx < 0 || idx >= len(c.subcases) {
		return
	}
	subcase := &c.subcases[idx]
	subcase.DurationMS = elapsedMillis(start)
	if subcase.Status == "skipped" {
		return
	}
	if err != nil {
		subcase.Status = "failed"
		subcase.Diagnostics = append(subcase.Diagnostics, Diagnostic{
			Kind:     "runtime_error",
			Severity: "error",
			Message:  err.Error(),
		})
		return
	}
	subcase.Status = "passed"
}

func (c *evalCollector) markActiveSubcaseSkipped(reason string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.activeStack) == 0 {
		return
	}
	idx := c.activeStack[len(c.activeStack)-1]
	if idx < 0 || idx >= len(c.subcases) {
		return
	}
	subcase := &c.subcases[idx]
	subcase.Status = "skipped"
	subcase.Diagnostics = append(subcase.Diagnostics, Diagnostic{
		Kind:     "skipped",
		Severity: "info",
		Message:  reason,
	})
}

func metricFromValue(name string, value runtime.Value) Metric {
	metric := Metric{Name: name}
	switch {
	case value.IsNil():
		metric.Type = "nil"
		metric.Value = nil
	case value.IsBool():
		metric.Type = "bool"
		metric.Value = value.Bool()
	case value.IsInt():
		metric.Type = "number"
		metric.Value = value.Int()
	case value.IsFloat():
		metric.Type = "number"
		metric.Value = value.Float()
	case value.IsString():
		metric.Type = "string"
		metric.Value = value.Str()
	default:
		metric.Type = value.TypeName()
		metric.Value = runtime.JSONValueToGo(value)
	}
	return metric
}

func normalizeJSONNumbers(v any) any {
	switch x := v.(type) {
	case json.Number:
		if i, err := x.Int64(); err == nil {
			return i
		}
		if f, err := x.Float64(); err == nil {
			return f
		}
		return x.String()
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, v := range x {
			out[k] = normalizeJSONNumbers(v)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, v := range x {
			out[i] = normalizeJSONNumbers(v)
		}
		return out
	default:
		return x
	}
}

func cloneSubcases(src []Subcase) []Subcase {
	if len(src) == 0 {
		return nil
	}
	out := make([]Subcase, len(src))
	for i := range src {
		out[i] = src[i]
		out[i].Metrics = append([]Metric(nil), src[i].Metrics...)
		out[i].Diagnostics = append([]Diagnostic(nil), src[i].Diagnostics...)
	}
	return out
}
