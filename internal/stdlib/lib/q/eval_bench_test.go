package q

import (
	"testing"
	"time"

	"github.com/never-labs/leia/internal/stdlib/lib/data"
)

var qEvalPlanBenchSink any

func BenchmarkEvalWithEnvScriptPlanCacheCold(b *testing.B) {
	const src = "x:a+1;y:x*2;z:y+3;z"
	env := map[string]any{"a": int64(10)}
	b.ReportAllocs()
	b.ResetTimer()
	start := time.Now()
	for i := 0; i < b.N; i++ {
		ClearEvalPlanCaches()
		out, err := EvalWithEnv(src, env)
		if err != nil {
			b.Fatalf("EvalWithEnv cold: %v", err)
		}
		qEvalPlanBenchSink = out
	}
	b.StopTimer()
	b.ReportMetric(float64(b.N)/time.Since(start).Seconds(), "eval/s")
}

func BenchmarkEvalWithEnvScriptPlanCacheWarm(b *testing.B) {
	ClearEvalPlanCaches()
	defer ClearEvalPlanCaches()

	const src = "x:a+1;y:x*2;z:y+3;z"
	env := map[string]any{"a": int64(10)}
	if _, err := EvalWithEnv(src, env); err != nil {
		b.Fatalf("warm EvalWithEnv setup: %v", err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	start := time.Now()
	for i := 0; i < b.N; i++ {
		out, err := EvalWithEnv(src, env)
		if err != nil {
			b.Fatalf("EvalWithEnv warm: %v", err)
		}
		qEvalPlanBenchSink = out
	}
	b.StopTimer()
	b.ReportMetric(float64(b.N)/time.Since(start).Seconds(), "eval/s")
}

func BenchmarkPreparedEvalScriptPlanWarm(b *testing.B) {
	ClearEvalPlanCaches()
	defer ClearEvalPlanCaches()

	const src = "x:a+1;y:x*2;z:y+3;z"
	env := map[string]any{"a": int64(10)}
	prepared, err := PrepareEval(src)
	if err != nil {
		b.Fatalf("PrepareEval script: %v", err)
	}
	if _, err := prepared.EvalWithEnv(env); err != nil {
		b.Fatalf("warm PreparedEval script setup: %v", err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	start := time.Now()
	for i := 0; i < b.N; i++ {
		out, err := prepared.EvalWithEnv(env)
		if err != nil {
			b.Fatalf("PreparedEval script warm: %v", err)
		}
		qEvalPlanBenchSink = out
	}
	b.StopTimer()
	b.ReportMetric(float64(b.N)/time.Since(start).Seconds(), "eval/s")
}

func BenchmarkEvalWithEnvPipelinePlanCacheCold(b *testing.B) {
	const src = "+/v where v>threshold"
	env := map[string]any{
		"v":         data.NewI64Range(0, 1, 8192),
		"threshold": int64(4096),
	}
	b.ReportAllocs()
	b.ResetTimer()
	start := time.Now()
	for i := 0; i < b.N; i++ {
		ClearEvalPlanCaches()
		out, err := EvalWithEnv(src, env)
		if err != nil {
			b.Fatalf("EvalWithEnv pipeline cold: %v", err)
		}
		qEvalPlanBenchSink = out
	}
	b.StopTimer()
	b.ReportMetric(float64(b.N)/time.Since(start).Seconds(), "eval/s")
}

func BenchmarkEvalWithEnvPipelinePlanCacheWarm(b *testing.B) {
	ClearEvalPlanCaches()
	defer ClearEvalPlanCaches()

	const src = "+/v where v>threshold"
	env := map[string]any{
		"v":         data.NewI64Range(0, 1, 8192),
		"threshold": int64(4096),
	}
	if _, err := EvalWithEnv(src, env); err != nil {
		b.Fatalf("warm EvalWithEnv pipeline setup: %v", err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	start := time.Now()
	for i := 0; i < b.N; i++ {
		out, err := EvalWithEnv(src, env)
		if err != nil {
			b.Fatalf("EvalWithEnv pipeline warm: %v", err)
		}
		qEvalPlanBenchSink = out
	}
	b.StopTimer()
	b.ReportMetric(float64(b.N)/time.Since(start).Seconds(), "eval/s")
}

func BenchmarkPreparedEvalPipelinePlanWarm(b *testing.B) {
	ClearEvalPlanCaches()
	defer ClearEvalPlanCaches()

	const src = "+/v where v>threshold"
	env := map[string]any{
		"v":         data.NewI64Range(0, 1, 8192),
		"threshold": int64(4096),
	}
	prepared, err := PrepareEval(src)
	if err != nil {
		b.Fatalf("PrepareEval pipeline: %v", err)
	}
	if _, err := prepared.EvalWithEnv(env); err != nil {
		b.Fatalf("warm PreparedEval pipeline setup: %v", err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	start := time.Now()
	for i := 0; i < b.N; i++ {
		out, err := prepared.EvalWithEnv(env)
		if err != nil {
			b.Fatalf("PreparedEval pipeline warm: %v", err)
		}
		qEvalPlanBenchSink = out
	}
	b.StopTimer()
	b.ReportMetric(float64(b.N)/time.Since(start).Seconds(), "eval/s")
}

func BenchmarkEvalWithEnvRuntimePrimitivePipelineWarm(b *testing.B) {
	ClearEvalPlanCaches()
	defer ClearEvalPlanCaches()

	const src = "(w wsum v)+(v cor y)+(count 32 mdev v)"
	env := map[string]any{
		"v": data.NewI64Range(1, 1, 8192),
		"w": data.NewI64Range(1, 1, 8192),
		"y": data.NewI64Range(2, 2, 8192),
	}
	if _, err := EvalWithEnv(src, env); err != nil {
		b.Fatalf("warm EvalWithEnv runtime primitive setup: %v", err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	start := time.Now()
	for i := 0; i < b.N; i++ {
		out, err := EvalWithEnv(src, env)
		if err != nil {
			b.Fatalf("EvalWithEnv runtime primitive warm: %v", err)
		}
		qEvalPlanBenchSink = out
	}
	b.StopTimer()
	b.ReportMetric(float64(b.N)/time.Since(start).Seconds(), "eval/s")
}

func BenchmarkPreparedEvalRuntimePrimitivePipelineWarm(b *testing.B) {
	ClearEvalPlanCaches()
	defer ClearEvalPlanCaches()

	const src = "(w wsum v)+(v cor y)+(count 32 mdev v)"
	env := map[string]any{
		"v": data.NewI64Range(1, 1, 8192),
		"w": data.NewI64Range(1, 1, 8192),
		"y": data.NewI64Range(2, 2, 8192),
	}
	prepared, err := PrepareEval(src)
	if err != nil {
		b.Fatalf("PrepareEval runtime primitive: %v", err)
	}
	if _, err := prepared.EvalWithEnv(env); err != nil {
		b.Fatalf("warm PreparedEval runtime primitive setup: %v", err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	start := time.Now()
	for i := 0; i < b.N; i++ {
		out, err := prepared.EvalWithEnv(env)
		if err != nil {
			b.Fatalf("PreparedEval runtime primitive warm: %v", err)
		}
		qEvalPlanBenchSink = out
	}
	b.StopTimer()
	b.ReportMetric(float64(b.N)/time.Since(start).Seconds(), "eval/s")
}

func BenchmarkEvalApplyIndexScalarFastPathWarm(b *testing.B) {
	cases := []struct {
		name  string
		setup string
		expr  string
	}{
		{name: "AtScalar", setup: "x:til 8192", expr: "x@100"},
		{name: "DotScalar", setup: "x:til 8192", expr: "x . 100"},
		{name: "DotApplyArgs", setup: "f:{x+y}", expr: ".[f;(100;23)]"},
	}
	for _, tt := range cases {
		b.Run(tt.name, func(b *testing.B) {
			state := NewEvalState(nil)
			if _, err := state.Eval(tt.setup); err != nil {
				b.Fatalf("setup %q: %v", tt.setup, err)
			}
			if _, err := state.Eval(tt.expr); err != nil {
				b.Fatalf("warm %q: %v", tt.expr, err)
			}
			ClearRuntimeKernelExecutionStats()
			b.ReportAllocs()
			b.ResetTimer()
			start := time.Now()
			for i := 0; i < b.N; i++ {
				out, err := state.Eval(tt.expr)
				if err != nil {
					b.Fatalf("Eval %q: %v", tt.expr, err)
				}
				qEvalPlanBenchSink = out
			}
			b.StopTimer()
			b.ReportMetric(float64(b.N)/time.Since(start).Seconds(), "eval/s")
			qEvalReportScalarIndexStats(b)
		})
	}
}

func qEvalReportScalarIndexStats(b *testing.B) {
	b.Helper()
	var attempts, hits, fallbacks, errors uint64
	for _, stat := range RuntimeKernelExecutionStats() {
		if stat.Kernel != "ArrayScalarIndex" && stat.Kernel != "StringScalarIndex" {
			continue
		}
		switch stat.Outcome {
		case "attempt":
			attempts += stat.Count
		case "hit":
			hits += stat.Count
		case "fallback":
			fallbacks += stat.Count
		case "error":
			errors += stat.Count
		}
	}
	if attempts > 0 {
		b.ReportMetric(100*float64(hits)/float64(attempts), "scalar_index_hit_pct")
	}
	if b.N > 0 {
		b.ReportMetric(float64(attempts)/float64(b.N), "scalar_index_attempts/op")
		b.ReportMetric(float64(hits)/float64(b.N), "scalar_index_hits/op")
		b.ReportMetric(float64(fallbacks)/float64(b.N), "scalar_index_fallbacks/op")
		b.ReportMetric(float64(errors)/float64(b.N), "scalar_index_errors/op")
	}
}
