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
