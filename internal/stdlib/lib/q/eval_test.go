package q

import (
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/never-labs/leia/internal/stdlib/lib/data"
)

func TestEvalNumericAtomsAndVectors(t *testing.T) {
	assertEvalValue(t, "42", int64(42))
	assertEvalValue(t, "1.5", 1.5)
	assertEvalValue(t, "0W", int64(math.MaxInt64))
	assertEvalValue(t, "-0W", -int64(math.MaxInt64))
	assertEvalValue(t, "0w", math.Inf(1))
	assertEvalValue(t, "-0w", math.Inf(-1))
	assertEvalValue(t, "0Wh", int16(math.MaxInt16))
	assertEvalValue(t, "-0Wh", -int16(math.MaxInt16))
	assertEvalValue(t, "0Wi", int32(math.MaxInt32))
	assertEvalValue(t, "-0Wi", -int32(math.MaxInt32))
	assertEvalValue(t, "0Wj", int64(math.MaxInt64))
	assertEvalValue(t, "-0Wj", -int64(math.MaxInt64))
	assertEvalValue(t, "0We", float32(math.Inf(1)))
	assertEvalValue(t, "-0We", float32(math.Inf(-1)))
	assertEvalValue(t, "0Wf", math.Inf(1))
	assertEvalValue(t, "-0Wf", math.Inf(-1))
	assertEvalArray(t, "1 2 3", data.KindI64, []any{int64(1), int64(2), int64(3)})
	assertEvalArray(t, "1 2.5 3", data.KindF64, []any{1.0, 2.5, 3.0})
	assertEvalArray(t, "0W 0w -0W", data.KindF64, []any{float64(math.MaxInt64), math.Inf(1), -float64(math.MaxInt64)})
	assertEvalArray(t, "0Wh -0Wh", data.KindI16, []any{int16(math.MaxInt16), -int16(math.MaxInt16)})
	assertEvalArray(t, "0Wi -0Wi", data.KindI32, []any{int32(math.MaxInt32), -int32(math.MaxInt32)})
	assertEvalArray(t, "0Wj -0Wj", data.KindI64, []any{int64(math.MaxInt64), -int64(math.MaxInt64)})
	assertEvalArray(t, "0We -0We", data.KindF32, []any{float32(math.Inf(1)), float32(math.Inf(-1))})
	assertEvalArray(t, "0Wf -0Wf", data.KindF64, []any{math.Inf(1), math.Inf(-1)})
}

func TestEvalTypedNumericSuffixLiterals(t *testing.T) {
	assertEvalValue(t, "1h", int16(1))
	assertEvalValue(t, "2i", int32(2))
	assertEvalValue(t, "3j", int64(3))
	assertEvalValue(t, "1.25e", float32(1.25))
	assertEvalValue(t, "1.5f", 1.5)

	assertEvalArray(t, "1h 2h 0Nh", data.KindI16, []any{int16(1), int16(2), data.NullValue})
	assertEvalArray(t, "1i 2i 0Ni", data.KindI32, []any{int32(1), int32(2), data.NullValue})
	assertEvalArray(t, "1j 2j 0Nj", data.KindI64, []any{int64(1), int64(2), data.NullValue})
	assertEvalArray(t, "1.5e 2.5e 0Ne", data.KindF32, []any{float32(1.5), float32(2.5), data.NullValue})
	assertEvalArray(t, "1.5f 2.5f 0Nf", data.KindF64, []any{1.5, 2.5, data.NullValue})

	assertEvalValue(t, "type 1h", int64(-5))
	assertEvalValue(t, "type 1i", int64(-6))
	assertEvalValue(t, "type 1j", int64(-7))
	assertEvalValue(t, "type 1e", int64(-8))
	assertEvalValue(t, "type 1f", int64(-9))
	assertEvalValue(t, "+/1h 2h 3h", int64(6))
	assertEvalArray(t, "1i 2i 3i<3i", data.KindBool, []any{true, true, false})
}

func TestRuntimeKernelExecutionStatsReportHitAndFallbackOutcomes(t *testing.T) {
	ClearRuntimeKernelExecutionStats()
	t.Cleanup(ClearRuntimeKernelExecutionStats)

	assertEvalValue(t, "+/1 2 3", int64(6))
	if _, err := Eval("+/`a`b"); err == nil {
		t.Fatalf("Eval(+/`a`b) succeeded, want numeric error")
	}
	stats := RuntimeKernelExecutionStats()
	counts := map[string]uint64{}
	for _, stat := range stats {
		key := stat.Kernel + "/" + stat.Outcome + "/" + stat.ReasonCode
		counts[key] += stat.Count
	}
	if counts["ArraySum/attempt/attempt"] != 2 {
		t.Fatalf("ArraySum attempts = %d, want 2; stats=%#v", counts["ArraySum/attempt/attempt"], stats)
	}
	if counts["ArraySum/hit/typed_kernel"] != 1 {
		t.Fatalf("ArraySum hits = %d, want 1; stats=%#v", counts["ArraySum/hit/typed_kernel"], stats)
	}
	if counts["ArraySum/fallback/unsupported_type"] != 1 {
		t.Fatalf("ArraySum fallbacks = %d, want 1; stats=%#v", counts["ArraySum/fallback/unsupported_type"], stats)
	}
}

func TestEvalTypedFindComparableStringSymbolRuntime(t *testing.T) {
	ClearRuntimeKernelExecutionStats()
	t.Cleanup(ClearRuntimeKernelExecutionStats)

	assertEvalArray(t, "d:`AAPL`MSFT`AAPL`NVDA;q:`MSFT`IBM`AAPL;d?q", data.KindI64, []any{int64(1), int64(4), int64(0)})
	assertEvalValue(t, "+/`AAPL`MSFT`AAPL`NVDA?`NVDA`IBM`AAPL", int64(7))
	assertEvalArray(t, "d:`AAPL`MSFT`AAPL`NVDA;q:(\"MSFT\";\"IBM\";\"AAPL\");d?q", data.KindI64, []any{int64(1), int64(4), int64(0)})

	seenFindSymbol := false
	seenFindMixed := false
	seenFindSumSymbol := false
	for _, stat := range RuntimeKernelExecutionStats() {
		if stat.Kernel == "ArrayFind" && stat.Shape == "find/symbol/symbol" && stat.Outcome == "hit" && stat.ReasonCode == "typed_kernel" && stat.Count > 0 {
			seenFindSymbol = true
		}
		if stat.Kernel == "ArrayFind" && stat.Shape == "find/symbol/string" && stat.Outcome == "hit" && stat.ReasonCode == "typed_kernel" && stat.Count > 0 {
			seenFindMixed = true
		}
		if stat.Kernel == "ArrayFindSum" && stat.Shape == "vector-reduce/find-sum/symbol/symbol" && stat.Outcome == "hit" && stat.ReasonCode == "typed_kernel" && stat.Count > 0 {
			seenFindSumSymbol = true
		}
		if (stat.Kernel == "ArrayFind" && stat.Shape == "find/symbol/symbol" ||
			stat.Kernel == "ArrayFindSum" && stat.Shape == "vector-reduce/find-sum/symbol/symbol") &&
			stat.Outcome == "fallback" {
			t.Fatalf("unexpected symbol find fallback stat: %#v all=%#v", stat, RuntimeKernelExecutionStats())
		}
	}
	if !seenFindSymbol || !seenFindMixed || !seenFindSumSymbol {
		t.Fatalf("missing typed find stats: findSymbol=%v findMixed=%v findSumSymbol=%v stats=%#v", seenFindSymbol, seenFindMixed, seenFindSumSymbol, RuntimeKernelExecutionStats())
	}
}

func TestRuntimeKernelExecutionStatsAggregateStableFallbackReasons(t *testing.T) {
	ClearRuntimeKernelExecutionStats()
	t.Cleanup(ClearRuntimeKernelExecutionStats)

	recordRuntimeKernelExecution("TestKernel", "shape", "fallback", "unsupported runtime shape")
	recordRuntimeKernelExecution("TestKernel", "shape", "fallback", "unsupported kind")
	recordRuntimeKernelExecution("TestKernel", "shape", "fallback", "unsupported plan")
	recordRuntimeKernelExecution("TestKernel", "shape", "fallback", "semantic guard")
	recordRuntimeKernelExecution("TestKernel", "shape", "error", "typed helper failed")
	recordRuntimeKernelExecution("TestKernel", "shape", "error", RuntimeFallbackUnsupportedType)
	recordRuntimeKernelExecution("TestKernel", "shape", "error", RuntimeFallbackApplyError)
	recordRuntimeKernelExecution("TestKernel", "shape", "error", RuntimeFallbackPipelineError)
	recordRuntimeKernelProbeReason("TestProbe", "shape", false, nil, "type mismatch")

	counts := map[string]uint64{}
	for _, stat := range RuntimeKernelExecutionStats() {
		key := stat.Kernel + "/" + stat.Outcome + "/" + stat.ReasonCode
		counts[key] += stat.Count
	}
	want := map[string]uint64{
		"TestKernel/fallback/unsupported_shape": 1,
		"TestKernel/fallback/unsupported_type":  1,
		"TestKernel/fallback/planner_unhandled": 1,
		"TestKernel/fallback/semantic_guard":    1,
		"TestKernel/error/runtime_error":        1,
		"TestKernel/error/unsupported_type":     1,
		"TestKernel/error/apply_error":          1,
		"TestKernel/error/pipeline_error":       1,
		"TestProbe/attempt/attempt":             1,
		"TestProbe/fallback/unsupported_type":   1,
	}
	for key, expected := range want {
		if counts[key] != expected {
			t.Fatalf("runtime stats %s = %d, want %d; stats=%#v", key, counts[key], expected, RuntimeKernelExecutionStats())
		}
	}
}

func TestRuntimeKernelExecutionStatsMapPipelineShapes(t *testing.T) {
	ClearRuntimeKernelExecutionStats()
	t.Cleanup(ClearRuntimeKernelExecutionStats)

	assertEvalValue(t, "x:til 32;y:x*2;+/y[where (x>=4) and x<12]", int64(120))
	assertEvalValue(t, "+/deltas til 16", int64(15))

	seen := map[string]bool{}
	for _, stat := range RuntimeKernelExecutionStats() {
		if stat.Outcome == "hit" && stat.ReasonCode == "typed_kernel" {
			seen[stat.PipelineShape] = true
		}
	}
	for _, shape := range []string{"vector_map", "mask_combine", "mask_reduce", "vector_reduce"} {
		if !seen[shape] {
			t.Fatalf("missing pipeline shape %q in runtime stats: %#v", shape, RuntimeKernelExecutionStats())
		}
	}
}

func TestRuntimeNumericUnaryUsesRuntimePrimitiveShape(t *testing.T) {
	ClearRuntimeKernelExecutionStats()
	t.Cleanup(ClearRuntimeKernelExecutionStats)

	assertEvalArray(t, "sqrt 4 9 16", data.KindF64, []any{2.0, 3.0, 4.0})

	seenTypedUnary := false
	for _, stat := range RuntimeKernelExecutionStats() {
		if strings.HasPrefix(stat.Shape, "vector-unary/") {
			t.Fatalf("numeric unary used legacy vector-unary shape: %#v all=%#v", stat, RuntimeKernelExecutionStats())
		}
		if stat.Kernel == "ArrayNumericUnary" &&
			stat.Shape == "runtime-unary/sqrt" &&
			stat.PipelineShape == "numeric_math" &&
			stat.Outcome == "hit" &&
			stat.ReasonCode == "typed_kernel" {
			seenTypedUnary = true
		}
	}
	if !seenTypedUnary {
		t.Fatalf("missing unified ArrayNumericUnary runtime primitive hit: %#v", RuntimeKernelExecutionStats())
	}
}

func TestRuntimeNumericMathVerbFamilyUsesTypedOrdinaryExpressionPath(t *testing.T) {
	tests := []struct {
		name   string
		expr   string
		kernel string
		shape  string
	}{
		{name: "abs", expr: "abs -2 3 0", kernel: "ArrayNumericUnary", shape: "runtime-unary/abs"},
		{name: "sqrt", expr: "sqrt 4 9 16", kernel: "ArrayNumericUnary", shape: "runtime-unary/sqrt"},
		{name: "log", expr: "log 1 2 4", kernel: "ArrayNumericUnary", shape: "runtime-unary/log"},
		{name: "exp", expr: "exp 0 1 2", kernel: "ArrayNumericUnary", shape: "runtime-unary/exp"},
		{name: "sin", expr: "sin 0 1 2", kernel: "ArrayNumericUnary", shape: "runtime-unary/sin"},
		{name: "cos", expr: "cos 0 1 2", kernel: "ArrayNumericUnary", shape: "runtime-unary/cos"},
		{name: "tan", expr: "tan 0 1 2", kernel: "ArrayNumericUnary", shape: "runtime-unary/tan"},
		{name: "asin", expr: "asin -0.5 0 0.5", kernel: "ArrayNumericUnary", shape: "runtime-unary/asin"},
		{name: "acos", expr: "acos -0.5 0 0.5", kernel: "ArrayNumericUnary", shape: "runtime-unary/acos"},
		{name: "atan", expr: "atan -1 0 1", kernel: "ArrayNumericUnary", shape: "runtime-unary/atan"},
		{name: "reciprocal", expr: "reciprocal 2 4 8", kernel: "ArrayNumericUnary", shape: "runtime-unary/reciprocal"},
		{name: "signum", expr: "signum -2 0 3", kernel: "ArrayNumericUnary", shape: "runtime-unary/signum"},
		{name: "floor", expr: "floor 1.9 -1.2 3.0", kernel: "ArrayNumericUnary", shape: "runtime-unary/floor"},
		{name: "ceiling", expr: "ceiling 1.1 -1.2 3.0", kernel: "ArrayNumericUnary", shape: "runtime-unary/ceiling"},
		{name: "xexp", expr: "2 xexp 3 4 5", kernel: "ArrayNumericDyadicFloat", shape: "runtime-dyadic/xexp"},
		{name: "xlog", expr: "2 xlog 8 16 32", kernel: "ArrayNumericDyadicFloat", shape: "runtime-dyadic/xlog"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ClearRuntimeKernelExecutionStats()
			t.Cleanup(ClearRuntimeKernelExecutionStats)

			if _, err := Eval(tt.expr); err != nil {
				t.Fatalf("Eval(%q) returned error: %v", tt.expr, err)
			}

			seen := false
			for _, stat := range RuntimeKernelExecutionStats() {
				if stat.Outcome == "fallback" || stat.Outcome == "error" {
					t.Fatalf("unexpected runtime fallback/error for %q: %#v all=%#v", tt.expr, stat, RuntimeKernelExecutionStats())
				}
				if stat.Kernel == tt.kernel &&
					stat.Shape == tt.shape &&
					stat.PipelineShape == "numeric_math" &&
					stat.Outcome == "hit" &&
					stat.ReasonCode == "typed_kernel" &&
					stat.Count > 0 {
					seen = true
				}
			}
			if !seen {
				t.Fatalf("missing %s typed runtime hit for %q: %#v", tt.kernel, tt.expr, RuntimeKernelExecutionStats())
			}
		})
	}
}

func TestEvalFunctionalAmendAddUsesTypedIndexedAccumulation(t *testing.T) {
	ClearRuntimeKernelExecutionStats()
	t.Cleanup(ClearRuntimeKernelExecutionStats)

	assertEvalArray(t, "x:6#0;@[x;2 4 2;+;10 40 3]", data.KindI64, []any{int64(0), int64(0), int64(13), int64(0), int64(40), int64(0)})

	for _, stat := range RuntimeKernelExecutionStats() {
		if stat.Kernel == "ArrayAmendAddIndexArray" && stat.Shape == "amend-add-index-array/i64/i64/i64" && stat.Outcome == "hit" && stat.ReasonCode == "typed_kernel" && stat.Count > 0 {
			return
		}
		if stat.Kernel == "ArrayAmendAddIndexes" && stat.Shape == "amend-add-indexes/i64/i64" && stat.Outcome == "hit" && stat.ReasonCode == "typed_kernel" && stat.Count > 0 {
			return
		}
	}
	t.Fatalf("missing ArrayAmendAddIndexArray hit: %#v", RuntimeKernelExecutionStats())
}

func TestQScriptPipelinePlannerDescribesAssignmentTerminalWhereIndexReduce(t *testing.T) {
	plan := buildQScriptPlan("x:til 64;y:(x*3)+7;lo:8;hi:32;idx:where (x>=lo) and x<hi;+/y[idx]")
	if plan.scriptPipeline == nil {
		t.Fatalf("script pipeline descriptor missing")
	}
	d := plan.scriptPipeline
	if d.kind != qScriptPipelineWhereIndexReduceSum {
		t.Fatalf("pipeline kind = %q, want %q", d.kind, qScriptPipelineWhereIndexReduceSum)
	}
	if got, want := len(d.assignments), 5; got != want {
		t.Fatalf("assignment count = %d, want %d", got, want)
	}
	for _, assignment := range d.assignments {
		if assignment.binding.kind == qScriptBindingInvalid {
			t.Fatalf("assignment %q binding plan is invalid", assignment.name)
		}
	}
	if d.terminalPlan.kind != qPipelineSumGatherIndexes {
		t.Fatalf("terminal plan kind = %v, want qPipelineSumGatherIndexes", d.terminalPlan.kind)
	}
	if d.valueExpr != "y" || d.valueBinding != "(x*3)+7" {
		t.Fatalf("value descriptor = expr %q binding %q, want y/(x*3)+7", d.valueExpr, d.valueBinding)
	}
	if d.indexExpr != "idx" || d.indexBinding != "where (x>=lo) and x<hi" {
		t.Fatalf("index descriptor = expr %q binding %q, want idx/where mask", d.indexExpr, d.indexBinding)
	}
	if d.maskExpr != "(x>=lo) and x<hi" {
		t.Fatalf("mask expr = %q, want compound mask", d.maskExpr)
	}
	if got, want := d.shape(), "script-pipeline/where-index-reduce/sum/assignments"; got != want {
		t.Fatalf("shape = %q, want %q", got, want)
	}
}

func TestQPipelineModuloComparePlanFromMask(t *testing.T) {
	plan, ok := qPipelineModuloComparePlanFromMask("(x mod 3)=0")
	if !ok {
		t.Fatal("modulo compare mask was not recognized")
	}
	if plan.kind != qPipelineWhereModuloCompareIndexes || plan.modExpr != "x" || plan.modulusExpr != "3" || plan.modTargetExpr != "0" || plan.compareOp != "=" {
		t.Fatalf("modulo compare plan = %#v", plan)
	}
	if plan.modPlan.kind == qScriptBindingInvalid || plan.modulusPlan.kind == qScriptBindingInvalid || plan.modTargetPlan.kind == qScriptBindingInvalid {
		t.Fatalf("modulo compare plan was not bound: %#v", plan)
	}
}

func TestQPipelinePlanNormalizeIsIdempotent(t *testing.T) {
	plan := qPipelinePlanWithBindingPlans(qPipelinePlan{
		kind:      qPipelineSumWhereIndex,
		shape:     "where-index-reduce/sum",
		valueExpr: "y",
		indexExpr: "idx",
		maskExpr:  "(x mod 3)=0",
	})
	plan = qPipelinePlanWithBindingPlans(plan)
	if got, want := len(plan.operands), 3; got != want {
		t.Fatalf("operand count after repeated normalize = %d, want %d; plan=%#v", got, want, plan)
	}
	if plan.moduloMaskPlan == nil || plan.moduloMaskPlan.modPlan.kind == qScriptBindingInvalid {
		t.Fatalf("normalized modulo mask sub-plan missing or unbound: %#v", plan.moduloMaskPlan)
	}
	if plan.stableShape() != "where-index-reduce/sum" || plan.stablePipelineShape() != "mask_reduce" {
		t.Fatalf("stable shape after normalize = %q/%q", plan.stableShape(), plan.stablePipelineShape())
	}
}

func TestQPipelinePlanCachesBoundModuloMaskPlan(t *testing.T) {
	state := NewEvalState(nil)
	plan := state.qPipelinePlan("+/y[where (x mod 3)=0]")
	if plan.kind != qPipelineSumWhereIndex {
		t.Fatalf("pipeline kind = %v, want qPipelineSumWhereIndex", plan.kind)
	}
	if plan.moduloMaskPlan == nil {
		t.Fatalf("modulo mask sub-plan was not cached: %#v", plan)
	}
	if plan.moduloMaskPlan.modPlan.kind == qScriptBindingInvalid ||
		plan.moduloMaskPlan.modulusPlan.kind == qScriptBindingInvalid ||
		plan.moduloMaskPlan.modTargetPlan.kind == qScriptBindingInvalid {
		t.Fatalf("modulo mask sub-plan was not bound: %#v", plan.moduloMaskPlan)
	}

	cached := state.qPipelinePlan("+/y[where (x mod 3)=0]")
	if cached.moduloMaskPlan == nil || cached.moduloMaskPlan.modExpr != "x" {
		t.Fatalf("cached modulo mask sub-plan missing: %#v", cached)
	}
}

func TestQPipelinePlanRecognizesGenericVectorReduceAndCount(t *testing.T) {
	sumPlan := buildQPipelinePlan("sum reverse 8#til 4")
	if sumPlan.kind != qPipelineSumSequenceTransform || sumPlan.shape != "vector-reduce/sum-reverse" || sumPlan.reductionInput != "8#til 4" {
		t.Fatalf("sum vector expr plan = %#v", sumPlan)
	}
	countPlan := buildQPipelinePlan("count 8#til 4")
	if countPlan.kind != qPipelineCountVectorExpr || countPlan.shape != "vector-count/expr" || countPlan.reductionInput != "8#til 4" {
		t.Fatalf("count vector expr plan = %#v", countPlan)
	}
	assertEvalValue(t, "sum reverse 8#til 4", int64(12))
	assertEvalValue(t, "count 8#til 4", int64(8))
}

func TestQPipelinePlanRecognizesSequenceStringCounts(t *testing.T) {
	tests := []struct {
		expr      string
		shape     string
		transform string
		want      any
		kernel    string
	}{
		{expr: "count 100 sublist til 1000", shape: "sequence-count/sublist", transform: "sublist", want: int64(100), kernel: "SequenceSublistCount"},
		{expr: "count 0 100 200 cut til 1000", shape: "sequence-count/cut", transform: "cut", want: int64(3), kernel: "SequenceCutCount"},
		{expr: "count (til 16) cross til 16", shape: "sequence-count/cross", transform: "cross", want: int64(256), kernel: "SequenceCrossCount"},
		{expr: `count trim 1000#" AAPL "`, shape: "sequence-count/trim", transform: "trim", want: int64(999), kernel: "StringRepeatedTrimCount"},
	}
	for _, tt := range tests {
		t.Run(tt.shape, func(t *testing.T) {
			ClearRuntimeKernelExecutionStats()
			t.Cleanup(ClearRuntimeKernelExecutionStats)

			assertEvalValue(t, tt.expr, tt.want)
			descriptor, ok := DescribeEvalPipeline(tt.expr)
			if !ok {
				t.Fatalf("DescribeEvalPipeline(%q) did not recognize sequence count", tt.expr)
			}
			if descriptor.Shape != tt.shape ||
				descriptor.PipelineShape != "sequence_count" ||
				descriptor.ShapeReducer != "count" ||
				descriptor.ShapeTransform != tt.transform {
				t.Fatalf("descriptor = %#v, want shape=%q transform=%q", descriptor, tt.shape, tt.transform)
			}
			out, handled, err := ExecuteEvalPipelineDescriptor(descriptor)
			if err != nil || !handled || !reflect.DeepEqual(out, tt.want) {
				t.Fatalf("ExecuteEvalPipelineDescriptor = %#v,%v,%v; want %#v,true,nil", out, handled, err, tt.want)
			}

			seenPipeline := false
			seenKernel := false
			for _, stat := range RuntimeKernelExecutionStats() {
				if stat.Outcome == "fallback" || stat.Outcome == "error" {
					t.Fatalf("unexpected pipeline fallback/error: %#v all=%#v", stat, RuntimeKernelExecutionStats())
				}
				if stat.Kernel == "QPipelinePlan" && stat.Shape == tt.shape && stat.Outcome == "hit" {
					seenPipeline = true
				}
				if stat.Kernel == tt.kernel && stat.Outcome == "hit" {
					seenKernel = true
				}
			}
			if !seenPipeline || !seenKernel {
				t.Fatalf("missing sequence pipeline/kernel stats: pipeline=%v kernel=%v all=%#v", seenPipeline, seenKernel, RuntimeKernelExecutionStats())
			}
		})
	}
}

func TestQPipelinePlanRecognizesRazeSumAndCounts(t *testing.T) {
	tests := []struct {
		expr          string
		shape         string
		pipelineShape string
		transform     string
		want          any
		kernel        string
	}{
		{expr: "+/raze 2 3#til 6", shape: "vector-reduce/sum-raze", pipelineShape: "vector_reduce", transform: "raze", want: int64(15), kernel: "ArrayNestedRazeSum"},
		{expr: "+/raze flip 2 3#til 6", shape: "vector-reduce/sum-raze", pipelineShape: "vector_reduce", transform: "raze", want: int64(15), kernel: "ArrayNestedRazeSum"},
		{expr: "+/raze[(til 6;reverse til 6;3#til 6)]", shape: "vector-reduce/sum-raze", pipelineShape: "vector_reduce", transform: "raze", want: int64(33), kernel: "ArrayNestedRazeSum"},
		{expr: "count raze 2 3#til 6", shape: "sequence-count/raze", pipelineShape: "sequence_count", transform: "raze", want: int64(6), kernel: "SequenceRazeCount"},
		{expr: "count 2 3#til 6", shape: "sequence-count/value", pipelineShape: "sequence_count", transform: "value", want: int64(2), kernel: "SequenceValueCount"},
	}
	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			ClearRuntimeKernelExecutionStats()
			t.Cleanup(ClearRuntimeKernelExecutionStats)

			descriptor, ok := DescribeEvalPipeline(tt.expr)
			if !ok {
				t.Fatalf("DescribeEvalPipeline(%q) did not recognize raze/count pipeline", tt.expr)
			}
			if descriptor.Shape != tt.shape ||
				descriptor.PipelineShape != tt.pipelineShape ||
				descriptor.ShapeTransform != tt.transform {
				t.Fatalf("descriptor = %#v, want shape=%q pipeline=%q transform=%q", descriptor, tt.shape, tt.pipelineShape, tt.transform)
			}
			out, handled, err := ExecuteEvalPipelineDescriptor(descriptor)
			if err != nil || !handled || !reflect.DeepEqual(out, tt.want) {
				t.Fatalf("ExecuteEvalPipelineDescriptor = %#v,%v,%v; want %#v,true,nil", out, handled, err, tt.want)
			}
			assertEvalValue(t, tt.expr, tt.want)

			seenPipeline := false
			seenKernel := false
			for _, stat := range RuntimeKernelExecutionStats() {
				if stat.Outcome == "fallback" || stat.Outcome == "error" {
					t.Fatalf("unexpected raze pipeline fallback/error: %#v all=%#v", stat, RuntimeKernelExecutionStats())
				}
				if stat.Kernel == "QPipelinePlan" && stat.Shape == tt.shape && stat.Outcome == "hit" {
					seenPipeline = true
				}
				if stat.Kernel == tt.kernel && stat.Outcome == "hit" {
					seenKernel = true
				}
			}
			if !seenPipeline || !seenKernel {
				t.Fatalf("missing raze pipeline/kernel stats: pipeline=%v kernel=%v all=%#v", seenPipeline, seenKernel, RuntimeKernelExecutionStats())
			}
		})
	}
}

func TestQPipelinePlanRecognizesMatrixMultiplyRazeSum(t *testing.T) {
	ClearRuntimeKernelExecutionStats()
	t.Cleanup(ClearRuntimeKernelExecutionStats)

	expr := "+/raze mmu[(2 3#1 2 3 4 5 6);(3 2#10 20 30 40 50 60)]"
	descriptor, ok := DescribeEvalPipeline(expr)
	if !ok {
		t.Fatalf("DescribeEvalPipeline(%q) did not recognize matrix multiply sum", expr)
	}
	if descriptor.Shape != "vector-reduce/sum-raze" ||
		descriptor.PipelineShape != "vector_reduce" ||
		descriptor.ShapeReducer != "sum" ||
		descriptor.ShapeTransform != "raze" {
		t.Fatalf("descriptor = %#v, want sum raze vector pipeline", descriptor)
	}
	out, handled, err := ExecuteEvalPipelineDescriptor(descriptor)
	if err != nil || !handled || out != 1630.0 {
		t.Fatalf("ExecuteEvalPipelineDescriptor = %#v,%v,%v; want 1630,true,nil", out, handled, err)
	}
	assertEvalValue(t, expr, 1630.0)

	seenPipeline := false
	seenKernel := false
	for _, stat := range RuntimeKernelExecutionStats() {
		if stat.Outcome == "fallback" || stat.Outcome == "error" {
			t.Fatalf("unexpected matrix multiply sum fallback/error: %#v all=%#v", stat, RuntimeKernelExecutionStats())
		}
		if stat.Kernel == "QPipelinePlan" && stat.Shape == "vector-reduce/sum-raze" && stat.Outcome == "hit" {
			seenPipeline = true
		}
		if stat.Kernel == "MatrixMultiplySum" && stat.Outcome == "hit" && stat.ReasonCode == "typed_kernel" {
			seenKernel = true
		}
	}
	if !seenPipeline || !seenKernel {
		t.Fatalf("missing matrix multiply pipeline/kernel stats: pipeline=%v kernel=%v all=%#v", seenPipeline, seenKernel, RuntimeKernelExecutionStats())
	}
}

func TestQPipelinePlanRecognizesScalarApplyIndex(t *testing.T) {
	if plan, ok := buildQPipelineApplyPathIndexPlan("m . 1 2"); !ok ||
		plan.shape != "apply-index/path-dot" ||
		plan.valueExpr != "m" ||
		plan.indexExpr != "1 2" {
		t.Fatalf("buildQPipelineApplyPathIndexPlan = %#v,%v; want path-dot m 1 2", plan, ok)
	}
	if plan, ok := buildQPipelineApplyGatherIndexPlan("x@2 0"); !ok ||
		plan.shape != "apply-index/gather-at" ||
		plan.valueExpr != "x" ||
		plan.indexExpr != "2 0" {
		t.Fatalf("buildQPipelineApplyGatherIndexPlan = %#v,%v; want gather-at x 2 0", plan, ok)
	}
	tests := []struct {
		expr          string
		shape         string
		valueExpr     string
		indexExpr     string
		want          any
		pipelineShape string
	}{
		{expr: "x:10 20 30;x@1", shape: "script-pipeline/apply-index/scalar-at/assignments", valueExpr: "x", indexExpr: "1", want: int64(20), pipelineShape: "script_pipeline"},
		{expr: "x:10 20 30 40;x@2 0", shape: "script-pipeline/apply-index/gather-at/assignments", valueExpr: "x", indexExpr: "2 0", want: data.NewI64([]int64{30, 10}), pipelineShape: "script_pipeline"},
		{expr: "x:10 20 30;x . 2", shape: "script-pipeline/apply-index/scalar-dot/assignments", valueExpr: "x", indexExpr: "2", want: int64(30), pipelineShape: "script_pipeline"},
		{expr: "m:2 3#til 6;m . 1", shape: "script-pipeline/apply-index/scalar-dot/assignments", valueExpr: "m", indexExpr: "1", want: data.NewI64([]int64{3, 4, 5}), pipelineShape: "script_pipeline"},
		{expr: "m:2 3#til 6;m . 1 2", shape: "script-pipeline/apply-index/path-dot/assignments", valueExpr: "m", indexExpr: "1 2", want: int64(5), pipelineShape: "script_pipeline"},
	}
	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			ClearRuntimeKernelExecutionStats()
			t.Cleanup(ClearRuntimeKernelExecutionStats)

			descriptor, ok := DescribeEvalPipeline(tt.expr)
			if !ok {
				plan := buildQScriptPlan(tt.expr)
				t.Fatalf("DescribeEvalPipeline(%q) did not recognize scalar apply-index; scriptPipeline=%#v statements=%#v", tt.expr, plan.scriptPipeline, plan.statements)
			}
			if descriptor.Shape != tt.shape ||
				descriptor.PipelineShape != tt.pipelineShape ||
				descriptor.ValueExpr != tt.valueExpr ||
				descriptor.IndexExpr != tt.indexExpr {
				t.Fatalf("descriptor = %#v, want shape=%q value=%q index=%q", descriptor, tt.shape, tt.valueExpr, tt.indexExpr)
			}
			out, handled, err := ExecuteEvalPipelineDescriptor(descriptor)
			if err != nil || !handled {
				t.Fatalf("ExecuteEvalPipelineDescriptor = %#v,%v,%v", out, handled, err)
			}
			if !qEvalPipelineTestValueEqual(out, tt.want) {
				t.Fatalf("ExecuteEvalPipelineDescriptor output = %#v, want %#v", out, tt.want)
			}

			seenPipeline := false
			seenApplyKernel := false
			for _, stat := range RuntimeKernelExecutionStats() {
				if stat.Outcome == "fallback" || stat.Outcome == "error" {
					t.Fatalf("unexpected scalar apply-index fallback/error: %#v all=%#v", stat, RuntimeKernelExecutionStats())
				}
				if stat.Kernel == "QScriptPipelinePlan" && stat.Shape == tt.shape && stat.Outcome == "hit" {
					seenPipeline = true
				}
				if (stat.Kernel == "ArrayScalarIndex" || stat.Kernel == "ArrayGatherIndex" || stat.Kernel == "ArrayMatrixRowIndex" || stat.Kernel == "MatrixIndex") && stat.Outcome == "hit" {
					seenApplyKernel = true
				}
			}
			if !seenPipeline || !seenApplyKernel {
				t.Fatalf("missing apply-index pipeline/kernel stats: pipeline=%v kernel=%v all=%#v", seenPipeline, seenApplyKernel, RuntimeKernelExecutionStats())
			}
		})
	}
}

func qEvalPipelineTestValueEqual(got, want any) bool {
	if gotArray, ok := got.(data.Array); ok {
		wantArray, ok := want.(data.Array)
		if !ok {
			return false
		}
		return gotArray.Kind() == wantArray.Kind() &&
			reflect.DeepEqual(normalizeNestedArrayValues(gotArray.Values()), normalizeNestedArrayValues(wantArray.Values()))
	}
	return reflect.DeepEqual(got, want)
}

func TestQPipelinePlanRecognizesRuntimePrimitiveStatsAndWindows(t *testing.T) {
	tests := []struct {
		expr          string
		shape         string
		pipelineShape string
		transform     string
		want          any
	}{
		{expr: "svar 1 2 3", shape: "runtime-unary/svar", pipelineShape: "numeric_stats", transform: "svar", want: 1.0},
		{expr: "sdev 1 2 3", shape: "runtime-unary/sdev", pipelineShape: "numeric_stats", transform: "sdev", want: 1.0},
		{expr: "wsum 1 2 3", shape: "runtime-unary/wsum", pipelineShape: "numeric_stats", transform: "wsum", want: int64(6)},
		{expr: "sqrt 4 9 16", shape: "runtime-unary/sqrt", pipelineShape: "numeric_math", transform: "sqrt", want: data.NewF64([]float64{2, 3, 4})},
		{expr: "sin 0", shape: "runtime-unary/sin", pipelineShape: "numeric_math", transform: "sin", want: 0.0},
		{expr: "signum -2 0 3", shape: "runtime-unary/signum", pipelineShape: "numeric_math", transform: "signum", want: data.NewI64([]int64{-1, 0, 1})},
		{expr: "floor 1.9", shape: "runtime-unary/floor", pipelineShape: "numeric_math", transform: "floor", want: int64(1)},
		{expr: "ceiling 1.1", shape: "runtime-unary/ceiling", pipelineShape: "numeric_math", transform: "ceiling", want: int64(2)},
		{expr: "2 xexp 3 4", shape: "runtime-dyadic/xexp", pipelineShape: "numeric_math", transform: "xexp", want: data.NewF64([]float64{8, 16})},
		{expr: "2 xlog 8", shape: "runtime-dyadic/xlog", pipelineShape: "numeric_math", transform: "xlog", want: 3.0},
		{expr: "1 2 3 wsum 10 20 30", shape: "runtime-dyadic/wsum", pipelineShape: "numeric_stats", transform: "wsum", want: int64(140)},
		{expr: "1 2 3 cov 1 2 3", shape: "runtime-dyadic/cov", pipelineShape: "numeric_stats", transform: "cov", want: float64(2) / 3},
		{expr: "1 2 3 scov 1 2 3", shape: "runtime-dyadic/scov", pipelineShape: "numeric_stats", transform: "scov", want: 1.0},
		{expr: "1 2 3 cor 1 2 3", shape: "runtime-dyadic/cor", pipelineShape: "numeric_stats", transform: "cor", want: 1.0},
		{expr: "2 mdev 1 2 3", shape: "runtime-dyadic/mdev", pipelineShape: "window_scan", transform: "mdev", want: data.NewF64([]float64{0, 0.5, 0.5})},
		{expr: "0.5 ema 1 2 3", shape: "runtime-dyadic/ema", pipelineShape: "window_scan", transform: "ema", want: data.NewF64([]float64{1, 1.5, 2.25})},
		{expr: "10 20 30 40 where 1010b", shape: "runtime-dyadic/where", pipelineShape: "runtime_primitive", transform: "where", want: data.NewI64([]int64{10, 30})},
	}
	for _, tt := range tests {
		t.Run(tt.shape, func(t *testing.T) {
			ClearRuntimeKernelExecutionStats()
			t.Cleanup(ClearRuntimeKernelExecutionStats)

			descriptor, ok := DescribeEvalPipeline(tt.expr)
			if !ok {
				t.Fatalf("DescribeEvalPipeline(%q) did not recognize runtime primitive", tt.expr)
			}
			if descriptor.Shape != tt.shape ||
				descriptor.PipelineShape != tt.pipelineShape ||
				descriptor.ShapeTransform != tt.transform {
				t.Fatalf("descriptor = %#v, want shape=%q pipeline=%q transform=%q", descriptor, tt.shape, tt.pipelineShape, tt.transform)
			}
			out, handled, err := ExecuteEvalPipelineDescriptor(descriptor)
			if err != nil || !handled {
				t.Fatalf("ExecuteEvalPipelineDescriptor = %#v,%v,%v", out, handled, err)
			}
			if !qEvalPipelineTestValueEqual(out, tt.want) {
				t.Fatalf("ExecuteEvalPipelineDescriptor output = %#v, want %#v", out, tt.want)
			}

			seenPipeline := false
			seenPrimitive := tt.pipelineShape == "numeric_stats" || tt.pipelineShape == "window_scan"
			for _, stat := range RuntimeKernelExecutionStats() {
				if stat.Outcome == "fallback" || stat.Outcome == "error" {
					t.Fatalf("unexpected runtime fallback/error: %#v all=%#v", stat, RuntimeKernelExecutionStats())
				}
				if stat.Kernel == "QPipelinePlan" && stat.Shape == tt.shape && stat.Outcome == "hit" {
					seenPipeline = true
				}
				if stat.Kernel == "QRuntimePrimitive" && stat.Outcome == "hit" {
					seenPrimitive = true
				}
			}
			if !seenPipeline || !seenPrimitive {
				t.Fatalf("missing runtime primitive stats: pipeline=%v primitive=%v all=%#v", seenPipeline, seenPrimitive, RuntimeKernelExecutionStats())
			}
		})
	}

	if descriptor, ok := DescribeEvalPipeline("floor 1.9 -1.2 0N"); ok {
		t.Fatalf("compound prefix expression unexpectedly recognized as runtime primitive: %#v", descriptor)
	}
}

func TestQPipelineExecutablePlanCoversRuntimeDyadicWhere(t *testing.T) {
	src := "10 20 30 40 where 1010b"
	backend, ok := DescribeEvalPipelineBackendPlan(src)
	if !ok {
		t.Fatalf("DescribeEvalPipelineBackendPlan(%q) did not recognize dyadic where", src)
	}
	if backend.Descriptor.Kind != "expression" ||
		backend.Descriptor.Shape != "runtime-dyadic/where" ||
		backend.Descriptor.LeftExpr != "10 20 30 40" ||
		backend.Descriptor.RightExpr != "1010b" {
		t.Fatalf("descriptor = %#v, want runtime-dyadic where expression fields", backend.Descriptor)
	}
	executable, ok := CompileEvalPipelineBackendPlan(backend)
	if !ok {
		t.Fatalf("CompileEvalPipelineBackendPlan(%q) failed", src)
	}
	out, handled, err := NewEvalState(nil).ExecuteEvalPipelineExecutablePlan(executable)
	if err != nil || !handled {
		t.Fatalf("ExecuteEvalPipelineExecutablePlan = %#v,%v,%v", out, handled, err)
	}
	if !qEvalPipelineTestValueEqual(out, data.NewI64([]int64{10, 30})) {
		t.Fatalf("ExecuteEvalPipelineExecutablePlan output = %#v, want 10 30", out)
	}
}

func TestEvalPipelineDescriptorExecutionUsesExecutablePlan(t *testing.T) {
	src := "10 20 30 40 where 1010b"
	backend, ok := DescribeEvalPipelineBackendPlan(src)
	if !ok {
		t.Fatalf("DescribeEvalPipelineBackendPlan(%q) failed", src)
	}
	descriptorOut, descriptorHandled, descriptorErr := ExecuteEvalPipelineDescriptor(backend.Descriptor)
	if descriptorErr != nil || !descriptorHandled {
		t.Fatalf("ExecuteEvalPipelineDescriptor = %#v,%v,%v", descriptorOut, descriptorHandled, descriptorErr)
	}
	executable, ok := CompileEvalPipelineDescriptor(backend.Descriptor)
	if !ok {
		t.Fatalf("CompileEvalPipelineDescriptor(%q) failed", src)
	}
	executableOut, executableHandled, executableErr := NewEvalState(nil).ExecuteEvalPipelineExecutablePlan(executable)
	if executableErr != nil || !executableHandled {
		t.Fatalf("ExecuteEvalPipelineExecutablePlan = %#v,%v,%v", executableOut, executableHandled, executableErr)
	}
	if !qEvalPipelineTestValueEqual(descriptorOut, executableOut) {
		t.Fatalf("descriptor/executable mismatch: %#v vs %#v", descriptorOut, executableOut)
	}
}

func TestEvalPipelineExecutablePlanCloneUsesRunnerWithoutPayload(t *testing.T) {
	src := "10 20 30 40 where 1010b"
	backend, ok := DescribeEvalPipelineBackendPlan(src)
	if !ok {
		t.Fatalf("DescribeEvalPipelineBackendPlan(%q) failed", src)
	}
	executable, ok := CompileEvalPipelineBackendPlan(backend)
	if !ok {
		t.Fatalf("CompileEvalPipelineBackendPlan(%q) failed", src)
	}
	cloned := cloneEvalPipelineExecutablePlan(executable)
	if !cloned.Valid() {
		t.Fatalf("cloned opaque executable is invalid: %#v", cloned)
	}
	out, handled, err := NewEvalState(nil).ExecuteEvalPipelineExecutablePlanRef(&cloned)
	if err != nil || !handled {
		t.Fatalf("ExecuteEvalPipelineExecutablePlanRef cloned opaque executable = %#v,%v,%v", out, handled, err)
	}
	if !qEvalPipelineTestValueEqual(out, data.NewI64([]int64{10, 30})) {
		t.Fatalf("cloned opaque executable output = %#v, want 10 30", out)
	}
}

func TestEvalPipelineExecutablePlanMetadataComesFromRunner(t *testing.T) {
	src := "10 20 30 40 where 1010b"
	backend, ok := DescribeEvalPipelineBackendPlan(src)
	if !ok {
		t.Fatalf("DescribeEvalPipelineBackendPlan(%q) failed", src)
	}
	executable, ok := CompileEvalPipelineBackendPlan(backend)
	if !ok {
		t.Fatalf("CompileEvalPipelineBackendPlan(%q) failed", src)
	}
	if executable.Backend() != EvalPipelineTypedRuntimeBackend || executable.Kind() != evalPipelineKindExpression {
		t.Fatalf("executable metadata = backend %q kind %q, want typed runtime expression", executable.Backend(), executable.Kind())
	}
	invalidRunner := evalPipelineExecutablePlanForRunner(&qEvalPipelineExpressionExecutable{})
	if invalidRunner.Valid() {
		t.Fatalf("invalid expression runner produced valid executable")
	}
}

func TestEvalSessionCachesExpressionExecutablePlan(t *testing.T) {
	session := NewEvalSession(nil)
	src := "10 20 30 40 where 1010b"
	out, err := session.Eval(src)
	if err != nil {
		t.Fatalf("EvalSession.Eval: %v", err)
	}
	if !qEvalPipelineTestValueEqual(out, data.NewI64([]int64{10, 30})) {
		t.Fatalf("EvalSession.Eval = %#v, want 10 30", out)
	}
	entry := session.cache[src]
	if entry == nil || !entry.executable.Valid() || entry.descriptor.Kind != "expression" {
		t.Fatalf("session cache entry = %#v, want expression executable plan", entry)
	}
}

func TestEvalSessionMultiStatementFallbackDoesNotCompileAsExpression(t *testing.T) {
	session := NewEvalSession(nil)
	src := "x:til 8192;idx:where x>=0;+/idx"
	got, err := session.Eval(src)
	if err != nil || got != int64(33550336) {
		t.Fatalf("EvalSession.Eval = %#v,%v; want 33550336,nil", got, err)
	}
	entry := session.cache[src]
	if entry == nil {
		t.Fatalf("session cache missing %q", src)
	}
	if entry.executable.Valid() {
		t.Fatalf("session cache executable = %#v, want ordinary script fallback", entry.executable)
	}
}

func TestQPipelinePlanRecognizesNumericUnaryComposedShapes(t *testing.T) {
	tests := []struct {
		name          string
		src           string
		shape         string
		pipelineShape string
		transform     string
		want          any
	}{
		{
			name:          "sum unary",
			src:           "+/sqrt 1 4 9 16",
			shape:         "vector-reduce/sum-unary-sqrt",
			pipelineShape: "vector_reduce",
			transform:     "sqrt",
			want:          float64(10),
		},
		{
			name:          "where unary compare",
			src:           "where sqrt 1 4 9 16>2",
			shape:         "numeric-unary-compare-to-index/sqrt",
			pipelineShape: "compare_index",
			transform:     "sqrt",
			want:          data.NewI64([]int64{2, 3}),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ClearRuntimeKernelExecutionStats()
			t.Cleanup(ClearRuntimeKernelExecutionStats)

			descriptor, ok := DescribeEvalPipeline(tt.src)
			if !ok {
				t.Fatalf("DescribeEvalPipeline(%q) did not recognize numeric unary composed shape", tt.src)
			}
			if descriptor.Shape != tt.shape ||
				descriptor.PipelineShape != tt.pipelineShape ||
				descriptor.ShapeFamily == "" ||
				descriptor.ShapeTransform != tt.transform ||
				descriptor.UnaryOp != tt.transform {
				t.Fatalf("descriptor = %#v, want shape=%q pipeline=%q transform=%q", descriptor, tt.shape, tt.pipelineShape, tt.transform)
			}

			backend, ok := DescribeEvalPipelineBackendPlan(tt.src)
			if !ok {
				t.Fatalf("DescribeEvalPipelineBackendPlan(%q) failed", tt.src)
			}
			executable, ok := CompileEvalPipelineBackendPlan(backend)
			if !ok {
				t.Fatalf("CompileEvalPipelineBackendPlan(%q) failed", tt.src)
			}
			out, handled, err := NewEvalState(nil).ExecuteEvalPipelineExecutablePlan(executable)
			if err != nil || !handled {
				t.Fatalf("ExecuteEvalPipelineExecutablePlan = %#v,%v,%v", out, handled, err)
			}
			if !qEvalPipelineTestValueEqual(out, tt.want) {
				t.Fatalf("ExecuteEvalPipelineExecutablePlan output = %#v, want %#v", out, tt.want)
			}

			seenPipeline := false
			seenTyped := false
			for _, stat := range RuntimeKernelExecutionStats() {
				if stat.Outcome == "fallback" || stat.Outcome == "error" {
					t.Fatalf("unexpected runtime fallback/error: %#v all=%#v", stat, RuntimeKernelExecutionStats())
				}
				if stat.Kernel == "QPipelinePlan" && stat.Shape == tt.shape && stat.Outcome == "hit" {
					seenPipeline = true
				}
				if (stat.Kernel == "ArrayNumericUnarySum" || stat.Kernel == "ArrayNumericUnaryCompareIndexes") && stat.Outcome == "hit" {
					seenTyped = true
				}
			}
			if !seenPipeline || !seenTyped {
				t.Fatalf("missing numeric unary composed stats: pipeline=%v typed=%v all=%#v", seenPipeline, seenTyped, RuntimeKernelExecutionStats())
			}
		})
	}
}

func TestQNumericUnaryMultiSumHandlesDenseIntegerAndCrossZeroRange(t *testing.T) {
	ClearRuntimeKernelExecutionStats()
	t.Cleanup(ClearRuntimeKernelExecutionStats)

	got, err := NewEvalState(nil).Eval("x:(til 16)-8;(+/abs x)+(+/neg x)+(+/signum x)")
	if err != nil {
		t.Fatalf("Eval(numeric unary multi-sum cross-zero range) returned error: %v", err)
	}
	if got != int64(71) {
		t.Fatalf("Eval(numeric unary multi-sum cross-zero range) = %#v, want 71", got)
	}
	seenHit := false
	for _, stat := range RuntimeKernelExecutionStats() {
		if stat.Kernel != "ArrayNumericUnaryMultiSum" {
			continue
		}
		if stat.Outcome == "fallback" || stat.Outcome == "error" {
			t.Fatalf("unexpected ArrayNumericUnaryMultiSum fallback/error: %#v all=%#v", stat, RuntimeKernelExecutionStats())
		}
		if stat.Outcome == "hit" {
			seenHit = true
		}
	}
	if !seenHit {
		t.Fatalf("missing ArrayNumericUnaryMultiSum hit; stats=%#v", RuntimeKernelExecutionStats())
	}
}

func TestQPipelinePlanRecognizesCastEnvelope(t *testing.T) {
	src := "(`long$3.7)+(\"J\"$\"42\")+(\"I\"$\"17\")+(count `$\"AAPL\")+(count string `$\"MSFT\")"
	ClearRuntimeKernelExecutionStats()
	t.Cleanup(ClearRuntimeKernelExecutionStats)

	descriptor, ok := DescribeEvalPipeline(src)
	if !ok {
		t.Fatalf("DescribeEvalPipeline(%q) did not recognize cast envelope", src)
	}
	if descriptor.Shape != "cast-envelope/sum" ||
		descriptor.PipelineShape != "cast" ||
		descriptor.ShapeFamily != "cast" ||
		descriptor.ShapeReducer != "sum" ||
		descriptor.ShapeTransform != "typed-cast" {
		t.Fatalf("descriptor = %#v, want cast sum descriptor", descriptor)
	}
	if len(descriptor.CastTerms) != 5 {
		t.Fatalf("descriptor cast terms = %#v, want 5 terms", descriptor.CastTerms)
	}
	if !descriptor.CastTerms[3].Count || !descriptor.CastTerms[4].Count || !descriptor.CastTerms[4].StringCast {
		t.Fatalf("descriptor cast term flags = %#v", descriptor.CastTerms)
	}

	out, handled, err := ExecuteEvalPipelineDescriptor(descriptor)
	if err != nil || !handled || out != int64(68) {
		t.Fatalf("ExecuteEvalPipelineDescriptor = %#v,%v,%v; want 68,true,nil", out, handled, err)
	}
	backend, ok := DescribeEvalPipelineBackendPlan(src)
	if !ok {
		t.Fatalf("DescribeEvalPipelineBackendPlan(%q) did not recognize cast envelope", src)
	}
	executable, ok := CompileEvalPipelineBackendPlan(backend)
	if !ok {
		t.Fatalf("CompileEvalPipelineBackendPlan(%#v) failed", backend)
	}
	out, handled, err = NewEvalState(nil).ExecuteEvalPipelineExecutablePlan(executable)
	if err != nil || !handled || out != int64(68) {
		t.Fatalf("ExecuteEvalPipelineExecutablePlan = %#v,%v,%v; want 68,true,nil", out, handled, err)
	}

	seenPipeline := false
	var seenCast uint64
	for _, stat := range RuntimeKernelExecutionStats() {
		if stat.Outcome == "fallback" || stat.Outcome == "error" {
			t.Fatalf("unexpected cast pipeline fallback/error: %#v all=%#v", stat, RuntimeKernelExecutionStats())
		}
		if stat.Kernel == "QPipelinePlan" && stat.Shape == "cast-envelope/sum" && stat.Outcome == "hit" {
			seenPipeline = true
		}
		if stat.Kernel == "QCastPrimitive" && stat.Outcome == "hit" {
			seenCast += stat.Count
		}
	}
	if !seenPipeline || seenCast < 10 {
		t.Fatalf("missing cast runtime stats: pipeline=%v castHits=%d all=%#v", seenPipeline, seenCast, RuntimeKernelExecutionStats())
	}
}

func TestEvalAdverbArithmeticUsesTypedRuntime(t *testing.T) {
	tests := []struct {
		expr string
		want any
	}{
		{"x:til 8;(+/100-\\:x)+(+/x-/:100)", int64(0)},
		{"x:til 8;y:(x*2)+1;+/x+'y", int64(92)},
	}
	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			ClearRuntimeKernelExecutionStats()
			t.Cleanup(ClearRuntimeKernelExecutionStats)

			assertEvalValue(t, tt.expr, tt.want)
			seenAdverb := false
			for _, stat := range RuntimeKernelExecutionStats() {
				if stat.Kernel == "QAdverbArithmetic" && stat.Outcome == "hit" && stat.Count > 0 {
					seenAdverb = true
				}
				if stat.Kernel == "QAdverbArithmetic" && (stat.Outcome == "fallback" || stat.Outcome == "error") {
					t.Fatalf("unexpected adverb arithmetic fallback/error: %#v all=%#v", stat, RuntimeKernelExecutionStats())
				}
			}
			if !seenAdverb {
				t.Fatalf("missing QAdverbArithmetic hit: %#v", RuntimeKernelExecutionStats())
			}
		})
	}
}

func TestEvalCallableOverScalarTermUsesTypedRuntime(t *testing.T) {
	ClearRuntimeKernelExecutionStats()
	t.Cleanup(ClearRuntimeKernelExecutionStats)

	assertEvalValue(t, "({x+y}/[10;1 2 3])+4", int64(20))
	seen := false
	for _, stat := range RuntimeKernelExecutionStats() {
		if stat.Kernel == "CallableOverScalar" && stat.Outcome == "hit" && stat.Count > 0 {
			seen = true
		}
	}
	if !seen {
		t.Fatalf("missing CallableOverScalar hit: %#v", RuntimeKernelExecutionStats())
	}
}

func TestEvalLastDyadicTerminalUsesTypedRuntime(t *testing.T) {
	ClearRuntimeKernelExecutionStats()
	t.Cleanup(ClearRuntimeKernelExecutionStats)

	assertEvalValue(t, "x:1+til 8;s:+\\x;last (s+count s)", int64(44))
	seen := false
	for _, stat := range RuntimeKernelExecutionStats() {
		if stat.Kernel == "QTerminalLastDyadic" && stat.Outcome == "hit" && stat.Count > 0 {
			seen = true
		}
	}
	if !seen {
		t.Fatalf("missing QTerminalLastDyadic hit: %#v", RuntimeKernelExecutionStats())
	}
}

func TestQPipelineShapeRegistryStabilizesExpressionPlans(t *testing.T) {
	tests := []struct {
		name      string
		source    string
		shape     string
		family    qPipelineShapeFamily
		reducer   string
		selector  string
		transform string
		pipeline  string
	}{
		{
			name:     "where mask reduce",
			source:   "+/y where x>8",
			shape:    "where-reduce/sum",
			family:   qPipelineShapeFamilyWhere,
			reducer:  "sum",
			selector: "mask",
			pipeline: "mask_reduce",
		},
		{
			name:     "gather reduce",
			source:   "+/y[idx]",
			shape:    "gather-reduce/sum",
			family:   qPipelineShapeFamilyGather,
			reducer:  "sum",
			selector: "index",
			pipeline: "gather_reduce",
		},
		{
			name:      "vector transform reduce",
			source:    "sum reverse 8#til 4",
			shape:     "vector-reduce/sum-reverse",
			family:    qPipelineShapeFamilyVector,
			reducer:   "sum",
			transform: "reverse",
			pipeline:  "vector_reduce",
		},
		{
			name:      "vector transform count",
			source:    "count 8#til 4",
			shape:     "vector-count/expr",
			family:    qPipelineShapeFamilyVector,
			reducer:   "count",
			transform: "expr",
			pipeline:  "vector_scan",
		},
		{
			name:      "sequence primitive count",
			source:    "count (til 16) cross til 16",
			shape:     "sequence-count/cross",
			family:    qPipelineShapeFamilyVector,
			reducer:   "count",
			transform: "cross",
			pipeline:  "sequence_count",
		},
		{
			name:      "deltas reduce",
			source:    "sum deltas til 8",
			shape:     "vector-reduce/sum-deltas",
			family:    qPipelineShapeFamilyVector,
			reducer:   "sum",
			transform: "deltas",
			pipeline:  "vector_reduce",
		},
		{
			name:      "running scan terminal",
			source:    "last sums til 8",
			shape:     "vector-last/sums",
			family:    qPipelineShapeFamilyVector,
			reducer:   "last",
			transform: "sums",
			pipeline:  "vector_scan",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := buildQPipelinePlan(tt.source)
			if plan.kind == qPipelineInvalid {
				t.Fatalf("plan was not recognized")
			}
			spec := qPipelinePlanShapeSpec(plan)
			if got := spec.stableID(); got != tt.shape {
				t.Fatalf("stable shape = %q, want %q", got, tt.shape)
			}
			if spec.Family != tt.family || spec.Reducer != tt.reducer || spec.Selector != tt.selector || spec.Transform != tt.transform || spec.PipelineShape != tt.pipeline {
				t.Fatalf("shape spec = %#v, want family=%q reducer=%q selector=%q transform=%q pipeline=%q",
					spec, tt.family, tt.reducer, tt.selector, tt.transform, tt.pipeline)
			}
			if plan.stableShape() != tt.shape || plan.stablePipelineShape() != tt.pipeline {
				t.Fatalf("plan stable shape = %q/%q, want %q/%q", plan.stableShape(), plan.stablePipelineShape(), tt.shape, tt.pipeline)
			}
		})
	}
}

func TestQPipelineLastRunningScanEvalAndDescriptor(t *testing.T) {
	tests := []struct {
		expr string
		want any
	}{
		{"last sums 1 2 3 4", int64(10)},
		{"last prds 2 3 4", int64(24)},
		{"last mins 9 3 5", int64(3)},
		{"last maxs 9 3 5", int64(9)},
		{"last avgs 2 4 9", 5.0},
	}
	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			ClearRuntimeKernelExecutionStats()
			t.Cleanup(ClearRuntimeKernelExecutionStats)

			assertEvalValue(t, tt.expr, tt.want)
			descriptor, ok := DescribeEvalPipeline(tt.expr)
			if !ok {
				t.Fatalf("DescribeEvalPipeline(%q) did not recognize running last", tt.expr)
			}
			if descriptor.ShapeFamily != "vector" || descriptor.ShapeReducer != "last" || descriptor.ShapeTransform == "" || descriptor.PipelineShape != "vector_scan" {
				t.Fatalf("descriptor = %#v, want vector last scan", descriptor)
			}
			out, handled, err := ExecuteEvalPipelineDescriptor(descriptor)
			if err != nil || !handled {
				t.Fatalf("ExecuteEvalPipelineDescriptor = %v,%v,%v", out, handled, err)
			}
			if !reflect.DeepEqual(out, tt.want) {
				t.Fatalf("ExecuteEvalPipelineDescriptor output = %#v, want %#v", out, tt.want)
			}

			seenPipeline := false
			seenLastScan := false
			for _, stat := range RuntimeKernelExecutionStats() {
				if stat.Kernel == "QPipelinePlan" && stat.Shape == descriptor.Shape && stat.Outcome == "hit" && stat.Count > 0 {
					seenPipeline = true
				}
				if stat.Kernel == "ArrayLastScan" && stat.Outcome == "hit" && stat.Count > 0 {
					seenLastScan = true
				}
			}
			if !seenPipeline || !seenLastScan {
				t.Fatalf("missing runtime stats: pipeline=%v lastScan=%v stats=%#v", seenPipeline, seenLastScan, RuntimeKernelExecutionStats())
			}
		})
	}
}

func TestQScriptPipelineCachesBoundModuloMaskPlan(t *testing.T) {
	plan := buildQScriptPlan("x:til 12;y:x*2;idx:where (x mod 3)=0;+/y[idx]")
	if plan.scriptPipeline == nil {
		t.Fatalf("script pipeline descriptor missing")
	}
	d := plan.scriptPipeline
	if d.kind != qScriptPipelineWhereIndexReduceSum {
		t.Fatalf("pipeline kind = %q, want %q", d.kind, qScriptPipelineWhereIndexReduceSum)
	}
	if d.moduloMaskPlan == nil {
		t.Fatalf("modulo mask sub-plan was not cached: %#v", d)
	}
	if d.moduloMaskPlan.modPlan.kind == qScriptBindingInvalid ||
		d.moduloMaskPlan.modulusPlan.kind == qScriptBindingInvalid ||
		d.moduloMaskPlan.modTargetPlan.kind == qScriptBindingInvalid {
		t.Fatalf("modulo mask sub-plan was not bound: %#v", d.moduloMaskPlan)
	}
	if len(d.assignments) == 0 || !qScriptPipelineCanDeferAssignment(d, d.assignments[len(d.assignments)-1]) {
		t.Fatalf("script pipeline did not use cached modulo sub-plan for deferred assignment: %#v", d.assignments)
	}
}

func TestQScriptPipelineDescriptorRestoreNormalizesTerminalPlan(t *testing.T) {
	descriptor, ok := DescribeEvalPipeline("x:til 12;y:x*2;idx:where (x mod 3)=0;+/y[idx]")
	if !ok {
		t.Fatalf("DescribeEvalPipeline did not recognize script pipeline")
	}
	restored, ok := qScriptPipelineDescriptorFromEvalDescriptor(descriptor)
	if !ok {
		t.Fatalf("qScriptPipelineDescriptorFromEvalDescriptor failed")
	}
	if restored.terminalPlan.kind != qPipelineSumWhereIndex {
		t.Fatalf("restored terminal plan kind = %v, want qPipelineSumWhereIndex", restored.terminalPlan.kind)
	}
	if restored.terminalPlan.moduloMaskPlan == nil || restored.terminalPlan.moduloMaskPlan.modPlan.kind == qScriptBindingInvalid {
		t.Fatalf("restored terminal modulo sub-plan missing or unbound: %#v", restored.terminalPlan.moduloMaskPlan)
	}
	if restored.shape() != descriptor.Shape {
		t.Fatalf("restored shape = %q, want %q", restored.shape(), descriptor.Shape)
	}
}

func TestQScriptPipelinePlannerDescribesTerminalWhereReduce(t *testing.T) {
	plan := buildQScriptPlan("x:til 64;y:x*2;lo:4;hi:12;+/y where (x>=lo) and x<hi")
	if plan.scriptPipeline == nil {
		t.Fatalf("script pipeline descriptor missing")
	}
	d := plan.scriptPipeline
	if d.kind != qScriptPipelineWhereReduceSum {
		t.Fatalf("pipeline kind = %q, want %q", d.kind, qScriptPipelineWhereReduceSum)
	}
	if !d.terminalUsesWhere {
		t.Fatalf("terminalUsesWhere = false, want true")
	}
	if d.valueExpr != "y" || d.valueBinding != "x*2" {
		t.Fatalf("value descriptor = expr %q binding %q, want y/x*2", d.valueExpr, d.valueBinding)
	}
	if d.maskExpr != "(x>=lo) and x<hi" {
		t.Fatalf("mask expr = %q, want compound mask", d.maskExpr)
	}
}

func TestQScriptPipelinePlannerDescribesTerminalGatherReduce(t *testing.T) {
	plan := buildQScriptPlan("x:til 64;y:x*2;idx:4 5 6;+/y[idx]")
	if plan.scriptPipeline == nil {
		t.Fatalf("script pipeline descriptor missing")
	}
	d := plan.scriptPipeline
	if d.kind != qScriptPipelineGatherReduceSum {
		t.Fatalf("pipeline kind = %q, want %q", d.kind, qScriptPipelineGatherReduceSum)
	}
	if d.valueExpr != "y" || d.valueBinding != "x*2" {
		t.Fatalf("value descriptor = expr %q binding %q, want y/x*2", d.valueExpr, d.valueBinding)
	}
	if d.indexExpr != "idx" || d.indexBinding != "4 5 6" {
		t.Fatalf("index descriptor = expr %q binding %q, want idx/4 5 6", d.indexExpr, d.indexBinding)
	}
	if d.maskExpr != "" || d.maskBinding != "" {
		t.Fatalf("mask descriptor = expr %q binding %q, want empty", d.maskExpr, d.maskBinding)
	}
	if got, want := d.shape(), "script-pipeline/gather-reduce/sum/assignments"; got != want {
		t.Fatalf("shape = %q, want %q", got, want)
	}
}

func TestQScriptPipelinePlannerDescribesIndexExprSumCount(t *testing.T) {
	plan := buildQScriptPlan("x:til 16;idx:where (x>=4) and x<8;pxi:10+(idx mod 5);szi:1+(idx mod 3);(+/(pxi*szi))+count idx")
	if plan.scriptPipeline == nil {
		t.Fatalf("script pipeline descriptor missing")
	}
	d := plan.scriptPipeline
	if d.kind != qScriptPipelineIndexExprSumCount {
		t.Fatalf("pipeline kind = %q, want %q", d.kind, qScriptPipelineIndexExprSumCount)
	}
	if d.indexExpr != "idx" || d.indexBinding != "where (x>=4) and x<8" {
		t.Fatalf("index descriptor = expr %q binding %q, want idx/where", d.indexExpr, d.indexBinding)
	}
	if stripEnclosingParens(d.valueExpr) != "pxi*szi" {
		t.Fatalf("value expr = %q, want pxi*szi", d.valueExpr)
	}
	if got, want := d.shape(), "script-pipeline/index-expr-reduce/sum-count/assignments"; got != want {
		t.Fatalf("shape = %q, want %q", got, want)
	}
}

func TestQScriptPipelinePlannerDescribesIndexExprMultiReducers(t *testing.T) {
	plan := buildQScriptPlan("x:til 16;idx:where x>=8;pxi:10+(idx mod 5);szi:1+(idx mod 3);(+/(pxi*szi))+(+/pxi)+count idx")
	if plan.scriptPipeline == nil {
		t.Fatalf("script pipeline descriptor missing")
	}
	d := plan.scriptPipeline
	if d.kind != qScriptPipelineIndexExprSumCount {
		t.Fatalf("pipeline kind = %q, want %q", d.kind, qScriptPipelineIndexExprSumCount)
	}
	if len(d.indexReducers) != 3 {
		t.Fatalf("index reducer count = %d, want 3", len(d.indexReducers))
	}
	if got, want := d.shape(), "script-pipeline/index-expr-reduce/sum2-count/assignments"; got != want {
		t.Fatalf("shape = %q, want %q", got, want)
	}
}

func TestQScriptPipelinePlannerDescribesIndexExprComputedProjectionReducers(t *testing.T) {
	src := "px:100+((til 16) mod 90);sz:1+((til 16) mod 64);idx:where sz>=8;pxi:100+(idx mod 90);szi:1+(idx mod 64);(+/(pxi*szi))+(+/(count idx)#2)+(+/10 xbar pxi)+count idx"
	plan := buildQScriptPlan(src)
	if plan.scriptPipeline == nil {
		t.Fatalf("script pipeline descriptor missing")
	}
	d := plan.scriptPipeline
	if d.kind != qScriptPipelineIndexExprSumCount {
		t.Fatalf("pipeline kind = %q, want %q", d.kind, qScriptPipelineIndexExprSumCount)
	}
	if len(d.indexReducers) != 4 {
		t.Fatalf("index reducer count = %d, want 4", len(d.indexReducers))
	}
	if got, want := d.shape(), "script-pipeline/index-expr-reduce/sum3-count/assignments"; got != want {
		t.Fatalf("shape = %q, want %q", got, want)
	}
}

func TestQScriptPipelinePlannerDescribesSequenceEdgeReduce(t *testing.T) {
	src := "x:til 1024;r:17 rotate x;y:128 sublist reverse r;(+/y)+first y+last y"
	plan := buildQScriptPlan(src)
	if plan.scriptPipeline == nil {
		t.Fatalf("script pipeline descriptor missing")
	}
	d := plan.scriptPipeline
	if d.kind != qScriptPipelineSequenceEdgeSum {
		t.Fatalf("pipeline kind = %q, want %q", d.kind, qScriptPipelineSequenceEdgeSum)
	}
	if d.valueExpr != "y" || d.valueBinding != "128 sublist reverse r" {
		t.Fatalf("value descriptor = expr %q binding %q, want y/128 sublist reverse r", d.valueExpr, d.valueBinding)
	}
	if d.sequenceValueExpr != "x" ||
		qScriptPipelineSequenceTransformName(d.sequenceSteps) != "rotate.reverse.sublist" ||
		encodeQScriptPipelineSequenceTransformSteps(d.sequenceSteps) != "rotate:17|reverse|sublist:0,128" ||
		encodeQScriptPipelineNames(d.sequenceBindings) != "r\x1fy" {
		t.Fatalf("sequence chain descriptor = base %q steps %#v names %#v", d.sequenceValueExpr, d.sequenceSteps, d.sequenceBindings)
	}
	if got, want := d.shape(), "script-pipeline/sequence-edge-reduce/sum-first-last-transform-chain/assignments"; got != want {
		t.Fatalf("shape = %q, want %q", got, want)
	}
	descriptor, ok := DescribeEvalPipeline(src)
	if !ok {
		t.Fatalf("DescribeEvalPipeline(%q) did not recognize sequence edge pipeline", src)
	}
	if descriptor.SequenceValueExpr != "x" ||
		descriptor.SequenceTransformChain != "rotate:17|reverse|sublist:0,128" ||
		descriptor.SequenceTransformNames != "r\x1fy" ||
		descriptor.ShapeTransform != "rotate.reverse.sublist" {
		t.Fatalf("eval descriptor sequence chain = %#v", descriptor)
	}
	out, handled, err := ExecuteEvalPipelineDescriptor(descriptor)
	if err != nil || !handled || out != int64(108513) {
		t.Fatalf("ExecuteEvalPipelineDescriptor sequence edge = %#v,%v,%v; want 108513,true,nil", out, handled, err)
	}
}

func TestQScriptPipelinePlannerDescribesSequenceSumCountReduce(t *testing.T) {
	src := "x:til 4096;y:1024#drop 128 x;z:512 sublist y;(+/z)+count z"
	plan := buildQScriptPlan(src)
	if plan.scriptPipeline == nil {
		t.Fatalf("script pipeline descriptor missing")
	}
	d := plan.scriptPipeline
	if d.kind != qScriptPipelineSequenceSumCount {
		t.Fatalf("pipeline kind = %q, want %q", d.kind, qScriptPipelineSequenceSumCount)
	}
	if d.sequenceValueExpr != "x" ||
		qScriptPipelineSequenceTransformName(d.sequenceSteps) != "drop.sublist.sublist" ||
		encodeQScriptPipelineSequenceTransformSteps(d.sequenceSteps) != "drop:128|sublist:1024|sublist:0,512" ||
		encodeQScriptPipelineNames(d.sequenceBindings) != "y\x1fz" {
		t.Fatalf("sequence sum-count descriptor = base %q steps %#v names %#v", d.sequenceValueExpr, d.sequenceSteps, d.sequenceBindings)
	}
	if got, want := d.shape(), "script-pipeline/sequence-reduce/sum-count-transform-chain/assignments"; got != want {
		t.Fatalf("shape = %q, want %q", got, want)
	}
	assertEvalValue(t, src, int64(196864))
	descriptor, ok := DescribeEvalPipeline(src)
	if !ok {
		t.Fatalf("DescribeEvalPipeline(%q) did not recognize sequence sum-count pipeline", src)
	}
	out, handled, err := ExecuteEvalPipelineDescriptor(descriptor)
	if err != nil || !handled || out != int64(196864) {
		t.Fatalf("ExecuteEvalPipelineDescriptor sequence sum-count = %#v,%v,%v; want 196864,true,nil", out, handled, err)
	}
}

func TestQScriptPipelinePlannerDescribesGatherSumCountReduce(t *testing.T) {
	src := "x:til 4096;idx:32*til 128;(+/x[idx])+count idx"
	plan := buildQScriptPlan(src)
	if plan.scriptPipeline == nil {
		t.Fatalf("script pipeline descriptor missing")
	}
	d := plan.scriptPipeline
	if d.kind != qScriptPipelineGatherReduceSumCount {
		t.Fatalf("pipeline kind = %q, want %q", d.kind, qScriptPipelineGatherReduceSumCount)
	}
	if d.valueExpr != "x" || d.indexExpr != "idx" || d.indexBinding != "32*til 128" {
		t.Fatalf("gather sum-count descriptor = value %q index %q binding %q", d.valueExpr, d.indexExpr, d.indexBinding)
	}
	if got, want := d.shape(), "script-pipeline/gather-reduce/sum-count/assignments"; got != want {
		t.Fatalf("shape = %q, want %q", got, want)
	}
	assertEvalValue(t, src, int64(260224))
	descriptor, ok := DescribeEvalPipeline(src)
	if !ok {
		t.Fatalf("DescribeEvalPipeline(%q) did not recognize gather sum-count pipeline", src)
	}
	out, handled, err := ExecuteEvalPipelineDescriptor(descriptor)
	if err != nil || !handled || out != int64(260224) {
		t.Fatalf("ExecuteEvalPipelineDescriptor gather sum-count = %#v,%v,%v; want 260224,true,nil", out, handled, err)
	}
}

func TestQScriptPipelinePlannerDescribesSumPlusDyadicFloat(t *testing.T) {
	src := "x:(til 8) mod 4;y:2 xexp x;(+/y)+(+/2 xlog y)"
	plan := buildQScriptPlan(src)
	if plan.scriptPipeline == nil {
		t.Fatalf("script pipeline descriptor missing")
	}
	d := plan.scriptPipeline
	if d.kind != qScriptPipelineSumPlusDyadicFloat {
		t.Fatalf("pipeline kind = %q, want %q", d.kind, qScriptPipelineSumPlusDyadicFloat)
	}
	if d.valueExpr != "y" || d.valueBinding != "2 xexp x" || d.dyadicOp != "xlog" || d.scalarExpr != "2" || !d.scalarLeft {
		t.Fatalf("sum-plus-dyadic descriptor = value %q binding %q op %q scalar %q scalarLeft %v",
			d.valueExpr, d.valueBinding, d.dyadicOp, d.scalarExpr, d.scalarLeft)
	}
	descriptor, ok := DescribeEvalPipeline(src)
	if !ok {
		t.Fatalf("DescribeEvalPipeline(%q) did not recognize sum-plus-dyadic pipeline", src)
	}
	if descriptor.DyadicOp != "xlog" || descriptor.ScalarExpr != "2" || !descriptor.ScalarLeft {
		t.Fatalf("descriptor dyadic fields = %#v", descriptor)
	}
	out, handled, err := ExecuteEvalPipelineDescriptor(descriptor)
	if err != nil || !handled || out != 42.0 {
		t.Fatalf("ExecuteEvalPipelineDescriptor sum-plus-dyadic = %#v,%v,%v; want 42,true,nil", out, handled, err)
	}
	assertEvalValue(t, src, 42.0)
}

func TestQScriptPipelinePlannerDescribesIntegerDivModReduce(t *testing.T) {
	src := "x:til 1024;y:x+1;(+/y div 3)+(+/y mod 3)+count y"
	plan := buildQScriptPlan(src)
	if plan.scriptPipeline == nil {
		t.Fatalf("script pipeline descriptor missing")
	}
	d := plan.scriptPipeline
	if d.kind != qScriptPipelineIntegerDivModReduce {
		t.Fatalf("pipeline kind = %q, want %q", d.kind, qScriptPipelineIntegerDivModReduce)
	}
	if d.valueExpr != "y" || d.valueBinding != "x+1" {
		t.Fatalf("value descriptor = expr %q binding %q, want y/x+1", d.valueExpr, d.valueBinding)
	}
	if !d.includeCount || len(d.integerTerms) != 2 {
		t.Fatalf("integer terms = includeCount %v len %d, want true/2", d.includeCount, len(d.integerTerms))
	}
	if got, want := d.shape(), "script-pipeline/multi-reduce/integer-divmod-sum-count/assignments"; got != want {
		t.Fatalf("shape = %q, want %q", got, want)
	}
	descriptor, ok := DescribeEvalPipeline(src)
	if !ok {
		t.Fatalf("DescribeEvalPipeline(%q) did not recognize integer div/mod pipeline", src)
	}
	out, handled, err := ExecuteEvalPipelineDescriptor(descriptor)
	if err != nil || !handled || out != int64(176640) {
		t.Fatalf("ExecuteEvalPipelineDescriptor integer div/mod = %#v,%v,%v; want 176640,true,nil", out, handled, err)
	}
	assertEvalValue(t, src, int64(176640))
}

func TestQScriptPipelinePlannerDescribesMatrixRowSumCount(t *testing.T) {
	src := "m:2 4#til 8;row:m . 1;(+/row)+count row"
	plan := buildQScriptPlan(src)
	if plan.scriptPipeline == nil {
		t.Fatalf("script pipeline descriptor missing")
	}
	d := plan.scriptPipeline
	if d.kind != qScriptPipelineMatrixRowSumCount {
		t.Fatalf("pipeline kind = %q, want %q", d.kind, qScriptPipelineMatrixRowSumCount)
	}
	if d.valueExpr != "row" || d.valueBinding != "m . 1" || d.rowValueExpr != "m" || d.rowIndexExpr != "1" {
		t.Fatalf("matrix row descriptor = value %q binding %q rowValue %q rowIndex %q", d.valueExpr, d.valueBinding, d.rowValueExpr, d.rowIndexExpr)
	}
	if got, want := d.shape(), "script-pipeline/matrix-row-reduce/sum-count/assignments"; got != want {
		t.Fatalf("shape = %q, want %q", got, want)
	}
	descriptor, ok := DescribeEvalPipeline(src)
	if !ok {
		t.Fatalf("DescribeEvalPipeline(%q) did not recognize matrix row sum-count pipeline", src)
	}
	if descriptor.RowValueExpr != "m" || descriptor.RowIndexExpr != "1" {
		t.Fatalf("descriptor row fields = %q/%q, want m/1", descriptor.RowValueExpr, descriptor.RowIndexExpr)
	}
	out, handled, err := ExecuteEvalPipelineDescriptor(descriptor)
	if err != nil || !handled || out != int64(26) {
		t.Fatalf("ExecuteEvalPipelineDescriptor matrix row sum-count = %#v,%v,%v; want 26,true,nil", out, handled, err)
	}
	assertEvalValue(t, src, int64(26))
}

func TestQScriptPipelinePlannerDescribesMatrixRowsSumCount(t *testing.T) {
	src := "m:4 8#til 32;t:flip m;(+/(t . 0))+(+/(t . 7))+count t"
	plan := buildQScriptPlan(src)
	if plan.scriptPipeline == nil {
		t.Fatalf("script pipeline descriptor missing")
	}
	d := plan.scriptPipeline
	if d.kind != qScriptPipelineMatrixRowsSumCount {
		t.Fatalf("pipeline kind = %q, want %q", d.kind, qScriptPipelineMatrixRowsSumCount)
	}
	if d.rowValueExpr != "t" || d.rowIndexExpr != "0" || d.indexExpr != "7" {
		t.Fatalf("matrix rows descriptor = matrix %q row %q rows %q", d.rowValueExpr, d.rowIndexExpr, d.indexExpr)
	}
	flipAssignment, ok := qScriptPipelineAssignmentByName(d, "t")
	if !ok || flipAssignment.binding.kind != qScriptBindingUnary || flipAssignment.binding.op != "flip" {
		t.Fatalf("transpose assignment binding = %#v ok %v, want unary flip", flipAssignment.binding, ok)
	}
	reshapeAssignment, ok := qScriptPipelineAssignmentByName(d, "m")
	if !ok || reshapeAssignment.binding.kind != qScriptBindingBinary || reshapeAssignment.binding.op != "#" {
		t.Fatalf("reshape assignment binding = %#v ok %v, want binary reshape", reshapeAssignment.binding, ok)
	}
	if got, want := d.shape(), "script-pipeline/matrix-rows-reduce/sum-plus-count/assignments"; got != want {
		t.Fatalf("shape = %q, want %q", got, want)
	}
	descriptor, ok := DescribeEvalPipeline(src)
	if !ok {
		t.Fatalf("DescribeEvalPipeline(%q) did not recognize matrix rows sum-count pipeline", src)
	}
	if descriptor.RowValueExpr != "t" || descriptor.RowIndexExpr != "0" || descriptor.IndexExpr != "7" {
		t.Fatalf("descriptor matrix fields = %q/%q/%q, want t/0/7", descriptor.RowValueExpr, descriptor.RowIndexExpr, descriptor.IndexExpr)
	}
	out, handled, err := ExecuteEvalPipelineDescriptor(descriptor)
	if err != nil || !handled || out != int64(132) {
		t.Fatalf("ExecuteEvalPipelineDescriptor matrix rows sum-count = %#v,%v,%v; want 132,true,nil", out, handled, err)
	}
	assertEvalValue(t, src, int64(132))
}

func TestQScriptPipelinePlannerDescribesMatrixCellPlusCount(t *testing.T) {
	for _, src := range []string{
		"m:2 4#til 8;(m . 1 2)+count m",
		"m:2 4#til 8;count m+(m . 1 2)",
	} {
		t.Run(src, func(t *testing.T) {
			plan := buildQScriptPlan(src)
			if plan.scriptPipeline == nil {
				t.Fatalf("script pipeline descriptor missing")
			}
			d := plan.scriptPipeline
			if d.kind != qScriptPipelineMatrixCellPlusCount {
				t.Fatalf("pipeline kind = %q, want %q", d.kind, qScriptPipelineMatrixCellPlusCount)
			}
			if d.rowValueExpr != "m" || d.rowIndexExpr != "1" || d.colIndexExpr != "2" {
				t.Fatalf("matrix cell descriptor = matrix %q row %q col %q", d.rowValueExpr, d.rowIndexExpr, d.colIndexExpr)
			}
			if got, want := d.shape(), "script-pipeline/matrix-cell-reduce/cell-plus-count/assignments"; got != want {
				t.Fatalf("shape = %q, want %q", got, want)
			}
			descriptor, ok := DescribeEvalPipeline(src)
			if !ok {
				t.Fatalf("DescribeEvalPipeline(%q) did not recognize matrix cell plus count pipeline", src)
			}
			if descriptor.RowValueExpr != "m" || descriptor.RowIndexExpr != "1" || descriptor.ColIndexExpr != "2" {
				t.Fatalf("descriptor matrix fields = %q/%q/%q, want m/1/2", descriptor.RowValueExpr, descriptor.RowIndexExpr, descriptor.ColIndexExpr)
			}
			out, handled, err := ExecuteEvalPipelineDescriptor(descriptor)
			if err != nil || !handled || out != int64(8) {
				t.Fatalf("ExecuteEvalPipelineDescriptor matrix cell plus count = %#v,%v,%v; want 8,true,nil", out, handled, err)
			}
			assertEvalValue(t, src, int64(8))
		})
	}
}

func TestQScriptPipelinePlannerDescribesMatrixNestedCellCount(t *testing.T) {
	src := "m:3 4#1 2 3 4 5;t:flip m;(+/raze t)+(m . 2 3)+count t"
	plan := buildQScriptPlan(src)
	if plan.scriptPipeline == nil {
		t.Fatalf("script pipeline descriptor missing")
	}
	d := plan.scriptPipeline
	if d.kind != qScriptPipelineMatrixNestedCell {
		t.Fatalf("pipeline kind = %q, want %q", d.kind, qScriptPipelineMatrixNestedCell)
	}
	if d.valueExpr != "t" || d.rowValueExpr != "m" || d.indexExpr != "t" || d.rowIndexExpr != "2" || d.colIndexExpr != "3" {
		t.Fatalf("matrix nested descriptor = value %q rowValue %q count %q row %q col %q", d.valueExpr, d.rowValueExpr, d.indexExpr, d.rowIndexExpr, d.colIndexExpr)
	}
	if got, want := d.shape(), "script-pipeline/matrix-nested-reduce/sum-cell-count/assignments"; got != want {
		t.Fatalf("shape = %q, want %q", got, want)
	}
	descriptor, ok := DescribeEvalPipeline(src)
	if !ok {
		t.Fatalf("DescribeEvalPipeline(%q) did not recognize matrix nested cell pipeline", src)
	}
	out, handled, err := ExecuteEvalPipelineDescriptor(descriptor)
	if err != nil || !handled || out != int64(39) {
		t.Fatalf("ExecuteEvalPipelineDescriptor matrix nested cell = %#v,%v,%v; want 39,true,nil", out, handled, err)
	}
	assertEvalValue(t, src, int64(39))
}

func TestQScriptPipelinePlannerDescribesCallableDotSumPlusRight(t *testing.T) {
	src := "f:{(+/x)+y};.[f;(til 8;10)]"
	plan := buildQScriptPlan(src)
	if plan.scriptPipeline == nil {
		t.Fatalf("script pipeline descriptor missing")
	}
	d := plan.scriptPipeline
	if d.kind != qScriptPipelineCallableDotSumRight {
		t.Fatalf("pipeline kind = %q, want %q", d.kind, qScriptPipelineCallableDotSumRight)
	}
	if d.callableExpr != "f" || d.valueExpr != "til 8" || d.indexExpr != "10" {
		t.Fatalf("callable descriptor = callable %q value %q index %q", d.callableExpr, d.valueExpr, d.indexExpr)
	}
	if got, want := d.shape(), "script-pipeline/callable-dot/sum-plus-right/assignments"; got != want {
		t.Fatalf("shape = %q, want %q", got, want)
	}
	descriptor, ok := DescribeEvalPipeline(src)
	if !ok {
		t.Fatalf("DescribeEvalPipeline(%q) did not recognize callable dot pipeline", src)
	}
	if descriptor.CallableExpr != "f" || descriptor.ValueExpr != "til 8" || descriptor.IndexExpr != "10" {
		t.Fatalf("descriptor = %#v, want callable f value til 8 index 10", descriptor)
	}
	out, handled, err := ExecuteEvalPipelineDescriptor(descriptor)
	if err != nil || !handled || out != int64(38) {
		t.Fatalf("ExecuteEvalPipelineDescriptor callable dot = %#v,%v,%v; want 38,true,nil", out, handled, err)
	}
	assertEvalValue(t, src, int64(38))
}

func TestQScriptPipelinePlannerDescribesCallableDotSumPlusCount(t *testing.T) {
	src := "f:{(+/x)+count y};.[f;(til 8;10#1)]"
	plan := buildQScriptPlan(src)
	if plan.scriptPipeline == nil {
		t.Fatalf("script pipeline descriptor missing")
	}
	d := plan.scriptPipeline
	if d.kind != qScriptPipelineCallableDotSumCount {
		t.Fatalf("pipeline kind = %q, want %q", d.kind, qScriptPipelineCallableDotSumCount)
	}
	if d.callableExpr != "f" || d.valueExpr != "til 8" || d.indexExpr != "10#1" || !d.includeCount {
		t.Fatalf("callable descriptor = callable %q value %q index %q includeCount=%v", d.callableExpr, d.valueExpr, d.indexExpr, d.includeCount)
	}
	if got, want := d.shape(), "script-pipeline/callable-dot/sum-plus-count-right/assignments"; got != want {
		t.Fatalf("shape = %q, want %q", got, want)
	}
	descriptor, ok := DescribeEvalPipeline(src)
	if !ok {
		t.Fatalf("DescribeEvalPipeline(%q) did not recognize callable dot count pipeline", src)
	}
	if descriptor.CallableExpr != "f" || descriptor.ValueExpr != "til 8" || descriptor.IndexExpr != "10#1" || !descriptor.IncludeCount {
		t.Fatalf("descriptor = %#v, want callable f value til 8 index 10#1 includeCount", descriptor)
	}
	out, handled, err := ExecuteEvalPipelineDescriptor(descriptor)
	if err != nil || !handled || out != int64(38) {
		t.Fatalf("ExecuteEvalPipelineDescriptor callable dot count = %#v,%v,%v; want 38,true,nil", out, handled, err)
	}
	assertEvalValue(t, src, int64(38))
}

func TestQScriptPipelinePlannerDescribesCallableOverScanSum(t *testing.T) {
	src := "x:1+til 8;s:+\\x;({x+y}/[10;x])+last s+count s"
	plan := buildQScriptPlan(src)
	if plan.scriptPipeline == nil {
		t.Fatalf("script pipeline descriptor missing")
	}
	d := plan.scriptPipeline
	if d.kind != qScriptPipelineCallableOverScanSum {
		t.Fatalf("pipeline kind = %q, want %q", d.kind, qScriptPipelineCallableOverScanSum)
	}
	if d.valueExpr != "x" || d.valueBinding != "1+til 8" || d.scalarExpr != "10" {
		t.Fatalf("callable-over descriptor = value %q binding %q scalar %q", d.valueExpr, d.valueBinding, d.scalarExpr)
	}
	if got, want := d.shape(), "script-pipeline/callable-over/scan-sum-count/assignments"; got != want {
		t.Fatalf("shape = %q, want %q", got, want)
	}
	descriptor, ok := DescribeEvalPipeline(src)
	if !ok {
		t.Fatalf("DescribeEvalPipeline(%q) did not recognize callable-over scan pipeline", src)
	}
	if descriptor.ValueExpr != "x" || descriptor.ValueBinding != "1+til 8" || descriptor.ScalarExpr != "10" {
		t.Fatalf("descriptor = %#v, want value x binding 1+til 8 scalar 10", descriptor)
	}
	out, handled, err := ExecuteEvalPipelineDescriptor(descriptor)
	if err != nil || !handled || out != int64(90) {
		t.Fatalf("ExecuteEvalPipelineDescriptor callable-over scan = %#v,%v,%v; want 90,true,nil", out, handled, err)
	}
	executable, ok := CompileEvalPipelineBackendPlan(EvalPipelineBackendPlan{
		Backend:    EvalPipelineTypedRuntimeBackend,
		Detail:     "kind=" + descriptor.Kind,
		Descriptor: descriptor,
	})
	if !ok {
		t.Fatalf("CompileEvalPipelineBackendPlan failed for callable-over scan descriptor")
	}
	out, handled, err = NewEvalState(nil).ExecuteEvalPipelineExecutablePlan(executable)
	if err != nil || !handled || out != int64(90) {
		t.Fatalf("ExecuteEvalPipelineExecutablePlan callable-over scan = %#v,%v,%v; want 90,true,nil", out, handled, err)
	}
	assertEvalValue(t, src, int64(90))
}

func TestQScriptPipelineExecutablePlanCoversNDReshapeDotPath(t *testing.T) {
	src := "t:2 3 4#til 24;t . 1 2 3"
	descriptor, ok := DescribeEvalPipeline(src)
	if !ok {
		t.Fatalf("DescribeEvalPipeline(%q) did not recognize N-D reshape dot path", src)
	}
	if descriptor.Kind != "script" ||
		descriptor.Shape != "script-pipeline/apply-index/path-dot/assignments" ||
		descriptor.ValueExpr != "t" ||
		descriptor.ValueBinding != "2 3 4#til 24" ||
		descriptor.IndexExpr != "1 2 3" {
		t.Fatalf("descriptor = %#v, want script apply path over N-D reshape", descriptor)
	}
	backend := EvalPipelineBackendPlan{
		Backend:    EvalPipelineTypedRuntimeBackend,
		Detail:     "kind=" + descriptor.Kind,
		Descriptor: descriptor,
	}
	executable, ok := CompileEvalPipelineBackendPlan(backend)
	if !ok {
		t.Fatalf("CompileEvalPipelineBackendPlan failed for N-D reshape dot path")
	}
	out, handled, err := NewEvalState(nil).ExecuteEvalPipelineExecutablePlan(executable)
	if err != nil || !handled || out != int64(23) {
		t.Fatalf("ExecuteEvalPipelineExecutablePlan N-D reshape dot path = %#v,%v,%v; want 23,true,nil", out, handled, err)
	}
}

func TestQScriptPipelinePlannerDescribesStringJoinCounts(t *testing.T) {
	src := "x:8#`AAPL`MSFT`AMD`NVDA;s:\",\" sv string x;(count \",\" vs s)+(count s ss \"A\")+(count ssr[s;\"A\";\"Z\"])"
	plan := buildQScriptPlan(src)
	if plan.scriptPipeline == nil {
		t.Fatalf("script pipeline descriptor missing")
	}
	d := plan.scriptPipeline
	if d.kind != qScriptPipelineStringJoinCounts {
		t.Fatalf("pipeline kind = %q, want %q", d.kind, qScriptPipelineStringJoinCounts)
	}
	if d.valueExpr != "x" || d.valueBinding != "8#`AAPL`MSFT`AMD`NVDA" || d.indexExpr != "s" || d.maskExpr != "\",\"" || d.rowValueExpr != "\"A\"" || d.scalarExpr != "\"A\"" || d.dyadicOp != "\"Z\"" {
		t.Fatalf("string descriptor = %#v", d)
	}
	if got, want := d.shape(), "script-pipeline/string-join/counts/assignments"; got != want {
		t.Fatalf("shape = %q, want %q", got, want)
	}
	descriptor, ok := DescribeEvalPipeline(src)
	if !ok {
		t.Fatalf("DescribeEvalPipeline(%q) did not recognize string join counts pipeline", src)
	}
	out, handled, err := ExecuteEvalPipelineDescriptor(descriptor)
	if err != nil || !handled || out != int64(53) {
		t.Fatalf("ExecuteEvalPipelineDescriptor string join counts = %#v,%v,%v; want 53,true,nil", out, handled, err)
	}
	executable, ok := CompileEvalPipelineBackendPlan(EvalPipelineBackendPlan{
		Backend:    EvalPipelineTypedRuntimeBackend,
		Detail:     "kind=" + descriptor.Kind,
		Descriptor: descriptor,
	})
	if !ok {
		t.Fatalf("CompileEvalPipelineBackendPlan failed for string join counts descriptor")
	}
	out, handled, err = NewEvalState(nil).ExecuteEvalPipelineExecutablePlan(executable)
	if err != nil || !handled || out != int64(53) {
		t.Fatalf("ExecuteEvalPipelineExecutablePlan string join counts = %#v,%v,%v; want 53,true,nil", out, handled, err)
	}
	ClearRuntimeKernelExecutionStats()
	t.Cleanup(ClearRuntimeKernelExecutionStats)
	assertEvalValue(t, src, int64(53))
	for _, stat := range RuntimeKernelExecutionStats() {
		if stat.Outcome == "fallback" || stat.Outcome == "error" {
			t.Fatalf("unexpected string join counts fallback/error: %#v all=%#v", stat, RuntimeKernelExecutionStats())
		}
	}
}

func TestQScriptPipelinePlannerRecordsRuntimeStats(t *testing.T) {
	ClearRuntimeKernelExecutionStats()
	t.Cleanup(ClearRuntimeKernelExecutionStats)

	assertEvalValue(t, "x:til 32;y:x*2;+/y[where (x>=4) and x<12]", int64(120))
	seen := false
	for _, stat := range RuntimeKernelExecutionStats() {
		if stat.Kernel == "QScriptPipelinePlan" &&
			stat.Shape == "script-pipeline/where-index-reduce/sum/assignments" &&
			stat.PipelineShape == "script_pipeline" &&
			stat.Outcome == "hit" &&
			stat.Count > 0 {
			seen = true
		}
	}
	if !seen {
		t.Fatalf("missing script pipeline runtime stat: %#v", RuntimeKernelExecutionStats())
	}
}

func TestQScriptPipelineDirectExecutionPreservesAssignments(t *testing.T) {
	ClearRuntimeKernelExecutionStats()
	t.Cleanup(ClearRuntimeKernelExecutionStats)

	state := NewEvalState(nil)
	assertStateEvalValue(t, state, "x:til 16;y:x*3;idx:where (x>=4) and x<8;+/y[idx]", int64(66))
	assertStateEvalArray(t, state, "idx", data.KindI64, []any{int64(4), int64(5), int64(6), int64(7)})
	assertStateEvalValue(t, state, "+/y[idx]", int64(66))

	seenScriptHit := false
	seenPipelineHit := false
	for _, stat := range RuntimeKernelExecutionStats() {
		if stat.Kernel == "QScriptPipelinePlan" && stat.Outcome == "hit" && stat.Count > 0 {
			seenScriptHit = true
		}
		if stat.Kernel == "QPipelinePlan" && stat.Outcome == "hit" && stat.Count > 0 {
			seenPipelineHit = true
		}
	}
	if !seenScriptHit || !seenPipelineHit {
		t.Fatalf("missing direct script pipeline hits: script=%v pipeline=%v stats=%#v", seenScriptHit, seenPipelineHit, RuntimeKernelExecutionStats())
	}
}

func TestDescribeEvalPipelineExposesReadOnlyRuntimeDescriptor(t *testing.T) {
	descriptor, ok := DescribeEvalPipeline("x:til 64;y:(x*3)+7;idx:where x>8;+/y[idx]")
	if !ok {
		t.Fatalf("DescribeEvalPipeline did not recognize script pipeline")
	}
	if descriptor.Kind != "script" ||
		descriptor.Kernel != "QScriptPipelinePlan" ||
		descriptor.Shape != "script-pipeline/where-index-reduce/sum/assignments" ||
		descriptor.PipelineShape != "script_pipeline" {
		t.Fatalf("descriptor identity = kind %q kernel %q shape %q pipeline %q, want script QScriptPipelinePlan where-index shape",
			descriptor.Kind, descriptor.Kernel, descriptor.Shape, descriptor.PipelineShape)
	}
	if got, want := len(descriptor.Assignments), 3; got != want {
		t.Fatalf("assignment count = %d, want %d", got, want)
	}
	if descriptor.ValueExpr != "y" || descriptor.ValueBinding != "(x*3)+7" {
		t.Fatalf("value descriptor = expr %q binding %q, want y/(x*3)+7", descriptor.ValueExpr, descriptor.ValueBinding)
	}
	if descriptor.IndexExpr != "idx" || descriptor.IndexBinding != "where x>8" || descriptor.MaskExpr != "x>8" {
		t.Fatalf("index/mask descriptor = index %q binding %q mask %q, want idx/where x>8/x>8",
			descriptor.IndexExpr, descriptor.IndexBinding, descriptor.MaskExpr)
	}

	descriptor, ok = DescribeEvalPipeline("+/where x>8")
	if !ok {
		t.Fatalf("DescribeEvalPipeline did not recognize expression pipeline")
	}
	if descriptor.Kind != "expression" ||
		descriptor.Kernel != "QPipelinePlan" ||
		descriptor.Shape != "compare-to-index-sum" ||
		descriptor.PipelineShape != "mask_reduce" ||
		descriptor.LeftExpr != "x" ||
		descriptor.RightExpr != "8" ||
		descriptor.CompareOp != ">" {
		t.Fatalf("expression descriptor = %#v, want compare-to-index-sum mask_reduce x > 8", descriptor)
	}
	for _, tc := range []struct {
		name           string
		expr           string
		shape          string
		transform      string
		leftExpr       string
		reductionInput string
		want           any
	}{
		{name: "reverse", expr: "sum reverse 8#til 4", shape: "vector-reduce/sum-reverse", transform: "reverse", reductionInput: "8#til 4", want: int64(12)},
		{name: "rotate", expr: "+/2 rotate 8#til 4", shape: "vector-reduce/sum-rotate", transform: "rotate", leftExpr: "2", reductionInput: "8#til 4", want: int64(12)},
		{name: "sublist", expr: "+/2 4 sublist 1+til 8", shape: "vector-reduce/sum-sublist", transform: "sublist", leftExpr: "2 4", reductionInput: "1+til 8", want: int64(18)},
		{name: "ratios", expr: "+/ratios 2 4 8 16", shape: "vector-reduce/sum-ratios", transform: "ratios", reductionInput: "2 4 8 16", want: float64(8)},
	} {
		t.Run("sequence_transform_sum_descriptor_"+tc.name, func(t *testing.T) {
			descriptor, ok = DescribeEvalPipeline(tc.expr)
			if !ok {
				t.Fatalf("DescribeEvalPipeline did not recognize generic vector sum pipeline")
			}
			if descriptor.Shape != tc.shape ||
				descriptor.PipelineShape != "vector_reduce" ||
				descriptor.ShapeFamily != "vector" ||
				descriptor.ShapeReducer != "sum" ||
				descriptor.ShapeTransform != tc.transform ||
				descriptor.LeftExpr != tc.leftExpr ||
				descriptor.ReductionInput != tc.reductionInput {
				t.Fatalf("generic sum descriptor = %#v", descriptor)
			}
			if got, handled, err := ExecuteEvalPipelineDescriptor(descriptor); err != nil || !handled || got != tc.want {
				t.Fatalf("ExecuteEvalPipelineDescriptor generic sum = %#v,%v,%v; want %#v,true,nil", got, handled, err, tc.want)
			}
		})
	}

	descriptor, ok = DescribeEvalPipeline("+/x min y")
	if !ok {
		t.Fatalf("DescribeEvalPipeline did not recognize dyadic min sum pipeline")
	}
	if descriptor.Shape != "vector-reduce/sum-dyadic-min" ||
		descriptor.PipelineShape != "vector_reduce" ||
		descriptor.ShapeFamily != "vector" ||
		descriptor.ShapeReducer != "sum" ||
		descriptor.ShapeTransform != "dyadic-min" ||
		descriptor.LeftExpr != "x" ||
		descriptor.RightExpr != "y" ||
		descriptor.CompareOp != "min" {
		t.Fatalf("dyadic min sum descriptor = %#v", descriptor)
	}
	if got, handled, err := ExecuteEvalPipelineDescriptorWithEnv(EvalPipelineDescriptor{
		Source:    "+/x min y",
		Kind:      descriptor.Kind,
		Kernel:    descriptor.Kernel,
		Shape:     descriptor.Shape,
		LeftExpr:  descriptor.LeftExpr,
		RightExpr: descriptor.RightExpr,
		CompareOp: descriptor.CompareOp,
	}, map[string]any{
		"x": data.NewI64Range(0, 1, 8),
		"y": data.NewI64Range(7, -1, 8),
	}); err != nil || !handled || got != int64(12) {
		t.Fatalf("ExecuteEvalPipelineDescriptor dyadic min sum = %#v,%v,%v; want 12,true,nil", got, handled, err)
	}

	descriptor, ok = DescribeEvalPipeline("count 8#til 4")
	if !ok {
		t.Fatalf("DescribeEvalPipeline did not recognize generic vector count pipeline")
	}
	if descriptor.Shape != "vector-count/expr" ||
		descriptor.PipelineShape != "vector_scan" ||
		descriptor.ShapeFamily != "vector" ||
		descriptor.ShapeReducer != "count" ||
		descriptor.ShapeTransform != "expr" ||
		descriptor.ReductionInput != "8#til 4" {
		t.Fatalf("generic count descriptor = %#v", descriptor)
	}
	if got, handled, err := ExecuteEvalPipelineDescriptor(descriptor); err != nil || !handled || got != int64(8) {
		t.Fatalf("ExecuteEvalPipelineDescriptor generic count = %#v,%v,%v; want 8,true,nil", got, handled, err)
	}

	descriptor, ok = DescribeEvalPipeline("+/3 msum 1+til 5")
	if !ok {
		t.Fatalf("DescribeEvalPipeline did not recognize moving window sum pipeline")
	}
	if descriptor.Shape != "vector-reduce/sum-msum" ||
		descriptor.PipelineShape != "vector_reduce" ||
		descriptor.ShapeFamily != "vector" ||
		descriptor.ShapeReducer != "sum" ||
		descriptor.ShapeTransform != "msum" ||
		descriptor.LeftExpr != "3" ||
		descriptor.RightExpr != "1+til 5" ||
		descriptor.CompareOp != "msum" {
		t.Fatalf("moving sum descriptor = %#v", descriptor)
	}
	if got, handled, err := ExecuteEvalPipelineDescriptor(descriptor); err != nil || !handled || got != int64(31) {
		t.Fatalf("ExecuteEvalPipelineDescriptor moving sum = %#v,%v,%v; want 31,true,nil", got, handled, err)
	}

	descriptor, ok = DescribeEvalPipeline("+/2 mdev 1 2 3")
	if !ok {
		t.Fatalf("DescribeEvalPipeline did not recognize moving stddev sum pipeline")
	}
	if descriptor.Shape != "vector-reduce/sum-mdev" ||
		descriptor.PipelineShape != "vector_reduce" ||
		descriptor.ShapeFamily != "vector" ||
		descriptor.ShapeReducer != "sum" ||
		descriptor.ShapeTransform != "mdev" ||
		descriptor.LeftExpr != "2" ||
		descriptor.RightExpr != "1 2 3" ||
		descriptor.CompareOp != "mdev" {
		t.Fatalf("moving stddev descriptor = %#v", descriptor)
	}
	if got, handled, err := ExecuteEvalPipelineDescriptor(descriptor); err != nil || !handled || got != 1.0 {
		t.Fatalf("ExecuteEvalPipelineDescriptor moving stddev sum = %#v,%v,%v; want 1,true,nil", got, handled, err)
	}

	descriptor, ok = DescribeEvalPipeline("+/0.5 ema 1 2 3")
	if !ok {
		t.Fatalf("DescribeEvalPipeline did not recognize ema sum pipeline")
	}
	if descriptor.Shape != "vector-reduce/sum-ema" ||
		descriptor.PipelineShape != "vector_reduce" ||
		descriptor.ShapeFamily != "vector" ||
		descriptor.ShapeReducer != "sum" ||
		descriptor.ShapeTransform != "ema" ||
		descriptor.LeftExpr != "0.5" ||
		descriptor.RightExpr != "1 2 3" ||
		descriptor.CompareOp != "ema" {
		t.Fatalf("ema descriptor = %#v", descriptor)
	}
	if got, handled, err := ExecuteEvalPipelineDescriptor(descriptor); err != nil || !handled || got != 4.75 {
		t.Fatalf("ExecuteEvalPipelineDescriptor ema sum = %#v,%v,%v; want 4.75,true,nil", got, handled, err)
	}

	descriptor, ok = DescribeEvalPipeline("count sums 8#til 4")
	if !ok {
		t.Fatalf("DescribeEvalPipeline did not recognize count running scan pipeline")
	}
	if descriptor.Shape != "vector-count/sums" ||
		descriptor.PipelineShape != "vector_scan" ||
		descriptor.ReductionInput != "8#til 4" ||
		descriptor.CompareOp != "sums" {
		t.Fatalf("count running scan descriptor = %#v", descriptor)
	}
	if got, handled, err := ExecuteEvalPipelineDescriptor(descriptor); err != nil || !handled || got != int64(8) {
		t.Fatalf("ExecuteEvalPipelineDescriptor count running scan = %#v,%v,%v; want 8,true,nil", got, handled, err)
	}
}

func TestExecuteEvalPipelineRunsOnlyRecognizedRuntimePlans(t *testing.T) {
	ClearRuntimeKernelExecutionStats()
	t.Cleanup(ClearRuntimeKernelExecutionStats)

	got, handled, err := ExecuteEvalPipeline("x:til 16;y:x*3;idx:where (x>=4) and x<8;+/y[idx]")
	if err != nil || !handled || got != int64(66) {
		t.Fatalf("ExecuteEvalPipeline script = %#v, %v, %v; want 66,true,nil", got, handled, err)
	}
	got, handled, err = ExecuteEvalPipeline("+/deltas til 8")
	if err != nil || !handled || got != int64(7) {
		t.Fatalf("ExecuteEvalPipeline expression = %#v, %v, %v; want 7,true,nil", got, handled, err)
	}
	got, handled, err = ExecuteEvalPipeline("1+2")
	if err != nil || handled || got != nil {
		t.Fatalf("ExecuteEvalPipeline unsupported = %#v, %v, %v; want nil,false,nil", got, handled, err)
	}
}

func TestEvalPipelineBackendEntrypointsShareExecutablePlan(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
		want any
	}{
		{name: "expression", src: "+/deltas til 8", want: int64(7)},
		{name: "script", src: "x:til 16;y:x*3;idx:where x>=8;+/y[idx]", want: int64(276)},
		{name: "find_sum", src: "+/`AAPL`MSFT`NVDA?`MSFT`TSLA", want: int64(4)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			descriptor, ok := DescribeEvalPipeline(tc.src)
			if !ok {
				t.Fatalf("DescribeEvalPipeline(%q) did not recognize pipeline", tc.src)
			}
			backend, ok := DescribeEvalPipelineBackendPlan(tc.src)
			if !ok {
				t.Fatalf("DescribeEvalPipelineBackendPlan(%q) did not recognize pipeline", tc.src)
			}
			executable, ok := CompileEvalPipelineBackendPlan(backend)
			if !ok {
				t.Fatalf("CompileEvalPipelineBackendPlan(%q) failed", tc.src)
			}

			for _, run := range []struct {
				name string
				call func() (any, bool, error)
			}{
				{name: "source", call: func() (any, bool, error) { return ExecuteEvalPipeline(tc.src) }},
				{name: "descriptor", call: func() (any, bool, error) { return ExecuteEvalPipelineDescriptor(descriptor) }},
				{name: "backend", call: func() (any, bool, error) { return ExecuteEvalPipelineBackendPlan(backend) }},
				{name: "executable", call: func() (any, bool, error) {
					return NewEvalState(nil).ExecuteEvalPipelineExecutablePlan(executable)
				}},
			} {
				got, handled, err := run.call()
				if err != nil || !handled || got != tc.want {
					t.Fatalf("%s pipeline run = %#v,%v,%v; want %#v,true,nil", run.name, got, handled, err, tc.want)
				}
			}
		})
	}
}

func TestEvalPipelineBackendEntrypointsRestoreFindIndexes(t *testing.T) {
	src := "`AAPL`MSFT`NVDA?`MSFT`TSLA"
	descriptor, ok := DescribeEvalPipeline(src)
	if !ok {
		t.Fatalf("DescribeEvalPipeline(%q) did not recognize find pipeline", src)
	}
	if descriptor.Shape != "find" || descriptor.PipelineShape != "find" || descriptor.LeftExpr == "" || descriptor.RightExpr == "" {
		t.Fatalf("find descriptor = %#v, want find shape with operands", descriptor)
	}
	backend, ok := DescribeEvalPipelineBackendPlan(src)
	if !ok {
		t.Fatalf("DescribeEvalPipelineBackendPlan(%q) did not recognize find pipeline", src)
	}
	executable, ok := CompileEvalPipelineBackendPlan(backend)
	if !ok {
		t.Fatalf("CompileEvalPipelineBackendPlan(%q) failed", src)
	}
	for _, run := range []struct {
		name string
		call func() (any, bool, error)
	}{
		{name: "source", call: func() (any, bool, error) { return ExecuteEvalPipeline(src) }},
		{name: "descriptor", call: func() (any, bool, error) { return ExecuteEvalPipelineDescriptor(descriptor) }},
		{name: "backend", call: func() (any, bool, error) { return ExecuteEvalPipelineBackendPlan(backend) }},
		{name: "executable", call: func() (any, bool, error) {
			return NewEvalState(nil).ExecuteEvalPipelineExecutablePlan(executable)
		}},
	} {
		got, handled, err := run.call()
		if err != nil || !handled {
			t.Fatalf("%s find pipeline run = %#v,%v,%v; want handled nil error", run.name, got, handled, err)
		}
		array, ok := got.(data.Array)
		if !ok {
			t.Fatalf("%s find pipeline result = %T, want data.Array", run.name, got)
		}
		if values := array.Values(); !reflect.DeepEqual(values, []any{int64(1), int64(3)}) {
			t.Fatalf("%s find pipeline values = %#v, want 1 3", run.name, values)
		}
	}
}

func TestEvalPipelineBackendEntrypointsRestoreScriptFindSum(t *testing.T) {
	ClearRuntimeKernelExecutionStats()
	t.Cleanup(ClearRuntimeKernelExecutionStats)

	src := "x:(til 12) mod 4;+/0 1 2 3?x"
	descriptor, ok := DescribeEvalPipeline(src)
	if !ok {
		t.Fatalf("DescribeEvalPipeline(%q) did not recognize script find-sum pipeline", src)
	}
	if descriptor.Shape != "script-pipeline/find-reduce/sum/assignments" ||
		descriptor.ValueExpr != "0 1 2 3" || descriptor.IndexExpr != "x" ||
		len(descriptor.Assignments) != 1 || descriptor.Assignments[0].Name != "x" {
		t.Fatalf("script find-sum descriptor = %#v", descriptor)
	}
	backend, ok := DescribeEvalPipelineBackendPlan(src)
	if !ok {
		t.Fatalf("DescribeEvalPipelineBackendPlan(%q) did not recognize script find-sum pipeline", src)
	}
	executable, ok := CompileEvalPipelineBackendPlan(backend)
	if !ok {
		t.Fatalf("CompileEvalPipelineBackendPlan(%q) failed", src)
	}
	for _, run := range []struct {
		name string
		call func() (any, bool, error)
	}{
		{name: "source", call: func() (any, bool, error) { return ExecuteEvalPipeline(src) }},
		{name: "descriptor", call: func() (any, bool, error) { return ExecuteEvalPipelineDescriptor(descriptor) }},
		{name: "backend", call: func() (any, bool, error) { return ExecuteEvalPipelineBackendPlan(backend) }},
		{name: "executable", call: func() (any, bool, error) {
			return NewEvalState(nil).ExecuteEvalPipelineExecutablePlan(executable)
		}},
	} {
		got, handled, err := run.call()
		if err != nil || !handled || got != int64(18) {
			t.Fatalf("%s script find-sum pipeline run = %#v,%v,%v; want 18,true,nil", run.name, got, handled, err)
		}
	}

	seenFindSum := false
	for _, stat := range RuntimeKernelExecutionStats() {
		if stat.Kernel == "ArrayFindSum" && stat.Shape == "vector-reduce/find-sum/i64/i64" && stat.Outcome == "hit" {
			seenFindSum = true
		}
		if stat.Outcome == "fallback" || stat.Outcome == "error" {
			t.Fatalf("unexpected script find-sum fallback/error: %#v all=%#v", stat, RuntimeKernelExecutionStats())
		}
	}
	if !seenFindSum {
		t.Fatalf("missing ArrayFindSum hit: %#v", RuntimeKernelExecutionStats())
	}
}

func TestExecuteEvalPipelineDescriptorRestoresModuloScriptSubPlan(t *testing.T) {
	ClearRuntimeKernelExecutionStats()
	t.Cleanup(ClearRuntimeKernelExecutionStats)

	descriptor, ok := DescribeEvalPipeline("x:til 128;y:(x*2)+1;idx:where (x mod 3)=0;+/y[idx]")
	if !ok {
		t.Fatalf("DescribeEvalPipeline did not recognize modulo script pipeline")
	}
	got, handled, err := ExecuteEvalPipelineDescriptor(descriptor)
	if err != nil || !handled || got != int64(5461) {
		t.Fatalf("ExecuteEvalPipelineDescriptor = %#v,%v,%v; want 5461,true,nil", got, handled, err)
	}

	seenReduce := false
	seenGather := false
	for _, stat := range RuntimeKernelExecutionStats() {
		if stat.Kernel == "ArrayModuloCompareReduceSum" && stat.Outcome == "hit" && stat.Count > 0 {
			seenReduce = true
		}
		if stat.Kernel == "ArrayGatherReduceSum" && stat.Outcome == "hit" && stat.Count > 0 {
			seenGather = true
		}
	}
	if !seenReduce || seenGather {
		t.Fatalf("descriptor script stats reduce=%v gather=%v all=%#v", seenReduce, seenGather, RuntimeKernelExecutionStats())
	}
}

func TestExecuteEvalPipelineUsesIndexExprReducer(t *testing.T) {
	ClearRuntimeKernelExecutionStats()
	t.Cleanup(ClearRuntimeKernelExecutionStats)

	got, handled, err := ExecuteEvalPipeline("x:til 16;idx:where (x>=4) and x<8;pxi:10+(idx mod 5);szi:1+(idx mod 3);(+/(pxi*szi))+count idx")
	if err != nil || !handled || got != int64(97) {
		t.Fatalf("ExecuteEvalPipeline index expr = %#v,%v,%v; want 97,true,nil", got, handled, err)
	}
	for _, stat := range RuntimeKernelExecutionStats() {
		if stat.Kernel == "ArrayIndexExprReducers" && stat.Outcome == "hit" && stat.Count > 0 {
			return
		}
	}
	t.Fatalf("missing ArrayIndexExprReducers hit: %#v", RuntimeKernelExecutionStats())
}

func TestExecuteEvalPipelineUsesIndexExprMultiReducers(t *testing.T) {
	ClearRuntimeKernelExecutionStats()
	t.Cleanup(ClearRuntimeKernelExecutionStats)

	got, handled, err := ExecuteEvalPipeline("x:til 16;idx:where x>=8;pxi:10+(idx mod 5);szi:1+(idx mod 3);(+/(pxi*szi))+(+/pxi)+count idx")
	if err != nil || !handled || got != int64(301) {
		t.Fatalf("ExecuteEvalPipeline index expr multi reducers = %#v,%v,%v; want 301,true,nil", got, handled, err)
	}
	for _, stat := range RuntimeKernelExecutionStats() {
		if stat.Kernel == "ArrayIndexExprReducers" && stat.Shape == "index-expr-reduce/reducers/i64/3" && stat.Outcome == "hit" && stat.Count > 0 {
			return
		}
	}
	t.Fatalf("missing ArrayIndexExprReducers multi hit: %#v", RuntimeKernelExecutionStats())
}

func TestExecuteEvalPipelineUsesIndexExprComputedProjectionReducers(t *testing.T) {
	ClearRuntimeKernelExecutionStats()
	t.Cleanup(ClearRuntimeKernelExecutionStats)

	src := "px:100+((til 16) mod 90);sz:1+((til 16) mod 64);idx:where sz>=8;pxi:100+(idx mod 90);szi:1+(idx mod 64);(+/(pxi*szi))+(+/(count idx)#2)+(+/10 xbar pxi)+count idx"
	got, handled, err := ExecuteEvalPipeline(src)
	if err != nil || !handled || got != int64(13035) {
		t.Fatalf("ExecuteEvalPipeline computed projection reducers = %#v,%v,%v; want 13035,true,nil", got, handled, err)
	}
	for _, stat := range RuntimeKernelExecutionStats() {
		if stat.Kernel == "ArrayIndexExprReducers" && stat.Shape == "index-expr-reduce/reducers/i64/4" && stat.Outcome == "hit" && stat.Count > 0 {
			return
		}
	}
	t.Fatalf("missing ArrayIndexExprReducers computed projection hit: %#v", RuntimeKernelExecutionStats())
}

func TestExecuteEvalPipelineUsesWhereGatherSumCountWithLazyPredicate(t *testing.T) {
	ClearRuntimeKernelExecutionStats()
	t.Cleanup(ClearRuntimeKernelExecutionStats)

	src := "x:til 8192;y:(x*2)+7;idx:where (y mod 5)>1;(+/y[idx])+count idx"
	got, handled, err := ExecuteEvalPipeline(src)
	if err != nil || !handled || got != int64(40306284) {
		t.Fatalf("ExecuteEvalPipeline lazy predicate gather sum-count = %#v,%v,%v; want 40306284,true,nil", got, handled, err)
	}
	seen := false
	for _, stat := range RuntimeKernelExecutionStats() {
		if stat.Outcome == "fallback" || stat.Outcome == "error" {
			t.Fatalf("unexpected lazy predicate gather sum-count fallback/error: %#v all=%#v", stat, RuntimeKernelExecutionStats())
		}
		if stat.Kernel == "ArrayWhereGatherSumCount" && strings.HasPrefix(stat.Shape, "where-index-reduce/sum-count/") &&
			stat.Outcome == "hit" && stat.Count > 0 {
			seen = true
		}
	}
	if !seen {
		t.Fatalf("missing ArrayWhereGatherSumCount hit: %#v", RuntimeKernelExecutionStats())
	}
}

func TestExecuteEvalPipelineUsesWhereGatherSumCountSelfPredicate(t *testing.T) {
	ClearRuntimeKernelExecutionStats()
	t.Cleanup(ClearRuntimeKernelExecutionStats)

	const rows = 8192
	var want int64
	for i := int64(0); i < rows; i++ {
		x := i * 2
		if x > 10 {
			want += x + 1
		}
	}
	src := "x:(til 8192)*2;idx:where x>10;(+/x[idx])+count idx"
	got, handled, err := ExecuteEvalPipeline(src)
	if err != nil || !handled || got != want {
		t.Fatalf("ExecuteEvalPipeline self predicate gather sum-count = %#v,%v,%v; want %d,true,nil", got, handled, err, want)
	}
	for _, stat := range RuntimeKernelExecutionStats() {
		if stat.Outcome == "fallback" || stat.Outcome == "error" {
			t.Fatalf("unexpected self predicate gather sum-count fallback/error: %#v all=%#v", stat, RuntimeKernelExecutionStats())
		}
		if stat.Kernel == "ArrayWhereGatherSumCount" && stat.Shape == "where-index-reduce/sum-count/i64/>/i64/i64" &&
			stat.Outcome == "hit" && stat.Count > 0 {
			return
		}
	}
	t.Fatalf("missing self predicate ArrayWhereGatherSumCount hit: %#v", RuntimeKernelExecutionStats())
}

func TestExecuteEvalPipelineUsesWhereGatherSumCountThroughAliases(t *testing.T) {
	ClearRuntimeKernelExecutionStats()
	t.Cleanup(ClearRuntimeKernelExecutionStats)

	const rows = 8192
	var want int64
	for i := int64(0); i < rows; i++ {
		x := i * 2
		if x > 10 {
			want += x + 1
		}
	}
	src := "x:(til 8192)*2;idx:where x>10;c:count idx;v:x[idx];s:+/v;s+c"
	got, handled, err := ExecuteEvalPipeline(src)
	if err != nil || !handled || got != want {
		t.Fatalf("ExecuteEvalPipeline alias gather sum-count = %#v,%v,%v; want %d,true,nil", got, handled, err, want)
	}
	for _, stat := range RuntimeKernelExecutionStats() {
		if stat.Outcome == "fallback" || stat.Outcome == "error" {
			t.Fatalf("unexpected alias gather sum-count fallback/error: %#v all=%#v", stat, RuntimeKernelExecutionStats())
		}
		if stat.Kernel == "ArrayWhereGatherSumCount" && stat.Shape == "where-index-reduce/sum-count/i64/>/i64/i64" &&
			stat.Outcome == "hit" && stat.Count > 0 {
			return
		}
	}
	t.Fatalf("missing alias ArrayWhereGatherSumCount hit: %#v", RuntimeKernelExecutionStats())
}

func TestExecuteEvalPipelineUsesWhereGatherSumCountWithinPredicate(t *testing.T) {
	ClearRuntimeKernelExecutionStats()
	t.Cleanup(ClearRuntimeKernelExecutionStats)

	const rows = 8192
	var want int64
	for i := int64(0); i < rows; i++ {
		y := i*2 + 7
		if i%10 >= 3 && i%10 <= 6 {
			want += y + 1
		}
	}
	src := "x:til 8192;y:(x*2)+7;idx:where (x mod 10) within 3 6;(+/y[idx])+count idx"
	got, handled, err := ExecuteEvalPipeline(src)
	if err != nil || !handled || got != want {
		t.Fatalf("ExecuteEvalPipeline within predicate gather sum-count = %#v,%v,%v; want %d,true,nil", got, handled, err, want)
	}
	for _, stat := range RuntimeKernelExecutionStats() {
		if stat.Outcome == "fallback" || stat.Outcome == "error" {
			t.Fatalf("unexpected within predicate gather sum-count fallback/error: %#v all=%#v", stat, RuntimeKernelExecutionStats())
		}
		if stat.Kernel == "ArrayWhereGatherSumCount" && stat.Shape == "where-index-reduce/sum-count/i64/within/i64/i64/i64" &&
			stat.Outcome == "hit" && stat.Count > 0 {
			return
		}
	}
	t.Fatalf("missing within predicate ArrayWhereGatherSumCount hit: %#v", RuntimeKernelExecutionStats())
}

func TestEvalPipelineSumWhereCompareAvoidsMaskMaterialization(t *testing.T) {
	ClearRuntimeKernelExecutionStats()
	t.Cleanup(ClearRuntimeKernelExecutionStats)

	env := map[string]any{
		"v":         data.NewI64([]int64{1, 2, 3, 4, 5}),
		"threshold": int64(2),
	}
	got, err := EvalWithEnv("+/v where v>threshold", env)
	if err != nil || got != int64(12) {
		t.Fatalf("EvalWithEnv sum where compare = %#v,%v; want 12,nil", got, err)
	}
	got, err = EvalWithEnv("+/v where threshold<v", env)
	if err != nil || got != int64(12) {
		t.Fatalf("EvalWithEnv reversed sum where compare = %#v,%v; want 12,nil", got, err)
	}
	hits := 0
	for _, stat := range RuntimeKernelExecutionStats() {
		if stat.Outcome == "fallback" || stat.Outcome == "error" {
			t.Fatalf("unexpected sum where compare fallback/error: %#v all=%#v", stat, RuntimeKernelExecutionStats())
		}
		if stat.Kernel == "ArrayWhereGatherSumCount" && strings.HasPrefix(stat.Shape, "where-reduce/sum-count/") &&
			stat.Outcome == "hit" && stat.Count > 0 {
			hits += int(stat.Count)
		}
		if stat.Kernel == "ArrayWhereReduceSum" && stat.Outcome == "hit" {
			t.Fatalf("sum where compare materialized mask reduce path: %#v all=%#v", stat, RuntimeKernelExecutionStats())
		}
	}
	if hits < 2 {
		t.Fatalf("missing fused sum where compare hits: hits=%d stats=%#v", hits, RuntimeKernelExecutionStats())
	}
}

func TestEvalNumericReductionBundleRecordsTypedRuntimeKernel(t *testing.T) {
	ClearRuntimeKernelExecutionStats()
	t.Cleanup(ClearRuntimeKernelExecutionStats)

	assertEvalValue(t, "x:1+til 8;s:+/x;named:sum x;mx:max x;mn:min x;cnt:count x;s+named+mx+mn+cnt", int64(89))

	counts := map[string]uint64{}
	for _, stat := range RuntimeKernelExecutionStats() {
		if stat.Outcome == "hit" && stat.ReasonCode == "typed_kernel" {
			counts[stat.Kernel] += stat.Count
		}
	}
	if counts["ArrayNumericStats"] != 1 {
		t.Fatalf("ArrayNumericStats hits = %d, want 1; stats=%#v", counts["ArrayNumericStats"], RuntimeKernelExecutionStats())
	}
	if counts["ArraySum"] != 0 || counts["ArrayMin"] != 0 || counts["ArrayMax"] != 0 {
		t.Fatalf("individual aggregate kernels hit during bundle: sum=%d min=%d max=%d stats=%#v", counts["ArraySum"], counts["ArrayMin"], counts["ArrayMax"], RuntimeKernelExecutionStats())
	}
}

func TestEvalDeferredScanAssignmentTerminalLast(t *testing.T) {
	ClearRuntimeKernelExecutionStats()
	t.Cleanup(ClearRuntimeKernelExecutionStats)

	assertEvalValue(t, "x:1+til 8;scan:+\\x;last scan", int64(36))
	assertEvalValue(t, "x:1+til 8;named:sums x;last named", int64(36))
	assertEvalValue(t, "x:1+til 5;p:prds x;last p", int64(120))
	assertEvalValue(t, "x:5 4 0N 2;m:mins x;last m", int64(2))
	assertEvalValue(t, "x:5 4 0N 7;m:maxs x;last m", int64(7))
	assertEvalValue(t, "x:1 2 0N 5;a:avgs x;last a", float64(8)/3)
	assertEvalArray(t, "x:1 2 3;scan:+\\x;scan", data.KindI64, []any{int64(1), int64(3), int64(6)})
	assertEvalValue(t, "x:(til 8) mod 3;scan:+\\x;last scan+count scan", int64(15))
	assertEvalValue(t, "x:1+til 8;scan:+\\x;scan:10 20;last scan", int64(20))

	seen := false
	for _, stat := range RuntimeKernelExecutionStats() {
		if stat.Kernel == "ArrayLastScanView" && stat.Outcome == "hit" && stat.ReasonCode == "typed_kernel" && stat.Count > 0 {
			seen = true
			break
		}
	}
	if !seen {
		t.Fatalf("missing ArrayLastScanView typed runtime stat: %#v", RuntimeKernelExecutionStats())
	}
}

func TestEvalSymbolVectorAndDictionary(t *testing.T) {
	assertEvalValue(t, "`AAPL", data.Symbol("AAPL"))
	assertEvalArray(t, "`AAPL`MSFT`NVDA", data.KindSymbol, []any{
		data.Symbol("AAPL"),
		data.Symbol("MSFT"),
		data.Symbol("NVDA"),
	})

	got, err := Eval("`sym`price!(`AAPL`MSFT;100 101)")
	if err != nil {
		t.Fatalf("Eval returned error: %v", err)
	}
	dict, ok := got.(EvalDict)
	if !ok {
		t.Fatalf("dict = %#v", got)
	}
	if !reflect.DeepEqual(dict.Keys, []any{data.Symbol("sym"), data.Symbol("price")}) {
		t.Fatalf("keys = %#v", dict.Keys)
	}
	if len(dict.Values) != 2 {
		t.Fatalf("values = %#v", dict.Values)
	}
	syms, ok := dict.Values[0].(data.Array)
	if !ok || syms.Kind() != data.KindSymbol {
		t.Fatalf("sym value = %#v", dict.Values[0])
	}
	prices, ok := dict.Values[1].(data.Array)
	if !ok || prices.Kind() != data.KindI64 {
		t.Fatalf("price value = %#v", dict.Values[1])
	}
}

func TestEvalTableFlipDictAndKeyedColumnOrder(t *testing.T) {
	assertEvalArray(t, "keys `price`sym`size!(100 101;`AAPL`MSFT;10 20)", data.KindSymbol, []any{
		data.Symbol("price"),
		data.Symbol("sym"),
		data.Symbol("size"),
	})
	assertEvalArray(t, "cols flip `price`sym`size!(100 101;`AAPL`MSFT;10 20)", data.KindSymbol, []any{
		data.Symbol("price"),
		data.Symbol("sym"),
		data.Symbol("size"),
	})
	assertEvalArray(t, "cols ([] price:100 101; sym:`AAPL`MSFT; size:10 20)", data.KindSymbol, []any{
		data.Symbol("price"),
		data.Symbol("sym"),
		data.Symbol("size"),
	})
	assertEvalArray(t, "keys ([venue:`XNYS`XNAS; sym:`AAPL`MSFT] price:100 101; size:10 20)", data.KindSymbol, []any{
		data.Symbol("venue"),
		data.Symbol("sym"),
	})
	assertEvalArray(t, "cols ([venue:`XNYS`XNAS; sym:`AAPL`MSFT] price:100 101; size:10 20)", data.KindSymbol, []any{
		data.Symbol("venue"),
		data.Symbol("sym"),
		data.Symbol("price"),
		data.Symbol("size"),
	})

	got, err := Eval("([venue:`XNYS`XNAS; sym:`AAPL`MSFT] price:100 101; size:10 20)")
	if err != nil {
		t.Fatalf("Eval(keyed table literal) returned error: %v", err)
	}
	keyed, ok := got.(data.KeyedFrame)
	if !ok {
		t.Fatalf("Eval(keyed table literal) = %#v, want data.KeyedFrame", got)
	}
	if !reflect.DeepEqual(keyed.Frame().Schema().Names(), []data.Symbol{"venue", "sym", "price", "size"}) {
		t.Fatalf("keyed frame schema = %#v", keyed.Frame().Schema().Names())
	}
	valueFrame, err := keyed.ValueFrame()
	if err != nil {
		t.Fatalf("ValueFrame returned error: %v", err)
	}
	if !reflect.DeepEqual(valueFrame.Schema().Names(), []data.Symbol{"price", "size"}) {
		t.Fatalf("value frame schema = %#v", valueFrame.Schema().Names())
	}

	got, err = Eval(" ( [ venue:`XNYS`XNYS`XNAS; sym:`AAPL`AAPL`MSFT; ] price:100 101 80; size:10 11 20; ) ")
	if err != nil {
		t.Fatalf("Eval(spaced keyed table literal) returned error: %v", err)
	}
	keyed, ok = got.(data.KeyedFrame)
	if !ok {
		t.Fatalf("Eval(spaced keyed table literal) = %#v, want data.KeyedFrame", got)
	}
	if !reflect.DeepEqual(keyed.Frame().Schema().Names(), []data.Symbol{"venue", "sym", "price", "size"}) {
		t.Fatalf("spaced keyed frame schema = %#v", keyed.Frame().Schema().Names())
	}
	latest, err := keyed.LookupValueByKey(data.Symbol("XNYS"), data.Symbol("AAPL"))
	if err != nil {
		t.Fatalf("LookupValueByKey returned error: %v", err)
	}
	if latest.Len() != 1 {
		t.Fatalf("latest len = %d, want 1", latest.Len())
	}
	if !reflect.DeepEqual(latest.Schema().Names(), []data.Symbol{"price", "size"}) {
		t.Fatalf("latest value schema = %#v", latest.Schema().Names())
	}
	assertFrameValue(t, latest, "price", 0, int64(101))
	assertFrameValue(t, latest, "size", 0, int64(11))

	assertEvalArray(t, "([venue:`XNYS`XNYS`XNAS; sym:`AAPL`AAPL`MSFT] price:100 101 80; size:10 11 20)[`venue`price]`price", data.KindI64, []any{int64(100), int64(101), int64(80)})
	assertEvalValue(t, "first (([venue:`XNYS`XNYS`XNAS; sym:`AAPL`AAPL`MSFT] price:100 101 80; size:10 11 20)[`XNYS`AAPL])`price", int64(101))
}

func TestEvalEnumCastMetadataAndVectorBehavior(t *testing.T) {
	assertEvalArray(t, "`sym$`AAPL`MSFT", data.KindSymbol, []any{
		data.Symbol("AAPL"),
		data.Symbol("MSFT"),
	})
	assertEvalArray(t, "domain `sym$`AAPL`MSFT`AAPL", data.KindSymbol, []any{
		data.Symbol("AAPL"),
		data.Symbol("MSFT"),
	})
	assertEvalArray(t, "codes `sym$`AAPL`MSFT`AAPL", data.KindI64, []any{int64(0), int64(1), int64(0)})
	assertEvalValue(t, "count `sym$`AAPL`MSFT", int64(2))
	assertEvalValue(t, "(`sym$`AAPL`MSFT)[1]", data.Symbol("MSFT"))
	assertEvalArray(t, "1#`sym$`AAPL`MSFT", data.KindSymbol, []any{data.Symbol("AAPL")})
	assertEvalArray(t, "codes 3#`sym$`AAPL`MSFT`AAPL", data.KindI64, []any{int64(0), int64(1), int64(0)})
	assertEvalArray(t, "domain 1#`sym$`AAPL`MSFT`AAPL", data.KindSymbol, []any{
		data.Symbol("AAPL"),
		data.Symbol("MSFT"),
	})
	assertEvalArray(t, "value `sym$`AAPL`MSFT", data.KindSymbol, []any{
		data.Symbol("AAPL"),
		data.Symbol("MSFT"),
	})
	assertEvalArray(t, "keys `sym$`AAPL`MSFT", data.KindSymbol, []any{
		data.Symbol("domain"),
		data.Symbol("value"),
	})
	assertEvalValue(t, "`sym$`AAPL`MSFT~`sym$`AAPL`MSFT", true)
	assertEvalValue(t, "`sym$`AAPL`MSFT`AAPL~`sym$`AAPL`MSFT`AAPL", true)
	assertEvalValue(t, "`sym$`AAPL`MSFT`AAPL~`sym$`MSFT`AAPL`MSFT", false)
	assertEvalValue(t, "`sym$`AAPL`MSFT~`venue$`AAPL`MSFT", false)
	assertEvalValue(t, "`sym$`AAPL`MSFT~`AAPL`MSFT", false)
	assertEvalValue(t, "attr 10 20 30", data.Symbol(""))
	assertEvalValue(t, "attr `s#10 20 30", data.Symbol("s"))
	assertEvalValue(t, "attr `g#`AAPL`MSFT`AAPL", data.Symbol("g"))
	assertEvalValue(t, "attr asc `s#30 10 20", data.Symbol("s"))
	assertEvalValue(t, "attr reverse `g#`AAPL`MSFT`AAPL", data.Symbol("g"))
	assertEvalValue(t, "attr 2#`u#`AAPL`MSFT`NVDA", data.Symbol("u"))
	assertEvalValue(t, "attr 1_(`p#`AAPL`MSFT`NVDA)", data.Symbol("p"))
	assertEvalValue(t, "attr 1 rotate `s#10 20 30", data.Symbol("s"))

	got, err := Eval("meta `sym$`AAPL`MSFT")
	if err != nil {
		t.Fatalf("Eval(meta enum) returned error: %v", err)
	}
	dict, ok := got.(EvalDict)
	if !ok {
		t.Fatalf("meta enum = %#v, want EvalDict", got)
	}
	if !reflect.DeepEqual(dict.Keys, []any{data.Symbol("domain"), data.Symbol("type"), data.Symbol("count")}) {
		t.Fatalf("meta enum keys = %#v", dict.Keys)
	}
	if !reflect.DeepEqual(dict.Values, []any{data.Symbol("sym"), int64(11), int64(2)}) {
		t.Fatalf("meta enum values = %#v", dict.Values)
	}
	assertEvalValue(t, "(meta `sym$`AAPL`MSFT)`domain", data.Symbol("sym"))
	assertEvalValue(t, "(meta `sym$`AAPL`MSFT)`type", int64(11))
	assertEvalValue(t, "(meta `sym$`AAPL`MSFT)`count", int64(2))
	assertEvalErrorContains(t, "`sym$100 101", "expects symbol values")
	assertEvalErrorContains(t, "codes `AAPL`MSFT", "encoded vector")
	assertEvalErrorContains(t, "domain `AAPL`MSFT", "encoded vector")
}

func TestEvalTypedCasts(t *testing.T) {
	assertEvalValue(t, "`short$1", int16(1))
	assertEvalValue(t, "`int$1", int32(1))
	assertEvalValue(t, "`long$1", int64(1))
	assertEvalValue(t, "`real$1.25", float32(1.25))
	assertEvalValue(t, "`float$1", float64(1))
	assertEvalValue(t, "$\"AAPL\"", data.Symbol("AAPL"))
	assertEvalValue(t, "`symbol$\"AAPL\"", data.Symbol("AAPL"))
	assertEvalValue(t, "`$\"AAPL\"", data.Symbol("AAPL"))
	assertEvalValue(t, "`string$`AAPL", "AAPL")

	assertEvalArray(t, "`int$1 2 0N", data.KindI32, []any{int32(1), int32(2), data.NullValue})
	assertEvalArray(t, "`real$1 2 0Nf", data.KindF32, []any{float32(1), float32(2), data.NullValue})
	assertEvalArray(t, "`float$1 2 0Ni", data.KindF64, []any{1.0, 2.0, data.NullValue})
	assertEvalArray(t, "`symbol$(\"AAPL\";\"MSFT\")", data.KindSymbol, []any{data.Symbol("AAPL"), data.Symbol("MSFT")})
	assertEvalArray(t, "`string$`AAPL`MSFT", data.KindString, []any{"AAPL", "MSFT"})

	assertEvalValue(t, "type `int$1", int64(-6))
	assertEvalValue(t, "type `byte$1", int64(-4))
	assertEvalValue(t, "type x$1", int64(-4))
	assertEvalValue(t, "type `real$1", int64(-8))
	assertEvalValue(t, "type `float$1", int64(-9))
	assertEvalValue(t, "type `boolean$0N", int64(-1))
	assertEvalValue(t, "type `byte$0N", int64(-4))
	assertEvalValue(t, "type `char$0N", int64(-10))
	assertEvalValue(t, "type `symbol$\"AAPL\"", int64(-11))
	assertEvalValue(t, "type i$3", int64(-6))
	assertEvalValue(t, "type f$0N", int64(-9))
	assertEvalValue(t, "type e$0N", int64(-8))
	assertEvalValue(t, "type p$0N", int64(-12))
	assertEvalValue(t, "s$\"AAPL\"", data.Symbol("AAPL"))
	assertEvalValue(t, "c$`AAPL", "AAPL")
	assertEvalValue(t, "\"J\"$\"42\"", int64(42))
	assertEvalValue(t, "\"I\"$\"42\"", int32(42))
	assertEvalValue(t, "\"F\"$\"3.5\"", float64(3.5))
	assertEvalValue(t, "\"E\"$\"3.5\"", float32(3.5))
	assertEvalValue(t, "long$\"42\"", int64(42))
	assertEvalValue(t, "int$\"42\"", int32(42))
	assertEvalValue(t, "float$\"3.5\"", float64(3.5))
	// Long/int atom $ string is canonical pad (see the canonical-q corpus);
	// the dialect's type-code string parsing lives on short atoms (7h$).
	assertEvalValue(t, "7$\"42\"", "42     ")
	assertEvalValue(t, "-7$\"42\"", "     42")
	assertEvalValue(t, "7h$\"42\"", int64(42))
	assertEvalValue(t, "-7h$\"42\"", int64(42))
	assertEvalValue(t, "6h$\"42\"", int32(42))
	assertEvalValue(t, "9h$\"3.5\"", float64(3.5))
	assertEvalValue(t, "p$\"1970-01-02T00:00:00.000000001Z\"", data.Timestamp(86_400_000_000_001))
	assertEvalArray(t, "b$1 0 0N", data.KindBool, []any{true, false, data.NullValue})
	assertEvalArray(t, "x$1 2 0N", data.KindU8, []any{uint8(1), uint8(2), data.NullValue})
	assertEvalArray(t, "c$`a`b", data.KindString, []any{"a", "b"})
	assertEvalArray(t, "i$1 2 0N", data.KindI32, []any{int32(1), int32(2), data.NullValue})
	assertEvalArray(t, "7h$\"1\" \"2\"", data.KindI64, []any{int64(1), int64(2)})
	assertEvalArray(t, "-6h$\"1\" \"2\"", data.KindI32, []any{int32(1), int32(2)})
	assertEvalArray(t, "3$\"1\" \"2\"", data.KindString, []any{"1  ", "2  "})
	assertEvalArray(t, "I$(\"1\";\"2\")", data.KindI32, []any{int32(1), int32(2)})
	assertEvalArray(t, "f$1 2 0Ni", data.KindF64, []any{1.0, 2.0, data.NullValue})
	assertEvalArray(t, "float$(\"1.5\";\"2.25\")", data.KindF64, []any{1.5, 2.25})
	assertEvalArray(t, "e$1 2 0Nf", data.KindF32, []any{float32(1), float32(2), data.NullValue})
	assertEvalArray(t, "$(\"AAPL\";\"MSFT\")", data.KindSymbol, []any{data.Symbol("AAPL"), data.Symbol("MSFT")})
	assertEvalArray(t, "`date$\"2026-06-06\" \"2026-06-07\"", data.KindDate, []any{
		data.DateFromDays(20610),
		data.DateFromDays(20611),
	})
	assertEvalArray(t, "`venue$`XNYS`XNAS`XNYS", data.KindSymbol, []any{
		data.Symbol("XNYS"),
		data.Symbol("XNAS"),
		data.Symbol("XNYS"),
	})

	assertEvalErrorContains(t, "`short$40000", "q cast")
	// Canonical q integer casts round half-to-even (1.5 -> 2).
	assertEvalValue(t, "`int$1.5", int32(2))
	// Canonical q: a failed string-to-number parse yields the target null.
	if got, err := Eval("\"I\"$\"42.5\""); err != nil || !data.IsNull(got) {
		t.Errorf("Eval(%q) = %#v, %v; want 0Ni (failed parses cast to null)", "\"I\"$\"42.5\"", got, err)
	}
}

func TestEvalDictionaryLookup(t *testing.T) {
	assertEvalValue(t, "lookup (`a`b!10 20) `b", int64(20))
	assertEvalValue(t, "(`a`b!10 20)`a", int64(10))
	assertEvalValue(t, "(`a`b!10 20)[`b]", int64(20))
	assertEvalArray(t, "lookup (`a`b`c!10 20 30) `c`a", data.KindI64, []any{int64(30), int64(10)})
	assertEvalValue(t, "lookup (`a`b!10 20) `c", data.NullValue)
	assertEvalValue(t, "lookup (0N 1!10 20) 0N", int64(10))
	assertEvalValue(t, "lookup (1 2!`one`two) 2", data.Symbol("two"))
	assertEvalArray(t, "lookup (10 20 30!100 200 300) 30 10", data.KindI64, []any{int64(300), int64(100)})
}

func TestEvalScriptAssignmentAndSequentialEvaluation(t *testing.T) {
	assertEvalValue(t, "x:10 20 30;+/x", int64(60))
	assertEvalArray(t, "x:10 20 30;y:x+1;y", data.KindI64, []any{int64(11), int64(21), int64(31)})
	assertEvalValue(t, "sym:`AAPL`MSFT;px:100 101;d:`sym`px!(sym;px);(d`px)[1]", int64(101))
	assertEvalValue(t, "x:1;y:2;x+y", int64(3))
}

func TestEvalParsedValueExprUsesScriptBindings(t *testing.T) {
	assertEvalArray(t, "x:10;y:20;x y", data.KindI64, []any{int64(10), int64(20)})
	assertEvalArray(t, "a:`AAPL;b:`MSFT;a b", data.KindSymbol, []any{data.Symbol("AAPL"), data.Symbol("MSFT")})

	date0 := data.DateFromDays(time.Date(2026, 6, 6, 0, 0, 0, 0, time.UTC).Unix() / 86400)
	date1 := data.DateFromDays(time.Date(2026, 6, 7, 0, 0, 0, 0, time.UTC).Unix() / 86400)
	assertEvalArray(t, "d0:2026.06.06;d1:2026.06.07;d0 d1", data.KindDate, []any{date0, date1})

	assertEvalValue(t, "x:10;y:20;d:`x`y!(x;y);d[`y]", int64(20))
	assertEvalValue(t, "x:10;y:20;x<y", true)
}

func TestEvalScriptWarmBindingsKeepFastAndSemanticPathsSeparate(t *testing.T) {
	state := NewEvalState(nil)

	rangePlan := state.qScriptPlan("x:10*til 8;probe:til 80;+/x bin probe")
	if len(rangePlan.statements) != 3 {
		t.Fatalf("range script statements = %d, want 3", len(rangePlan.statements))
	}
	if rangePlan.statements[0].bindingPlan.kind == qScriptBindingInvalid ||
		rangePlan.statements[1].bindingPlan.kind == qScriptBindingInvalid {
		t.Fatalf("range assignments missing warm bindings: %#v", rangePlan.statements)
	}
	assertStateEvalValue(t, state, "x:10*til 8;probe:til 80;+/x bin probe", int64(280))
	assertStateEvalValue(t, state, "x:10*til 8;probe:til 80;+/x bin probe", int64(280))

	takePlan := state.qScriptPlan("dates:9#2026.06.06 2026.06.07 2026.06.08;count where dates>=2026.06.07")
	if len(takePlan.statements) != 2 || takePlan.statements[0].bindingPlan.kind == qScriptBindingInvalid {
		t.Fatalf("temporal take assignment missing warm binding: %#v", takePlan.statements)
	}
	assertStateEvalValue(t, state, "dates:9#2026.06.06 2026.06.07 2026.06.08;count where dates>=2026.06.07", int64(6))

	dropPlan := state.qPipelinePlan("sum drop 4 x")
	if dropPlan.kind != qPipelineSumVectorExpr || dropPlan.reductionPlan.kind == qScriptBindingInvalid {
		t.Fatalf("drop transform pipeline missing warm binding: %#v", dropPlan)
	}
	assertStateEvalValue(t, state, "x:til 16;sum drop 4 x", int64(114))

	rotatePlan := state.qPipelinePlan("sum 5 rotate x")
	if rotatePlan.kind != qPipelineSumSequenceTransform || rotatePlan.leftPlan.kind == qScriptBindingInvalid || rotatePlan.reductionPlan.kind == qScriptBindingInvalid {
		t.Fatalf("rotate transform pipeline missing warm binding: %#v", rotatePlan)
	}
	assertStateEvalValue(t, state, "x:til 16;sum 5 rotate x", int64(120))

	for _, src := range []string{
		"`sym$`AAPL`MSFT",
		"`s#10 20 30",
		"-0D00:01:00",
	} {
		plan := state.qScriptPlan(src)
		if len(plan.statements) != 1 {
			t.Fatalf("%q statements = %d, want 1", src, len(plan.statements))
		}
		if plan.statements[0].bindingPlan.kind != qScriptBindingInvalid {
			t.Fatalf("%q should stay on semantic string path, got warm binding %#v", src, plan.statements[0].bindingPlan)
		}
	}
}

func TestEvalLogicalDyadicSymbols(t *testing.T) {
	// Canonical q: `&`/`and` is elementwise minimum, `|`/`or` is maximum;
	// integer operands stay numeric while boolean operands keep their
	// logical-and/or behavior (min/max on booleans IS and/or).
	assertEvalArray(t, "1 0 1 & 1 1 0", data.KindI64, []any{int64(1), int64(0), int64(0)})
	assertEvalArray(t, "1 0 1 | 0 1 0", data.KindI64, []any{int64(1), int64(1), int64(1)})
	assertEvalArray(t, "true false true & true true false", data.KindBool, []any{true, false, false})
	assertEvalArray(t, "true false true | false true false", data.KindBool, []any{true, true, true})
	assertEvalArray(t, "1 & 1 0 1", data.KindI64, []any{int64(1), int64(0), int64(1)})
	assertEvalArray(t, "0 | 1 0 1", data.KindI64, []any{int64(1), int64(0), int64(1)})
	assertEvalArray(t, "1 0 1 and 1 1 0", data.KindI64, []any{int64(1), int64(0), int64(0)})
	assertEvalArray(t, "1 0 1 or 0 1 0", data.KindI64, []any{int64(1), int64(1), int64(1)})
	assertEvalArray(t, "2 & 1 5 2", data.KindI64, []any{int64(1), int64(2), int64(2)})
	assertEvalArray(t, "1 5 2 | 3", data.KindI64, []any{int64(3), int64(5), int64(3)})
}

// TestEvalCanonicalMinMaxLogical pins the canonical q semantics of `&`/`|`
// and their word aliases: elementwise min/max with null ordering (nulls are
// smallest), bool/numeric promotion, and intact boolean mask pipelines.
func TestEvalCanonicalMinMaxLogical(t *testing.T) {
	for _, tc := range []struct{ src, want string }{
		{"2&3", "2"},
		{"2|3", "3"},
		{"0N&5", "0N"},
		{"0N|5", "5"},
		{"1b&0b", "0b"},
		{"1b|0b", "1b"},
		{"&/1 2 3", "1"},
		{"|/1 2 3", "3"},
		{"2 and 3", "2"},
		{"2 or 3", "3"},
		{"1b&2", "1"},
		{"0b|0", "0"},
		{"2.5|1 2 3", "2.5 2.5 3"},
		{"0.5&1 2 3", "0.5 0.5 0.5"},
		// Boolean compare masks keep the lazy logical-mask pipelines.
		{"x:til 10;count where (x>1)&(x<9)", "7"},
		{"x:til 10;m:(x>1)&(x<9);m~(x>1) and (x<9)", "1b"},
		// `where` over a 0/1 min-mask replicates indexes like a bool mask.
		{"x:til 12;a:x mod 2;b:x mod 3;count where a & b", "4"},
	} {
		got, err := Eval(tc.src)
		if err != nil {
			t.Errorf("Eval(%q) returned error: %v", tc.src, err)
			continue
		}
		want, err := Eval(tc.want)
		if err != nil {
			t.Errorf("Eval(%q) returned error: %v", tc.want, err)
			continue
		}
		if !matchValue(got, want) {
			t.Errorf("Eval(%q) = %v (%T), want %v (%T)", tc.src, got, got, want, want)
		}
	}
}

func TestEvalParsedValueExprRunsCallsInNestedCombinations(t *testing.T) {
	assertEvalArray(t, "x:10 20 30;(count x;sum x)", data.KindI64, []any{int64(3), int64(60)})
	assertEvalValue(t, "x:10 20 30;(count x;sum x)[1]", int64(60))
	assertEvalValue(t, "x:10 20 30;d:`n`s!(count x;sum x);d[`s]", int64(60))
	assertEvalArray(t, "x:10 20 30;(xbar 10 x)", data.KindI64, []any{int64(10), int64(20), int64(30)})
	assertEvalValue(t, "x:10 20 30;d:`bucket`total!((xbar 10 x);sum x);(d`bucket)[2]", int64(30))
}

func TestEvalStateAndEvalWithEnvAreExplicit(t *testing.T) {
	got, err := EvalWithEnv("f:{x+a};a:20;f[1]", map[string]any{"a": int64(10)})
	if err != nil {
		t.Fatalf("EvalWithEnv returned error: %v", err)
	}
	if got != int64(11) {
		t.Fatalf("EvalWithEnv closure = %#v, want 11", got)
	}

	state := NewEvalState(map[string]any{"a": int64(3)})
	assertStateEvalValue(t, state, "a+1", int64(4))
	assertStateEvalValue(t, state, "a:10", int64(10))
	assertStateEvalValue(t, state, "a+1", int64(11))

	var wg sync.WaitGroup
	errs := make(chan string, 32)
	for i := 0; i < cap(errs); i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := EvalWithEnv("f:{x+a};a:999;f[1]", map[string]any{"a": int64(i)})
			if err != nil {
				errs <- err.Error()
				return
			}
			if got != int64(i+1) {
				errs <- "unexpected concurrent EvalWithEnv value"
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
}

func TestEvalCacheabilityClassification(t *testing.T) {
	cacheableSources := []string{
		"1+2",
		"`sym`price!( `AAPL`MSFT; 100 101)",
		`"\\p;not a system command"`,
		"(1;2;3)",
	}
	for _, src := range cacheableSources {
		if !EvalSourceCacheable(src) {
			t.Fatalf("EvalSourceCacheable(%q) = false, want true", src)
		}
	}

	uncacheableSources := []string{
		"",
		"\\p",
		"1+2;\\p",
		".z.P",
		"h:hopen \"loopback\";h[\"1+2\"]",
		"get `:/tmp/q/table",
		"`:/tmp/q/table set ([] a:1 2)",
		"system \"p\"",
	}
	for _, src := range uncacheableSources {
		if EvalSourceCacheable(src) {
			t.Fatalf("EvalSourceCacheable(%q) = true, want false", src)
		}
	}

	if !EvalValueCacheable(Dict{Keys: []any{data.Symbol("a")}, Values: []any{int64(1)}}) {
		t.Fatal("EvalValueCacheable(Dict) = false, want true")
	}
	if !EvalValueCacheable(data.NewI64([]int64{1, 2, 3})) {
		t.Fatal("EvalValueCacheable(data.Array) = false, want true")
	}
	if EvalValueCacheable(qLambda{body: "x+1"}) {
		t.Fatal("EvalValueCacheable(qLambda) = true, want false")
	}
}

func TestEvalStateScriptPlanCachePreservesEnvironmentSemantics(t *testing.T) {
	state := NewEvalState(nil)
	if got, err := state.Eval("x:1;y:x+2;y"); err != nil || got != int64(3) {
		t.Fatalf("first Eval returned %#v, %v; want 3,nil", got, err)
	}
	if got, err := state.Eval("x:10;y:x+2;y"); err != nil || got != int64(12) {
		t.Fatalf("second Eval returned %#v, %v; want 12,nil", got, err)
	}
	if len(state.scriptCache) == 0 {
		t.Fatal("EvalState script plan cache was not populated")
	}
	if got, err := state.Eval("x:20;y:x+2;y"); err != nil || got != int64(22) {
		t.Fatalf("third Eval returned %#v, %v; want 22,nil", got, err)
	}
}

func TestEvalScriptExecutableSingleStatementPreservesEnvironmentSemantics(t *testing.T) {
	state := NewEvalState(map[string]any{"x": data.NewI64([]int64{10, 20, 30})})
	if got, err := state.Eval("x@1"); err != nil || got != int64(20) {
		t.Fatalf("first executable Eval returned %#v, %v; want 20,nil", got, err)
	}
	plan := state.scriptCache["x@1"]
	if plan.executable == nil || plan.executable.kind != qScriptExecutableSingleStatement {
		t.Fatalf("cached plan executable = %#v, want single-statement executable", plan.executable)
	}
	state.env["x"] = data.NewI64([]int64{100, 200, 300})
	if got, err := state.Eval("x@1"); err != nil || got != int64(200) {
		t.Fatalf("warm executable Eval returned %#v, %v; want 200,nil", got, err)
	}
}

func TestEvalScriptExecutablePipelineBackendWarmPath(t *testing.T) {
	state := NewEvalState(nil)
	src := "x:1+til 8;s:+\\x;({x+y}/[10;x])+last s+count s"
	if got, err := state.Eval(src); err != nil || got != int64(90) {
		t.Fatalf("first executable pipeline Eval returned %#v, %v; want 90,nil", got, err)
	}
	plan := state.scriptCache[src]
	if plan.executable == nil || plan.executable.kind != qScriptExecutablePipelineBackend || !plan.executable.pipeline.Valid() {
		t.Fatalf("cached executable = %#v, want valid pipeline backend", plan.executable)
	}
	if got, err := state.Eval(src); err != nil || got != int64(90) {
		t.Fatalf("warm executable pipeline Eval returned %#v, %v; want 90,nil", got, err)
	}
}

func TestEvalSessionWarmCachePreservesEnvironmentSemantics(t *testing.T) {
	session := NewEvalSession(map[string]any{
		"v":         data.NewI64([]int64{1, 2, 3, 4, 5}),
		"threshold": int64(2),
	})
	src := "idx:where v>threshold;+/v[idx]"
	if got, err := session.Eval(src); err != nil || got != int64(12) {
		t.Fatalf("cold session Eval returned %#v, %v; want 12,nil", got, err)
	}
	entry, ok := session.cache[strings.TrimSpace(src)]
	if !ok {
		t.Fatal("session plan cache was not populated")
	}
	if !entry.executable.Valid() || entry.descriptor.Kind != "script" || entry.backend.Backend != EvalPipelineTypedRuntimeBackend {
		t.Fatalf("session cached entry = %#v, want script descriptor/backend executable", entry)
	}
	session.state.env["threshold"] = int64(3)
	if got, err := session.Eval(src); err != nil || got != int64(9) {
		t.Fatalf("warm session Eval returned %#v, %v; want 9,nil", got, err)
	}
}

func TestEvalSessionWarmCacheDoesNotPoisonErrorPath(t *testing.T) {
	session := NewEvalSession(nil)
	if _, err := session.Eval("x+"); err == nil {
		t.Fatal("cold invalid session Eval succeeded, want error")
	}
	if _, err := session.Eval("x+"); err == nil {
		t.Fatal("warm invalid session Eval succeeded, want error")
	}
	if got, err := session.Eval("x:41;x+1"); err != nil || got != int64(42) {
		t.Fatalf("valid session Eval after error returned %#v, %v; want 42,nil", got, err)
	}
}

func TestEvalGlobalScriptPlanCachePreservesEnvironmentSemantics(t *testing.T) {
	ClearEvalPlanCaches()
	t.Cleanup(ClearEvalPlanCaches)

	src := "x:a+1;y:x*2;y"
	if got, err := EvalWithEnv(src, map[string]any{"a": int64(10)}); err != nil || got != int64(22) {
		t.Fatalf("first EvalWithEnv returned %#v, %v; want 22,nil", got, err)
	}
	afterCold := EvalPlanCacheStatsSnapshot()
	if afterCold.ScriptMisses == 0 || afterCold.ScriptEntries == 0 {
		t.Fatalf("cold eval did not populate global script cache: %#v", afterCold)
	}
	if got, err := EvalWithEnv(src, map[string]any{"a": int64(20)}); err != nil || got != int64(42) {
		t.Fatalf("warm EvalWithEnv returned %#v, %v; want 42,nil", got, err)
	}
	afterWarm := EvalPlanCacheStatsSnapshot()
	if afterWarm.ScriptHits <= afterCold.ScriptHits {
		t.Fatalf("warm eval did not hit global script cache: before=%#v after=%#v", afterCold, afterWarm)
	}
	if afterWarm.ScriptEntries != afterCold.ScriptEntries {
		t.Fatalf("warm eval changed script cache entries: before=%#v after=%#v", afterCold, afterWarm)
	}
}

func TestPreparedEvalPreservesEnvironmentSemantics(t *testing.T) {
	ClearEvalPlanCaches()
	t.Cleanup(ClearEvalPlanCaches)

	prepared, err := PrepareEval("x:a+1;y:x*2;y")
	if err != nil {
		t.Fatalf("PrepareEval returned error: %v", err)
	}
	if prepared.Source() != "x:a+1;y:x*2;y" {
		t.Fatalf("prepared source = %q", prepared.Source())
	}
	if got, err := prepared.EvalWithEnv(map[string]any{"a": int64(10)}); err != nil || got != int64(22) {
		t.Fatalf("first prepared EvalWithEnv returned %#v, %v; want 22,nil", got, err)
	}
	if got, err := prepared.EvalWithEnv(map[string]any{"a": int64(20)}); err != nil || got != int64(42) {
		t.Fatalf("warm prepared EvalWithEnv returned %#v, %v; want 42,nil", got, err)
	}
}

func TestPreparedEvalPipelineExecutablePreservesEnvironmentSemantics(t *testing.T) {
	ClearEvalPlanCaches()
	t.Cleanup(ClearEvalPlanCaches)

	src := "i:where v>threshold;+/v[i]"
	prepared, err := PrepareEval(src)
	if err != nil {
		t.Fatalf("PrepareEval returned error: %v", err)
	}
	if prepared.entry == nil || !prepared.entry.executable.Valid() {
		t.Fatalf("prepared entry executable = %#v, want valid executable", prepared.entry)
	}
	env1 := map[string]any{"v": data.NewI64([]int64{1, 2, 3, 4}), "threshold": int64(2)}
	if got, err := prepared.EvalWithEnv(env1); err != nil || got != int64(7) {
		t.Fatalf("first prepared pipeline returned %#v, %v; want 7,nil", got, err)
	}
	env2 := map[string]any{"v": data.NewI64([]int64{10, 20, 30}), "threshold": int64(15)}
	if got, err := prepared.EvalWithEnv(env2); err != nil || got != int64(50) {
		t.Fatalf("warm prepared pipeline returned %#v, %v; want 50,nil", got, err)
	}
}

func TestPreparedEvalRejectsUncacheableSource(t *testing.T) {
	for _, src := range []string{"", "rand 3", "\\p", "system \"p\"", "get `:/tmp/q/table"} {
		if prepared, err := PrepareEval(src); err == nil || prepared != nil {
			t.Fatalf("PrepareEval(%q) = %#v,%v; want nil,error", src, prepared, err)
		}
	}
	if got := (*PreparedEval)(nil).Source(); got != "" {
		t.Fatalf("nil PreparedEval Source = %q, want empty", got)
	}
	if _, err := (*PreparedEval)(nil).EvalWithEnv(nil); err == nil {
		t.Fatal("nil PreparedEval EvalWithEnv succeeded, want error")
	}
}

func TestPreparedEvalDoesNotMutateHostEnv(t *testing.T) {
	prepared, err := PrepareEval("x:2;y:3;x+y")
	if err != nil {
		t.Fatalf("PrepareEval returned error: %v", err)
	}
	env := map[string]any{"x": int64(1)}
	if got, err := prepared.EvalWithEnv(env); err != nil || got != int64(5) {
		t.Fatalf("prepared EvalWithEnv returned %#v, %v; want 5,nil", got, err)
	}
	if got := env["x"]; got != int64(1) {
		t.Fatalf("prepared EvalWithEnv mutated host env x = %#v, want 1", got)
	}
	if _, ok := env["y"]; ok {
		t.Fatal("prepared EvalWithEnv leaked y into host env")
	}
}

func TestEvalGlobalPipelinePlanCachePreservesEnvironmentSemantics(t *testing.T) {
	ClearEvalPlanCaches()
	t.Cleanup(ClearEvalPlanCaches)

	src := "+/v where v>threshold"
	env1 := map[string]any{"v": data.NewI64([]int64{1, 2, 3, 4}), "threshold": int64(2)}
	if got, err := EvalWithEnv(src, env1); err != nil || got != int64(7) {
		t.Fatalf("first pipeline EvalWithEnv returned %#v, %v; want 7,nil", got, err)
	}
	afterCold := EvalPlanCacheStatsSnapshot()
	if afterCold.PipelineMisses == 0 || afterCold.PipelineEntries == 0 {
		t.Fatalf("cold eval did not populate global pipeline cache: %#v", afterCold)
	}
	env2 := map[string]any{"v": data.NewI64([]int64{10, 20, 30}), "threshold": int64(15)}
	if got, err := EvalWithEnv(src, env2); err != nil || got != int64(50) {
		t.Fatalf("warm pipeline EvalWithEnv returned %#v, %v; want 50,nil", got, err)
	}
	afterWarm := EvalPlanCacheStatsSnapshot()
	if afterWarm.PipelineHits <= afterCold.PipelineHits {
		t.Fatalf("warm eval did not hit global pipeline cache: before=%#v after=%#v", afterCold, afterWarm)
	}
}

func TestEvalGlobalPipelinePlanCacheStoresNegativeCandidates(t *testing.T) {
	ClearEvalPlanCaches()
	t.Cleanup(ClearEvalPlanCaches)

	src := "+/sum v fby g"
	env := map[string]any{
		"v": data.NewI64([]int64{1, 2, 3, 4}),
		"g": data.NewSymbols([]string{"a", "a", "b", "b"}),
	}
	if got, err := EvalWithEnv(src, env); err != nil || got != int64(20) {
		t.Fatalf("cold EvalWithEnv returned %#v,%v; want 20,nil", got, err)
	}
	afterCold := EvalPlanCacheStatsSnapshot()
	if afterCold.PipelineEntries == 0 || afterCold.PipelineMisses == 0 {
		t.Fatalf("cold eval did not cache negative pipeline candidate: %#v", afterCold)
	}
	if got, err := EvalWithEnv(src, env); err != nil || got != int64(20) {
		t.Fatalf("warm EvalWithEnv returned %#v,%v; want 20,nil", got, err)
	}
	afterWarm := EvalPlanCacheStatsSnapshot()
	if afterWarm.PipelineHits <= afterCold.PipelineHits {
		t.Fatalf("warm negative pipeline candidate did not hit cache: before=%#v after=%#v", afterCold, afterWarm)
	}
	if afterWarm.PipelineEntries != afterCold.PipelineEntries {
		t.Fatalf("warm negative pipeline candidate changed cache entries: before=%#v after=%#v", afterCold, afterWarm)
	}
}

func TestEvalWithEnvDoesNotMutateHostEnv(t *testing.T) {
	env := map[string]any{"x": int64(1)}
	if got, err := EvalWithEnv("x:2;x", env); err != nil || got != int64(2) {
		t.Fatalf("EvalWithEnv assignment returned %#v, %v; want 2,nil", got, err)
	}
	if got := env["x"]; got != int64(1) {
		t.Fatalf("EvalWithEnv mutated host env x = %#v, want 1", got)
	}
	if _, ok := env["y"]; ok {
		t.Fatalf("EvalWithEnv leaked new binding y into host env")
	}
	if got, err := EvalWithEnv("y:3;y", env); err != nil || got != int64(3) {
		t.Fatalf("EvalWithEnv new assignment returned %#v, %v; want 3,nil", got, err)
	}
	if _, ok := env["y"]; ok {
		t.Fatalf("EvalWithEnv leaked new binding y into host env after new assignment")
	}
}

func TestEvalGlobalScriptPipelinePlanCachePreservesEnvironmentSemantics(t *testing.T) {
	ClearEvalPlanCaches()
	t.Cleanup(ClearEvalPlanCaches)

	src := "i:where v>threshold;+/v[i]"
	env1 := map[string]any{"v": data.NewI64([]int64{1, 2, 3, 4}), "threshold": int64(2)}
	if got, err := EvalWithEnv(src, env1); err != nil || got != int64(7) {
		t.Fatalf("first script-pipeline EvalWithEnv returned %#v, %v; want 7,nil", got, err)
	}
	afterCold := EvalPlanCacheStatsSnapshot()
	if afterCold.ScriptMisses == 0 || afterCold.ScriptEntries == 0 {
		t.Fatalf("cold eval did not populate global script cache: %#v", afterCold)
	}
	env2 := map[string]any{"v": data.NewI64([]int64{10, 20, 30}), "threshold": int64(15)}
	if got, err := EvalWithEnv(src, env2); err != nil || got != int64(50) {
		t.Fatalf("warm script-pipeline EvalWithEnv returned %#v, %v; want 50,nil", got, err)
	}
	afterWarm := EvalPlanCacheStatsSnapshot()
	if afterWarm.ScriptHits <= afterCold.ScriptHits {
		t.Fatalf("warm eval did not hit global script cache: before=%#v after=%#v", afterCold, afterWarm)
	}
}

func TestEvalGlobalPipelineBindingCacheKeysByOperandKind(t *testing.T) {
	ClearEvalPlanCaches()
	t.Cleanup(ClearEvalPlanCaches)

	src := "sum reverse v"
	envI64 := map[string]any{"v": data.NewI64([]int64{1, 2, 3})}
	if got, err := EvalWithEnv(src, envI64); err != nil || got != int64(6) {
		t.Fatalf("first i64 EvalWithEnv returned %#v, %v; want 6,nil", got, err)
	}
	afterI64 := EvalPlanCacheStatsSnapshot()
	if afterI64.PipelineBindingMisses == 0 || afterI64.PipelineBindingStores == 0 || afterI64.PipelineBindingEntries != 1 {
		t.Fatalf("i64 eval did not populate typed pipeline binding cache: %#v", afterI64)
	}

	envF64 := map[string]any{"v": data.NewF64([]float64{1.5, 2.5, 3.5})}
	if got, err := EvalWithEnv(src, envF64); err != nil || got != 7.5 {
		t.Fatalf("f64 EvalWithEnv returned %#v, %v; want 7.5,nil", got, err)
	}
	afterF64 := EvalPlanCacheStatsSnapshot()
	if afterF64.PipelineBindingEntries != 2 {
		t.Fatalf("typed pipeline binding cache reused i64 binding for f64: before=%#v after=%#v", afterI64, afterF64)
	}
	if afterF64.PipelineBindingMisses <= afterI64.PipelineBindingMisses {
		t.Fatalf("f64 eval did not record a distinct binding miss: before=%#v after=%#v", afterI64, afterF64)
	}

	if got, err := EvalWithEnv(src, envI64); err != nil || got != int64(6) {
		t.Fatalf("warm i64 EvalWithEnv returned %#v, %v; want 6,nil", got, err)
	}
	afterWarmI64 := EvalPlanCacheStatsSnapshot()
	if afterWarmI64.PipelineBindingHits <= afterF64.PipelineBindingHits {
		t.Fatalf("warm i64 eval did not hit typed pipeline binding cache: before=%#v after=%#v", afterF64, afterWarmI64)
	}
	if afterWarmI64.PipelineBindingEntries != afterF64.PipelineBindingEntries {
		t.Fatalf("warm i64 eval changed binding cache entries: before=%#v after=%#v", afterF64, afterWarmI64)
	}
}

func TestQPipelineWhereCompareBoundMetadata(t *testing.T) {
	plan := &qPipelinePlan{
		compareOp:     ">",
		comparePrefix: "compare-to-index",
	}
	left := data.NewI64([]int64{1, 2, 3})
	right := int64(1)

	kernel, shape, ok := qPipelineWhereCompareBoundMetadata(plan, left, right, qPipelineBoundResultCompareCount, "")
	if !ok || kernel != "ArrayWhereCompareCount" || !strings.HasPrefix(shape, "compare-count/>/i64/i64") {
		t.Fatalf("compare count metadata = %q,%q,%v", kernel, shape, ok)
	}
	kernel, shape, ok = qPipelineWhereCompareBoundMetadata(plan, left, right, qPipelineBoundResultCompareStatsSum, "")
	if !ok || kernel != "ArrayWhereCompareStats" || !strings.HasPrefix(shape, "compare-to-index-stats/>/i64/i64") {
		t.Fatalf("compare stats metadata = %q,%q,%v", kernel, shape, ok)
	}
	kernel, shape, ok = qPipelineWhereCompareBoundMetadata(plan, left, right, qPipelineBoundResultCompareIndexSum, data.KindI64)
	if !ok || kernel != "ArrayWhereCompareSum" || shape != "index-sum/i64" {
		t.Fatalf("compare index sum metadata = %q,%q,%v", kernel, shape, ok)
	}
	if kernel, shape, ok = qPipelineWhereCompareBoundMetadata(plan, left, right, qPipelineBoundResultCompareIndexSum, ""); ok {
		t.Fatalf("empty index result kind metadata = %q,%q,%v; want !ok", kernel, shape, ok)
	}
}

func TestEvalGlobalPipelineBindingCacheMoreShapes(t *testing.T) {
	cases := []struct {
		name  string
		src   string
		i64   any
		f64   any
		envI  map[string]any
		envF  map[string]any
		cache bool
	}{
		{
			name:  "count vector expr",
			src:   "count 4#v",
			i64:   int64(4),
			f64:   int64(4),
			envI:  map[string]any{"v": data.NewI64([]int64{1, 2, 3})},
			envF:  map[string]any{"v": data.NewF64([]float64{1.5, 2.5, 3.5})},
			cache: true,
		},
		{
			name:  "count where compare",
			src:   "count where v>threshold",
			i64:   int64(2),
			f64:   int64(2),
			envI:  map[string]any{"v": data.NewI64([]int64{1, 2, 3, 4}), "threshold": int64(2)},
			envF:  map[string]any{"v": data.NewF64([]float64{1.5, 2.5, 3.5, 4.5}), "threshold": 2.5},
			cache: true,
		},
		{
			name:  "sum where compare",
			src:   "+/where v>threshold",
			i64:   int64(5),
			f64:   int64(5),
			envI:  map[string]any{"v": data.NewI64([]int64{1, 2, 3, 4}), "threshold": int64(2)},
			envF:  map[string]any{"v": data.NewF64([]float64{1.5, 2.5, 3.5, 4.5}), "threshold": 2.5},
			cache: true,
		},
		{
			name: "sum moving window",
			src:  "+/2 msum v",
			i64:  int64(9),
			f64:  11.5,
			envI: map[string]any{"v": data.NewI64([]int64{1, 2, 3})},
			envF: map[string]any{"v": data.NewF64([]float64{1.5, 2.5, 3.5})},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ClearEvalPlanCaches()
			t.Cleanup(ClearEvalPlanCaches)

			if got, err := EvalWithEnv(tc.src, tc.envI); err != nil || !reflect.DeepEqual(got, tc.i64) {
				t.Fatalf("cold i64 EvalWithEnv returned %#v, %v; want %#v,nil", got, err, tc.i64)
			}
			afterI64 := EvalPlanCacheStatsSnapshot()
			if !tc.cache {
				if got, err := EvalWithEnv(tc.src, tc.envF); err != nil || !reflect.DeepEqual(got, tc.f64) {
					t.Fatalf("cold f64 EvalWithEnv returned %#v, %v; want %#v,nil", got, err, tc.f64)
				}
				if got, err := EvalWithEnv(tc.src, tc.envI); err != nil || !reflect.DeepEqual(got, tc.i64) {
					t.Fatalf("warm i64 EvalWithEnv returned %#v, %v; want %#v,nil", got, err, tc.i64)
				}
				afterWarmI64 := EvalPlanCacheStatsSnapshot()
				if afterWarmI64.PipelineBindingEntries != afterI64.PipelineBindingEntries {
					t.Fatalf("non-cached shape changed binding cache entries: before=%#v after=%#v", afterI64, afterWarmI64)
				}
				return
			}
			if afterI64.PipelineBindingMisses == 0 || afterI64.PipelineBindingStores == 0 || afterI64.PipelineBindingEntries != 1 {
				t.Fatalf("cold i64 eval did not populate typed pipeline binding cache: %#v", afterI64)
			}

			if got, err := EvalWithEnv(tc.src, tc.envF); err != nil || !reflect.DeepEqual(got, tc.f64) {
				t.Fatalf("cold f64 EvalWithEnv returned %#v, %v; want %#v,nil", got, err, tc.f64)
			}
			afterF64 := EvalPlanCacheStatsSnapshot()
			if afterF64.PipelineBindingEntries != 2 {
				t.Fatalf("typed pipeline binding cache reused i64 binding for f64: before=%#v after=%#v", afterI64, afterF64)
			}
			if afterF64.PipelineBindingMisses <= afterI64.PipelineBindingMisses {
				t.Fatalf("f64 eval did not record a distinct binding miss: before=%#v after=%#v", afterI64, afterF64)
			}

			if got, err := EvalWithEnv(tc.src, tc.envI); err != nil || !reflect.DeepEqual(got, tc.i64) {
				t.Fatalf("warm i64 EvalWithEnv returned %#v, %v; want %#v,nil", got, err, tc.i64)
			}
			afterWarmI64 := EvalPlanCacheStatsSnapshot()
			if afterWarmI64.PipelineBindingHits <= afterF64.PipelineBindingHits {
				t.Fatalf("warm i64 eval did not hit typed pipeline binding cache: before=%#v after=%#v", afterF64, afterWarmI64)
			}
			if afterWarmI64.PipelineBindingEntries != afterF64.PipelineBindingEntries {
				t.Fatalf("warm i64 eval changed binding cache entries: before=%#v after=%#v", afterF64, afterWarmI64)
			}
		})
	}
}

func TestEvalStatePipelinePlanCachePreservesEnvironmentSemantics(t *testing.T) {
	ClearRuntimeKernelExecutionStats()
	t.Cleanup(ClearRuntimeKernelExecutionStats)

	state := NewEvalState(nil)
	if got, err := state.Eval("x:til 8;y:x*2;idx:where x<4;+/y[idx]"); err != nil || got != int64(12) {
		t.Fatalf("first Eval returned %#v, %v; want 12,nil", got, err)
	}
	if got, err := state.Eval("x:til 8;y:x*2;idx:where x<6;+/y[idx]"); err != nil || got != int64(30) {
		t.Fatalf("second Eval returned %#v, %v; want 30,nil", got, err)
	}
	var plan *qPipelinePlan
	if state.pipelineCache1Src == "+/y[idx]" {
		plan = state.pipelineCache1Plan
	} else if state.pipelineCache != nil {
		plan = state.pipelineCache["+/y[idx]"]
	}
	if plan == nil {
		t.Fatal("EvalState pipeline plan cache was not populated")
	}
	if plan.kind != qPipelineSumGatherIndexes {
		t.Fatalf("cached +/y[idx] plan kind = %v, want qPipelineSumGatherIndexes", plan.kind)
	}
	seenPlanHit := false
	for _, stat := range RuntimeKernelExecutionStats() {
		if stat.Kernel == "QPipelinePlan" && stat.Outcome == "hit" && stat.Count > 0 {
			seenPlanHit = true
			break
		}
	}
	if !seenPlanHit {
		t.Fatalf("missing QPipelinePlan hit stat: %#v", RuntimeKernelExecutionStats())
	}
}

func TestEvalScriptPipelineAssignmentsOverrideSessionBindings(t *testing.T) {
	state := NewEvalState(nil)
	if got, err := state.Eval("x:(til 8)*0.5;y:0.5^x;idx:where x<4;y mod 2"); err != nil {
		t.Fatalf("seed Eval returned %#v, %v; want nil error", got, err)
	}
	if got, err := state.Eval("x:til 8;y:(x*2)+7;idx:where (y mod 5)>1;(+/y[idx])+count idx"); err != nil || got != int64(70) {
		t.Fatalf("pipeline Eval returned %#v, %v; want 70,nil", got, err)
	}
}

func TestEvalIPCLoopbackHandle(t *testing.T) {
	state := NewEvalState(map[string]any{"x": int64(100)})
	assertStateEvalValue(t, state, `h:hopen "loopback";h["1+2"]`, int64(3))
	assertStateEvalValue(t, state, `h["x:10"]`, int64(10))
	assertStateEvalValue(t, state, `h["x+5"]`, int64(15))
	assertStateEvalValue(t, state, `x`, int64(100))
	assertStateEvalValue(t, state, `neg[h]["x:20"]`, data.NullValue)
	assertStateEvalValue(t, state, `h["x"]`, int64(20))
}

func TestEvalIPCLoopbackMessageListCalls(t *testing.T) {
	assertEvalValue(t, `h:hopen "loopback";h[("x+y";2;3)]`, int64(5))
	assertEvalValue(t, `h:hopen "loopback";h[("{x*y}";4;5)]`, int64(20))
	assertEvalValue(t, `h:hopen "loopback";h[({x-y};9;2)]`, int64(7))
	assertEvalValue(t, "h:hopen \"loopback\";h[\"f:{x+y}\"];h[(`f;8;6)]", int64(14))
	assertEvalValue(t, `h:hopen "loopback";h[("x0+x1+x2";2;3;4)]`, int64(9))
	assertEvalValue(t, `h:hopen "loopback";h["x:100"];h[("x+y";2;3)];h["x"]`, int64(100))
	assertEvalValue(t, `h:hopen "loopback";neg[h][("a:x+y";20;22)];h["a"]`, int64(42))
}

func TestEvalIPCLoopbackTargetFormsAndBoundaries(t *testing.T) {
	assertEvalValue(t, "h:hopen `:loopback;h[\"+/1 2 3\"]", int64(6))
	assertEvalValue(t, `h:hopen "local";h["a:7"];h["a*3"]`, int64(21))
	assertEvalErrorContains(t, "hopen `:localhost:5000", "unsupported q IPC target")
	assertEvalErrorContains(t, `hopen 42`, "hopen expects a loopback target")
	assertEvalErrorContains(t, `h:hopen "loopback";h[10]`, "expects a string command or message list")
	assertEvalErrorContains(t, `h:hopen "loopback";h[()]`, "message list is empty")
	assertEvalErrorContains(t, `h:hopen "loopback";h[(10;20)]`, "message list head")
	assertEvalErrorContains(t, "h:hopen \"loopback\";h[(`missing;20)]", "is not defined")
	assertEvalErrorContains(t, `h:hopen "loopback";h["\\p"]`, "system commands are not supported")
	assertEvalErrorContains(t, `h:hopen "loopback";h[("\\p";1)]`, "system commands are not supported")
	assertEvalErrorContains(t, `h:hopen "loopback";h[""]`, "command is empty")
}

func TestEvalSafeSystemCommands(t *testing.T) {
	state := NewEvalState(nil)
	assertStateEvalValue(t, state, `\p`, int64(0))
	assertStateEvalValue(t, state, `\p 5000`, int64(5000))
	assertStateEvalValue(t, state, `\p`, int64(5000))
	assertStateEvalValue(t, state, `\p 0`, int64(0))

	other := NewEvalState(nil)
	assertStateEvalValue(t, other, `\p`, int64(0))

	_, err := state.Eval("x:1;y:{x+1};t:([] sym:`AAPL`MSFT;price:10 20);kt:([sym:`AAPL`MSFT]price:10 20)")
	if err != nil {
		t.Fatalf("state setup returned error: %v", err)
	}
	assertStateEvalArray(t, state, `\v`, data.KindSymbol, []any{data.Symbol("x")})
	assertStateEvalArray(t, state, `\f`, data.KindSymbol, []any{data.Symbol("y")})
	assertStateEvalArray(t, state, `\a`, data.KindSymbol, []any{data.Symbol("kt"), data.Symbol("t")})
	assertStateEvalArray(t, state, `\b`, data.KindSymbol, []any{})
	assertStateEvalValue(t, state, `\d`, data.Symbol("."))
	got, err := state.Eval(`\w`)
	if err != nil {
		t.Fatalf("state.Eval(`\\w`) returned error: %v", err)
	}
	workspaceStats, ok := got.(data.Array)
	if !ok {
		t.Fatalf("state.Eval(`\\w`) = %#v, want data.Array", got)
	}
	if workspaceStats.Kind() != data.KindI64 {
		t.Fatalf("state.Eval(`\\w`) kind = %s, want %s", workspaceStats.Kind(), data.KindI64)
	}
	if values := workspaceStats.Values(); len(values) != 4 {
		t.Fatalf("state.Eval(`\\w`) values = %#v, want 4 workspace stats", values)
	}
	assertStateEvalValue(t, state, ".z.K", int64(4))
	assertStateEvalValue(t, state, ".z.k", "leia-q")
	assertStateEvalValue(t, state, ".z.o", "leia")
	assertStateEvalValue(t, state, ".z.q", data.Symbol("leia"))
	pid, err := state.Eval(".z.i")
	if err != nil {
		t.Fatalf("state.Eval(.z.i) returned error: %v", err)
	}
	if got, ok := pid.(int64); !ok || got <= 0 {
		t.Fatalf("state.Eval(.z.i) = %#v, want positive process id", pid)
	}

	assertEvalErrorContains(t, `\p -1`, "0..65535")
	assertEvalErrorContains(t, `\p abc`, "0..65535")
	assertEvalErrorContains(t, `\p 1 2`, "zero or one")
	assertEvalErrorContains(t, `\v x`, "expects no arguments")
	assertEvalErrorContains(t, `\f x`, "expects no arguments")
	assertEvalErrorContains(t, `\a x`, "expects no arguments")
	assertEvalErrorContains(t, `\b x`, "expects no arguments")
	assertStateEvalValue(t, state, `\d .foo`, data.Symbol(".foo"))
	assertEvalErrorContains(t, `\w 1`, "expects no arguments")
	assertEvalErrorContains(t, `\x`, "unsupported q system command")
}

func TestEvalStateNamespaceMinimalSupport(t *testing.T) {
	state := NewEvalState(nil)
	assertStateEvalValue(t, state, `\d`, data.Symbol("."))
	assertStateEvalValue(t, state, `x:1`, int64(1))
	assertStateEvalValue(t, state, `\d .ns`, data.Symbol(".ns"))
	assertStateEvalValue(t, state, `\d`, data.Symbol(".ns"))
	assertStateEvalValue(t, state, `x:2`, int64(2))
	assertStateEvalValue(t, state, `x`, int64(2))
	assertStateEvalValue(t, state, `.ns.x`, int64(2))
	assertStateEvalValue(t, state, `.other.x:3`, int64(3))
	assertStateEvalValue(t, state, `.other.x`, int64(3))
	assertStateEvalArray(t, state, `\v`, data.KindSymbol, []any{data.Symbol("x")})
	assertStateEvalValue(t, state, `\d .`, data.Symbol("."))
	assertStateEvalValue(t, state, `x`, int64(1))
	assertStateEvalValue(t, state, `.ns.x`, int64(2))

	// This is intentionally a flat, single-level namespace model for workspace
	// isolation; it is not a full kdb+ namespace tree.
	assertEvalErrorContains(t, `\d .a.b`, "single-level namespace")
	assertEvalErrorContains(t, `.a.b.c:1`, "invalid")
}

func TestEvalStateDoesNotRewriteTableLiteralIdentifiers(t *testing.T) {
	state := NewEvalState(nil)
	assertStateEvalValue(t, state, "sym:99", int64(99))
	got, err := state.Eval("trades:([] sym:`AAPL`MSFT`AAPL; qty:10 20 30); qty:trades`qty; +/qty")
	if err != nil {
		t.Fatalf("state.Eval(table script) returned error: %v", err)
	}
	if got != int64(60) {
		t.Fatalf("state.Eval(table script) = %#v, want 60", got)
	}
	assertStateEvalValue(t, state, "sym", int64(99))
}

func TestEvalSystemLoadScriptFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "loaded.q")
	if err := os.WriteFile(file, []byte("a:10;b:{x+a};b[5]"), 0o644); err != nil {
		t.Fatalf("write q load fixture: %v", err)
	}
	state := NewEvalState(nil)
	got, err := state.Eval(`\l ` + file)
	if err != nil {
		t.Fatalf("state.Eval(load) returned error: %v", err)
	}
	if got != int64(15) {
		t.Fatalf("state.Eval(load) = %#v, want 15", got)
	}
	got, err = state.Eval("b[7]")
	if err != nil {
		t.Fatalf("state.Eval(loaded fn) returned error: %v", err)
	}
	if got != int64(17) {
		t.Fatalf("state.Eval(loaded fn) = %#v, want 17", got)
	}
	got, err = state.Eval(`\l ` + "`:" + file)
	if err != nil {
		t.Fatalf("state.Eval(q path load) returned error: %v", err)
	}
	if got != int64(15) {
		t.Fatalf("state.Eval(q path load) = %#v, want 15", got)
	}
	assertEvalErrorContains(t, `\l`, "expects one file path")
	assertEvalErrorContains(t, `\l :remote`, "local file path")
}

func TestEvalLambdaCallProjectionAndAdverbs(t *testing.T) {
	assertEvalValue(t, "{x+y}[1;2]", int64(3))
	assertEvalValue(t, "{[a;b]a*b}[3;4]", int64(12))
	assertEvalValue(t, "{{x+1}[10]}[0]", int64(11))
	assertEvalValue(t, "{{x+1}}[0][10]", int64(11))
	assertEvalValue(t, "a:10;f:{x+a};a:20;f[1]", int64(11))
	assertEvalValue(t, "mk:{[a]{x+a}};add10:mk[10];add20:mk[20];v1:add10[1];v2:add20[1];v1+v2", int64(32))
	assertEvalValue(t, "a:10;p:{x+y+a}[1;];a:20;p[2]", int64(13))
	assertEvalValue(t, "a:10;p:{x+y+z+a}[1;;];q:p[;3];a:20;q[2]", int64(16))
	assertEvalValue(t, "first[10 20 30]", int64(10))
	assertEvalValue(t, "(first reverse)[10 20 30]", int64(30))
	assertEvalValue(t, "(count distinct)[10 20 10 30]", int64(3))
	assertEvalValue(t, "count distinct 10 20 10 30", int64(3))
	assertEvalValue(t, "(count distinct)[10 20 10 30]+(first reverse)[1 2 3]", int64(6))
	assertEvalValue(t, "a:(count distinct)[10 20 10 30];b:(first reverse)[1 2 3];a+b", int64(6))
	assertEvalValue(t, "{x+y}[1;][2]", int64(3))
	assertEvalValue(t, "{x+y}[;2][1]", int64(3))
	assertEvalValue(t, "{x+y+z}[1;;3][2]", int64(6))
	assertEvalValue(t, "p:{x+y+z}[;;3];q:p[1];q[2]", int64(6))
	assertEvalValue(t, "p:{x+y+z}[1;;];q:p[2];q[3]", int64(6))
	assertEvalArray(t, "{x+1}'10 20 30", data.KindI64, []any{int64(11), int64(21), int64(31)})
	assertEvalArray(t, "{x+y}'[1 2 3;10 20 30]", data.KindI64, []any{int64(11), int64(22), int64(33)})
	assertEvalArray(t, "({x+y}[1;])'[10 20 30]", data.KindI64, []any{int64(11), int64(21), int64(31)})
	assertEvalArray(t, "({x+y}[1;])'10 20 30", data.KindI64, []any{int64(11), int64(21), int64(31)})
	assertEvalArray(t, "a:10;p:{x+y+a}[1;];a:20;p'10 20 30", data.KindI64, []any{int64(21), int64(31), int64(41)})
	assertEvalArray(t, "(first reverse)'(1 2;3 4 5)", data.KindI64, []any{int64(2), int64(5)})
	assertEvalArray(t, "(neg abs)'-1 2 -3", data.KindI64, []any{int64(-1), int64(-2), int64(-3)})
	assertEvalArray(t, "(first reverse) each (1 2;3 4 5)", data.KindI64, []any{int64(2), int64(5)})
	assertEvalValue(t, "((count distinct)')[10 20 10;1 2 3]", int64(3))
	assertEvalDict(t, "count'`a`b!(1 2;3 4 5)", []data.Symbol{"a", "b"}, []any{int64(2), int64(3)})
	assertEvalDict(t, "{x+10}'`a`b!1 2", []data.Symbol{"a", "b"}, []any{int64(11), int64(12)})
	assertEvalDict(t, "(count distinct)'`a`b!(1 1 2;3 3 3)", []data.Symbol{"a", "b"}, []any{int64(2), int64(1)})
	assertEvalDict(t, "(first reverse) each `a`b!(1 2;3 4 5)", []data.Symbol{"a", "b"}, []any{int64(2), int64(5)})
	assertEvalArray(t, "{x-y}\\:[10;1 2 3]", data.KindI64, []any{int64(9), int64(8), int64(7)})
	assertEvalArray(t, "{x-y}/:[10 20 30;1]", data.KindI64, []any{int64(9), int64(19), int64(29)})
	assertEvalArray(t, "{x-y}':10 15 14 20", data.KindI64, []any{int64(10), int64(5), int64(-1), int64(6)})
	assertEvalArray(t, "({x-y}':)[100;10 15 14]", data.KindI64, []any{int64(-90), int64(5), int64(-1)})
	assertEvalValue(t, "{x+y}/1 2 3 4", int64(10))
	assertEvalValue(t, "{x+y}/[10;1 2 3]", int64(16))
	assertEvalValue(t, "{[a;b]a+b}/[10;1 2 3]", int64(16))
	assertEvalValue(t, "a:10;({x+y+a}[1;])/[2 3 4]", int64(24))
	assertEvalArray(t, "{x+y}\\1 2 3 4", data.KindI64, []any{int64(1), int64(3), int64(6), int64(10)})
	assertEvalArray(t, "{x+y}\\[10;1 2 3]", data.KindI64, []any{int64(11), int64(13), int64(16)})
	assertEvalArray(t, "{[a;b]a+b}\\[10;1 2 3]", data.KindI64, []any{int64(11), int64(13), int64(16)})
	assertEvalArray(t, "a:10;({x+y+a}[1;])\\2 3 4", data.KindI64, []any{int64(2), int64(13), int64(24)})
	assertEvalValue(t, "(+/)[1 2 3 4]", int64(10))
	assertEvalArray(t, "(+\\)[1 2 3 4]", data.KindI64, []any{int64(1), int64(3), int64(6), int64(10)})
	assertEvalArray(t, "(-':)[10 15 14 20]", data.KindI64, []any{int64(10), int64(5), int64(-1), int64(6)})
	assertEvalArray(t, "(-':)[100;10 15 14]", data.KindI64, []any{int64(-90), int64(5), int64(-1)})
	assertEvalArray(t, "(-\\:)[10;1 2 3]", data.KindI64, []any{int64(9), int64(8), int64(7)})
	assertEvalArray(t, "(-/:)[10 20 30;1]", data.KindI64, []any{int64(9), int64(19), int64(29)})
	assertEvalArray(t, "f:+;f'[1 2 3;10 20 30]", data.KindI64, []any{int64(11), int64(22), int64(33)})
	assertEvalValue(t, "f:+;f/1 2 3 4", int64(10))
	assertEvalArray(t, "f:+;f\\1 2 3 4", data.KindI64, []any{int64(1), int64(3), int64(6), int64(10)})
	assertEvalArray(t, "f:-;f':10 15 14 20", data.KindI64, []any{int64(10), int64(5), int64(-1), int64(6)})
	assertEvalArray(t, "f:-;f\\:[10;1 2 3]", data.KindI64, []any{int64(9), int64(8), int64(7)})
	assertEvalArray(t, "f:-;f/:[10 20 30;1]", data.KindI64, []any{int64(9), int64(19), int64(29)})
	assertEvalValue(t, "f:{x+y};g:f/;g[10;1 2 3]", int64(16))
	assertEvalValue(t, "p:{x+y+z}[1;;];p/[10 20 30]", int64(62))
	assertEvalArray(t, "(+/)'(1 2 3;10 20 30)", data.KindI64, []any{int64(6), int64(60)})
	assertEvalArray(t, "(+\\)'(1 2 3;10 20 30)", data.KindAny, []any{
		data.NewI64([]int64{1, 3, 6}),
		data.NewI64([]int64{10, 30, 60}),
	})
	assertEvalArray(t, "p:{x+y+z}[1;;];p':10 20 30", data.KindI64, []any{int64(10), int64(31), int64(51)})
	assertEvalArray(t, "p:{x+y+z}[1;;];q:p':;q[100;10 20 30]", data.KindI64, []any{int64(111), int64(31), int64(51)})
	assertEvalArray(t, "f:+;g:f';h:g[;10 20 30];h[1 2 3]", data.KindI64, []any{int64(11), int64(22), int64(33)})
	assertEvalArray(t, "f:+;g:f\\;h:g[10;];h[1 2 3]", data.KindI64, []any{int64(11), int64(13), int64(16)})
	assertEvalErrorContains(t, "{[a;b]a+b}[1;2;3]", "lambda expected at most 2 arguments")
	assertEvalErrorContains(t, "{x+y}/[1;2;3]", "callable over expected 1 or 2 arguments")
	assertEvalErrorContains(t, "{x+y}'[1 2;10 20 30]", "callable each length mismatch")
	assertEvalErrorContains(t, "{x-y}\\:1 2 3", "callable each-left expected 2 arguments")
	assertEvalErrorContains(t, "{x+y}':[1;2;3]", "callable each-prior expected 1 or 2 arguments")
	assertEvalErrorContains(t, "(first reverse)\\:1 2 3", "callable each-left expected 2 arguments")
	assertEvalErrorContains(t, "(+\\:)[10]", "expected 2 arguments")
	assertEvalErrorContains(t, "(first reverse)[1;2]", "composition expected 1 argument")
	assertEvalErrorContains(t, "(first reverse)/[10;1 2 3]", "composition cannot be used with over")
	assertEvalErrorContains(t, "(first reverse)\\[10;1 2 3]", "composition cannot be used with scan")
}

func TestEvalASTValueExpressions(t *testing.T) {
	assertEvalParsedValue(t, "sum abs -1", int64(1))
	assertEvalParsedValue(t, "(10 20 30)[1]+(`a`b!100 200)[`b]", int64(220))
	assertEvalParsedArray(t, "2 xprev 10 20 30 40", data.KindI64, []any{data.NullValue, data.NullValue, int64(10), int64(20)})
	assertEvalParsedValue(t, "((1;2 3);(`a`b!10 20))[1][`b]", int64(20))
	assertEvalParsedValue(t, "sum[1 2 3]", int64(6))

	state := NewEvalState(map[string]any{"x": int64(10)})
	got, ok, err := state.evalParsedValueExpr("x+5")
	if err != nil {
		t.Fatalf("evalParsedValueExpr(env binary) returned error: %v", err)
	}
	if !ok || got != int64(15) {
		t.Fatalf("evalParsedValueExpr(env binary) = %#v, %v, want 15, true", got, ok)
	}
}

func TestEvalListAndStringIndexing(t *testing.T) {
	assertEvalValue(t, "(10 20 30)[1]", int64(20))
	assertEvalArray(t, "(10 20 30)[2 0]", data.KindI64, []any{int64(30), int64(10)})
	assertEvalValue(t, `("alpha" "beta" "gamma")[2]`, "gamma")
	assertEvalValue(t, `"abcd"[2]`, "c")
	assertEvalArray(t, `"abcd"[3 1]`, data.KindString, []any{"d", "b"})
	assertEvalValue(t, "(`a`b!10 20)[`b]", int64(20))
}

func TestEvalApplyIndexOperators(t *testing.T) {
	assertEvalValue(t, "x:10 20 30;x@1", int64(20))
	assertEvalArray(t, "x:10 20 30;x@2 0", data.KindI64, []any{int64(30), int64(10)})
	assertEvalValue(t, `s:"abcd";s@2`, "c")
	assertEvalValue(t, "d:`a`b!10 20;d@`b", int64(20))
	assertEvalValue(t, "sum@1 2 3", int64(6))
	assertEvalValue(t, "{x+y} . (4;5)", int64(9))
	assertEvalValue(t, "x:10 20 30;x . 1", int64(20))
	assertEvalValue(t, "x:(10 20;30 40);x . (0;1)", int64(20))
	assertEvalValue(t, ".[{x+y};(2;3)]", int64(5))
	assertEvalValue(t, ".[sum;enlist 1 2 3]", int64(6))
	assertEvalDict(t, ".[(`a`b!10 20);`b;:;25]", []data.Symbol{"a", "b"}, []any{int64(10), int64(25)})
}

func TestEvalDictionaryAmendAndUpsert(t *testing.T) {
	assertEvalDict(t, "@[(`a`b!10 20);`b;:;25]", []data.Symbol{"a", "b"}, []any{int64(10), int64(25)})
	assertEvalDict(t, "@[(`a`b!10 20);`c;:;30]", []data.Symbol{"a", "b", "c"}, []any{int64(10), int64(20), int64(30)})
	assertEvalDict(t, "@[(`a`b!10 20);`b`c;:;22 33]", []data.Symbol{"a", "b", "c"}, []any{int64(10), int64(22), int64(33)})
	assertEvalDict(t, "@[(`a`b!10 20);`b;+;5]", []data.Symbol{"a", "b"}, []any{int64(10), int64(25)})
	assertEvalDict(t, ".[(`a`b!10 20);`b;:;25]", []data.Symbol{"a", "b"}, []any{int64(10), int64(25)})
	assertEvalDict(t, ".[(`a`b!(1 2 3;10 20 30));(`a;1);:;99]", []data.Symbol{"a", "b"}, []any{data.NewI64([]int64{1, 99, 3}), data.NewI64([]int64{10, 20, 30})})
	assertEvalDictAny(t, "@[(0N 1!10 20);0N;:;99]", []any{data.NullValue, int64(1)}, []any{int64(99), int64(20)})
	assertEvalDictAny(t, "@[(1 2!10 20);3;:;30]", []any{int64(1), int64(2), int64(3)}, []any{int64(10), int64(20), int64(30)})
	assertEvalArray(t, "@[(10 20 30);1;:;99]", data.KindI64, []any{int64(10), int64(99), int64(30)})
	assertEvalArray(t, "@[(10 20 30);1;+;5]", data.KindI64, []any{int64(10), int64(25), int64(30)})
	assertEvalArray(t, "@[(10 20 30);0 2;:;100 300]", data.KindI64, []any{int64(100), int64(20), int64(300)})
	assertEvalArray(t, "@[(10 20 30);0 2;+;1 2]", data.KindI64, []any{int64(11), int64(20), int64(32)})
	assertEvalArray(t, "@[(1 2 0Ni);2;:;3]", data.KindI32, []any{int32(1), int32(2), int32(3)})
	assertEvalArray(t, "@[(1 2 0Ni);2;:;0Ni]", data.KindI32, []any{int32(1), int32(2), data.NullValue})
	assertEvalArray(t, "@[(1h 2h 0Nh);2;:;3]", data.KindI16, []any{int16(1), int16(2), int16(3)})
	assertEvalArray(t, "@[(1e 0Ne);1;:;2]", data.KindF32, []any{float32(1), float32(2)})
	assertEvalArray(t, "@[(1i 2i 3i);0 2;:;10 30]", data.KindI32, []any{int32(10), int32(2), int32(30)})
	assertEvalArray(t, "@[(1 2 0Ni);2;:;i$3]", data.KindI32, []any{int32(1), int32(2), int32(3)})
	assertEvalArray(t, "@[(1e 0Ne);1;:;e$2]", data.KindF32, []any{float32(1), float32(2)})
	assertEvalErrorContains(t, "@[(10 20);3;:;99]", "out of range")
	// Canonical q broadcasts an atom amend value over every index.
	assertEvalArray(t, "@[(10 20);0 1;:;99]", data.KindI64, []any{int64(99), int64(99)})
	assertEvalErrorContains(t, "@[(10 20 30);0 1;:;1 2 3]", "value length mismatch")
}

func TestEvalTableAndKeyedFunctionalAmend(t *testing.T) {
	got, err := Eval("@[(flip `sym`price`size!(`AAPL`MSFT;100 80;10 20));1;:;`price`size!81 21]")
	if err != nil {
		t.Fatalf("Eval table amend returned error: %v", err)
	}
	frame, ok := got.(data.Frame)
	if !ok {
		t.Fatalf("table amend = %#v, want data.Frame", got)
	}
	assertFrameValue(t, frame, "sym", 0, data.Symbol("AAPL"))
	assertFrameValue(t, frame, "price", 1, int64(81))
	assertFrameValue(t, frame, "size", 1, int64(21))

	got, err = Eval("@[(flip `sym`price`size!(`AAPL`MSFT`TSLA;100 80 120;10 20 30));0 2;:;flip `price`size!(101 121;11 31)]")
	if err != nil {
		t.Fatalf("Eval multi-row table amend returned error: %v", err)
	}
	frame, ok = got.(data.Frame)
	if !ok {
		t.Fatalf("multi-row table amend = %#v, want data.Frame", got)
	}
	assertFrameValue(t, frame, "price", 0, int64(101))
	assertFrameValue(t, frame, "price", 1, int64(80))
	assertFrameValue(t, frame, "price", 2, int64(121))
	assertFrameValue(t, frame, "size", 2, int64(31))

	got, err = Eval("@[(flip `sym`price`size!(`AAPL`MSFT`TSLA;100 80 120;10 20 30));where true false true;+;`price`size!(1 2;10 20)]")
	if err != nil {
		t.Fatalf("Eval table functional amend returned error: %v", err)
	}
	frame, ok = got.(data.Frame)
	if !ok {
		t.Fatalf("table functional amend = %#v, want data.Frame", got)
	}
	assertFrameValue(t, frame, "sym", 0, data.Symbol("AAPL"))
	assertFrameValue(t, frame, "price", 0, int64(101))
	assertFrameValue(t, frame, "price", 1, int64(80))
	assertFrameValue(t, frame, "price", 2, int64(122))
	assertFrameValue(t, frame, "size", 0, int64(20))
	assertFrameValue(t, frame, "size", 2, int64(50))

	got, err = Eval(".[(flip `sym`price`size!(`AAPL`MSFT;100 80;10 20));(`price;1);:;81]")
	if err != nil {
		t.Fatalf("Eval table dot amend returned error: %v", err)
	}
	frame, ok = got.(data.Frame)
	if !ok {
		t.Fatalf("table dot amend = %#v, want data.Frame", got)
	}
	assertFrameValue(t, frame, "sym", 1, data.Symbol("MSFT"))
	assertFrameValue(t, frame, "price", 1, int64(81))

	got, err = Eval("@[(flip `sym`ts`qty!(`AAPL`MSFT;0Np 1970.01.02D00:00:00.000000001;1 2));0;:;`ts`qty!(1970.01.03D00:00:00.000000001;0Ni)]")
	if err != nil {
		t.Fatalf("Eval typed table amend returned error: %v", err)
	}
	frame, ok = got.(data.Frame)
	if !ok {
		t.Fatalf("typed table amend = %#v, want data.Frame", got)
	}
	if kind, _ := frame.Schema().Kind("ts"); kind != data.KindTimestamp {
		t.Fatalf("ts kind = %q, want timestamp", kind)
	}
	if kind, _ := frame.Schema().Kind("qty"); kind != data.KindI64 {
		t.Fatalf("qty kind = %q, want i64", kind)
	}
	assertFrameValue(t, frame, "qty", 0, data.NullValue)

	got, err = Eval("@[(`sym xkey flip `sym`price`size!(`AAPL`MSFT;100 80;10 20));`AAPL;:;`price`size!101 11]")
	if err != nil {
		t.Fatalf("Eval keyed table amend returned error: %v", err)
	}
	keyed, ok := got.(data.KeyedFrame)
	if !ok {
		t.Fatalf("keyed table amend = %#v, want data.KeyedFrame", got)
	}
	hit, err := keyed.LookupValueByKey(data.Symbol("AAPL"))
	if err != nil {
		t.Fatalf("keyed lookup returned error: %v", err)
	}
	assertFrameValue(t, hit, "price", 0, int64(101))
	assertFrameValue(t, hit, "size", 0, int64(11))
	if !reflect.DeepEqual(keyed.Keys(), []data.Symbol{"sym"}) {
		t.Fatalf("keyed keys after amend = %#v, want [sym]", keyed.Keys())
	}
	assertFrameValue(t, keyed.Frame(), "sym", 0, data.Symbol("AAPL"))

	got, err = Eval("@[(`sym xkey flip `sym`price`size!(`AAPL`MSFT;100 80;10 20));`AAPL;+;`price`size!1 2]")
	if err != nil {
		t.Fatalf("Eval keyed functional amend returned error: %v", err)
	}
	keyed, ok = got.(data.KeyedFrame)
	if !ok {
		t.Fatalf("keyed functional amend = %#v, want data.KeyedFrame", got)
	}
	hit, err = keyed.LookupValueByKey(data.Symbol("AAPL"))
	if err != nil {
		t.Fatalf("keyed functional lookup returned error: %v", err)
	}
	assertFrameValue(t, hit, "price", 0, int64(101))
	assertFrameValue(t, hit, "size", 0, int64(12))

	got, err = Eval("@[(`sym xkey flip `sym`price`size!(`AAPL`MSFT;100 80;10 20));`TSLA;+;`price`size!120 30]")
	if err != nil {
		t.Fatalf("Eval keyed missing-key functional amend returned error: %v", err)
	}
	keyed, ok = got.(data.KeyedFrame)
	if !ok {
		t.Fatalf("keyed missing-key functional amend = %#v, want data.KeyedFrame", got)
	}
	hit, err = keyed.LookupValueByKey(data.Symbol("TSLA"))
	if err != nil {
		t.Fatalf("keyed missing-key functional lookup returned error: %v", err)
	}
	assertFrameValue(t, hit, "price", 0, int64(120))
	assertFrameValue(t, hit, "size", 0, int64(30))

	got, err = Eval("@[(`sym xkey flip `sym`qty!(`AAPL`MSFT;1 2));`TSLA;{[x;y]x};`qty!5]")
	if err != nil {
		t.Fatalf("Eval keyed missing-key typed-null functional amend returned error: %v", err)
	}
	keyed, ok = got.(data.KeyedFrame)
	if !ok {
		t.Fatalf("keyed missing-key typed-null amend = %#v, want data.KeyedFrame", got)
	}
	hit, err = keyed.LookupValueByKey(data.Symbol("TSLA"))
	if err != nil {
		t.Fatalf("keyed missing-key typed-null lookup returned error: %v", err)
	}
	assertFrameValue(t, hit, "qty", 0, data.NullValue)
	if kind, _ := keyed.Frame().Schema().Kind("qty"); kind != data.KindI64 {
		t.Fatalf("keyed missing-key qty kind = %q, want i64", kind)
	}

	got, err = Eval("@[(`sym xkey flip `sym`price`size!(`AAPL`MSFT;100 80;10 20));`TSLA;:;`price`size!120 30]")
	if err != nil {
		t.Fatalf("Eval keyed table upsert returned error: %v", err)
	}
	keyed, ok = got.(data.KeyedFrame)
	if !ok {
		t.Fatalf("keyed table upsert = %#v, want data.KeyedFrame", got)
	}
	hit, err = keyed.LookupValueByKey(data.Symbol("TSLA"))
	if err != nil {
		t.Fatalf("keyed upsert lookup returned error: %v", err)
	}
	assertFrameValue(t, hit, "price", 0, int64(120))
	assertFrameValue(t, hit, "size", 0, int64(30))
	if keyed.Frame().Len() != 3 {
		t.Fatalf("keyed upsert rows = %d, want 3", keyed.Frame().Len())
	}

	assertEvalErrorContains(t, "@[(flip `sym`price!(`AAPL`MSFT;100 80));2;:;`price!101]", "out of range")
	assertEvalErrorContains(t, "@[(flip `sym`price!(`AAPL`MSFT;100 80));0;:;`missing!1]", "does not exist")
	assertEvalErrorContains(t, "@[(`sym xkey flip `sym`price!(`AAPL`MSFT;100 80));`TSLA;+;`missing!1]", "does not exist")
	assertEvalErrorContains(t, "@[(`sym`venue xkey flip `sym`venue`price!(`AAPL`MSFT;`XNYS`XNAS;100 80));`AAPL;:;`price!101]", "expects 2 key values")
	assertEvalErrorContains(t, "@[(`sym xkey flip `sym`price!(`AAPL`TSLA;100 120));`AAPL;:;`sym`price!(`MSFT;101)]", "conflicts")
}

func TestEvalGroup(t *testing.T) {
	assertEvalGroupedIndexes(t, "group 10 20 10 0N", []any{int64(10), int64(20), data.NullValue}, [][]any{
		{int64(0), int64(2)},
		{int64(1)},
		{int64(3)},
	})
	assertEvalGroupedIndexes(t, "group `AAPL`MSFT`AAPL", []any{data.Symbol("AAPL"), data.Symbol("MSFT")}, [][]any{
		{int64(0), int64(2)},
		{int64(1)},
	})
}

func TestEvalTemporalLiteralsAndTypedNullVectors(t *testing.T) {
	assertEvalValue(t, "2024.01", data.MonthFromMonths(648))
	assertEvalValue(t, "2024.01m", data.MonthFromMonths(648))
	assertEvalValue(t, "2024.01.02", data.DateFromDays(19724))
	assertEvalValue(t, "2024-01-02", data.DateFromDays(19724))
	assertEvalValue(t, "1970.01.02T00:00:00.000000001", data.DateTimeFromUnixNanos(86_400_000_000_001))
	assertEvalValue(t, "1D00:00:00.000000001", data.TimespanFromNanos(86_400_000_000_001))
	assertEvalValue(t, "-0D00:01:00", data.TimespanFromNanos(-60*1_000_000_000))
	assertEvalValue(t, "string 2024.01", "2024.01")
	assertEvalValue(t, "string 2024.01m", "2024.01")
	assertEvalValue(t, "string 1970.01.02T00:00:00.000000001", "1970-01-02T00:00:00.000000001")
	assertEvalValue(t, "string 1D00:00:00.000000001", "1D00:00:00.000000001")
	assertEvalValue(t, "string -0D00:01:00", "-0D00:01:00")
	assertEvalValue(t, "string 09:30", "09:30")
	assertEvalValue(t, "string 09:30:00", "09:30:00")
	assertEvalValue(t, "09:30", data.MinuteFromMinutes(570))
	assertEvalValue(t, "09:30:00", data.SecondFromSeconds(34_200))
	assertEvalValue(t, "09:30:00.001", data.TimeFromNanos(34_200_001_000_000))
	assertEvalValue(t, "09:30:00.123456789", data.TimeFromNanos(34_200_123_456_789))
	assertEvalValue(t, "1970.01.02D00:00:00.000000001", data.TimestampFromUnixNanos(86_400_000_000_001))
	assertEvalValue(t, "1970-01-02T00:00:00.000000001Z", data.TimestampFromUnixNanos(86_400_000_000_001))
	assertEvalValue(t, "1970-01-02T00:00:00.000000001", data.TimestampFromUnixNanos(86_400_000_000_001))
	assertEvalValue(t, "type 0Ni", int64(-6))
	assertEvalValue(t, "type 0Nb", int64(-1))
	assertEvalValue(t, "type 0Nx", int64(-4))
	assertEvalValue(t, "type 0Nc", int64(-10))
	assertEvalValue(t, "type 0Nf", int64(-9))
	assertEvalValue(t, "type 0Nm", int64(-13))
	assertEvalValue(t, "type 0Nd", int64(-14))
	assertEvalValue(t, "type 0Nz", int64(-15))
	assertEvalValue(t, "type 0Nn", int64(-16))
	assertEvalValue(t, "type 0Nu", int64(-17))
	assertEvalValue(t, "type 0Nv", int64(-18))
	assertEvalValue(t, "type 0Nt", int64(-19))
	assertEvalValue(t, "type 0Np", int64(-12))

	assertEvalArray(t, "0Nd 0Nd", data.KindDate, []any{data.NullValue, data.NullValue})
	assertEvalArray(t, "0Nb true false", data.KindBool, []any{data.NullValue, true, false})
	assertEvalArray(t, "0Nx 1 2", data.KindU8, []any{data.NullValue, uint8(1), uint8(2)})
	assertEvalArray(t, "0Nc \"a\" \"b\"", data.KindString, []any{data.NullValue, "a", "b"})
	assertEvalArray(t, "0Np 0Np", data.KindTimestamp, []any{data.NullValue, data.NullValue})
	assertEvalArray(t, "0Nm 2024.01 2024.02", data.KindMonth, []any{data.NullValue, data.Month(648), data.Month(649)})
	assertEvalArray(t, "0Nm 2024.01m 2024.02m", data.KindMonth, []any{data.NullValue, data.Month(648), data.Month(649)})
	assertEvalArray(t, "2024-01-02 0Nd 2024.01.03", data.KindDate, []any{data.DateFromDays(19724), data.NullValue, data.DateFromDays(19725)})
	assertEvalArray(t, "0Nd 2024.01.02", data.KindDate, []any{data.NullValue, data.Date(19724)})
	assertEvalArray(t, "0Nz 1970.01.02T00:00:00.000000001", data.KindDateTime, []any{data.NullValue, data.DateTime(86_400_000_000_001)})
	assertEvalArray(t, "0Nn 1D00:00:00.000000001", data.KindTimespan, []any{data.NullValue, data.Timespan(86_400_000_000_001)})
	assertEvalArray(t, "0Nu 09:30", data.KindMinute, []any{data.NullValue, data.Minute(570)})
	assertEvalArray(t, "0Nv 09:30:00", data.KindSecond, []any{data.NullValue, data.Second(34_200)})
	assertEvalArray(t, "0Nt 09:30:00.001", data.KindTime, []any{data.NullValue, data.Time(34_200_001_000_000)})
	assertEvalArray(t, "0Np 1970.01.02D00:00:00.000000001 1970-01-02T00:00:00.000000001Z", data.KindTimestamp, []any{data.NullValue, data.Timestamp(86_400_000_000_001), data.Timestamp(86_400_000_000_001)})
	assertEvalArray(t, "0Np 1970-01-02T00:00:00.000000001", data.KindTimestamp, []any{data.NullValue, data.Timestamp(86_400_000_000_001)})
	assertEvalArray(t, "09:30 09:31:02", data.KindSecond, []any{data.Second(34_200), data.Second(34_262)})
	assertEvalArray(t, "09:30 09:31:02.250", data.KindTime, []any{data.Time(34_200_000_000_000), data.Time(34_262_250_000_000)})
	assertEvalArray(t, "09:30 0Nv 09:31:02", data.KindSecond, []any{data.Second(34_200), data.NullValue, data.Second(34_262)})
	assertEvalArray(t, "09:30 0Nv 09:31:02.250", data.KindTime, []any{data.Time(34_200_000_000_000), data.NullValue, data.Time(34_262_250_000_000)})
	assertEvalErrorContains(t, "2024.01.02 1", "mixed temporal and non-temporal vectors")
	assertEvalErrorContains(t, "2024.01.02 0Np", "mixed temporal vector kinds")
	assertEvalErrorContains(t, "1970.01.02T00:00:00 1970.01.02D00:00:00", "mixed temporal vector kinds")
	if _, err := ParseTemporal("date", "0Np"); err == nil {
		t.Fatalf("ParseTemporal(date, 0Np) succeeded, want typed-null kind mismatch error")
	}
	if _, err := ParseTemporal("boolean", "0Nb"); err == nil {
		t.Fatalf("ParseTemporal(boolean, 0Nb) succeeded, want unsupported typed-null error")
	}
	if _, err := ParseTemporal("time", "09:30:00.1234567899"); err == nil {
		t.Fatalf("ParseTemporal(time, over-precise fractional time) succeeded, want error")
	}
}

func TestEvalZNamespaceCurrentTemporalValues(t *testing.T) {
	before := time.Now().UTC()
	got, err := Eval(".z.P")
	after := time.Now().UTC()
	if err != nil {
		t.Fatalf("Eval(.z.P) returned error: %v", err)
	}
	ts, ok := got.(data.Timestamp)
	if !ok {
		t.Fatalf("Eval(.z.P) = %#v, want data.Timestamp", got)
	}
	if nanos := ts.UnixNanos(); nanos < before.UnixNano() || nanos > after.UnixNano() {
		t.Fatalf(".z.P = %d, want between %d and %d", nanos, before.UnixNano(), after.UnixNano())
	}

	before = time.Now().UTC()
	got, err = Eval(".z.D")
	after = time.Now().UTC()
	if err != nil {
		t.Fatalf("Eval(.z.D) returned error: %v", err)
	}
	date, ok := got.(data.Date)
	if !ok {
		t.Fatalf("Eval(.z.D) = %#v, want data.Date", got)
	}
	minDate := data.DateFromDays(before.Unix() / 86400)
	maxDate := data.DateFromDays(after.Unix() / 86400)
	if date < minDate || date > maxDate {
		t.Fatalf(".z.D = %#v, want between %#v and %#v", date, minDate, maxDate)
	}

	got, err = Eval(".z.T")
	if err != nil {
		t.Fatalf("Eval(.z.T) returned error: %v", err)
	}
	tod, ok := got.(data.Time)
	if !ok {
		t.Fatalf("Eval(.z.T) = %#v, want data.Time", got)
	}
	if !tod.Valid() {
		t.Fatalf(".z.T = %#v, want valid time of day", tod)
	}

	for _, src := range []string{".z.p"} {
		got, err := Eval(src)
		if err != nil {
			t.Fatalf("Eval(%s) returned error: %v", src, err)
		}
		if _, ok := got.(data.Timestamp); !ok {
			t.Fatalf("Eval(%s) = %#v, want data.Timestamp", src, got)
		}
	}
	for _, src := range []string{".z.d"} {
		got, err := Eval(src)
		if err != nil {
			t.Fatalf("Eval(%s) returned error: %v", src, err)
		}
		if _, ok := got.(data.Date); !ok {
			t.Fatalf("Eval(%s) = %#v, want data.Date", src, got)
		}
	}
	for _, src := range []string{".z.t"} {
		got, err := Eval(src)
		if err != nil {
			t.Fatalf("Eval(%s) returned error: %v", src, err)
		}
		tod, ok := got.(data.Time)
		if !ok {
			t.Fatalf("Eval(%s) = %#v, want data.Time", src, got)
		}
		if !tod.Valid() {
			t.Fatalf("%s = %#v, want valid time of day", src, tod)
		}
	}
	for _, src := range []string{".z.Z", ".z.z"} {
		got, err := Eval(src)
		if err != nil {
			t.Fatalf("Eval(%s) returned error: %v", src, err)
		}
		if _, ok := got.(data.DateTime); !ok {
			t.Fatalf("Eval(%s) = %#v, want data.DateTime", src, got)
		}
	}

	assertEvalErrorContains(t, ".z.X", "unsupported q .z namespace value")
}

func TestEvalFlipTableLiteral(t *testing.T) {
	got, err := Eval("flip `sym`price`size!(`AAPL`MSFT;100.5 101;10 20)")
	if err != nil {
		t.Fatalf("Eval returned error: %v", err)
	}
	frame, ok := got.(data.Frame)
	if !ok {
		t.Fatalf("frame = %#v", got)
	}
	if frame.Len() != 2 {
		t.Fatalf("len = %d, want 2", frame.Len())
	}
	if names := frame.Schema().Names(); !reflect.DeepEqual(names, []data.Symbol{"sym", "price", "size"}) {
		t.Fatalf("names = %#v", names)
	}
	assertFrameValue(t, frame, "sym", 0, data.Symbol("AAPL"))
	assertFrameValue(t, frame, "price", 1, 101.0)
	assertFrameValue(t, frame, "size", 1, int64(20))
	got, err = Eval("flip `sym`price!(`AAPL`MSFT;100)")
	if err != nil {
		t.Fatalf("Eval scalar-column flip returned error: %v", err)
	}
	frame, ok = got.(data.Frame)
	if !ok {
		t.Fatalf("scalar-column frame = %#v", got)
	}
	if frame.Len() != 2 {
		t.Fatalf("scalar-column len = %d, want 2", frame.Len())
	}
	assertFrameValue(t, frame, "price", 0, int64(100))
	assertFrameValue(t, frame, "price", 1, int64(100))
	assertEvalErrorContains(t, "flip (100 101!`AAPL`MSFT)", "symbol column names")
	assertEvalErrorContains(t, "flip ((flip `sym!enlist `AAPL)!flip `price!enlist 100)", "dictionary")
}

func TestEvalFlipDictLiteralBoundaries(t *testing.T) {
	got, err := Eval("flip `price`qty`sym!(100 101;1i 0Ni;`AAPL`MSFT)")
	if err != nil {
		t.Fatalf("Eval ordered typed-null flip returned error: %v", err)
	}
	frame, ok := got.(data.Frame)
	if !ok {
		t.Fatalf("ordered typed-null flip = %#v, want data.Frame", got)
	}
	if names := frame.Schema().Names(); !reflect.DeepEqual(names, []data.Symbol{"price", "qty", "sym"}) {
		t.Fatalf("ordered typed-null flip names = %#v, want [price qty sym]", names)
	}
	if kind, _ := frame.Schema().Kind("qty"); kind != data.KindI32 {
		t.Fatalf("ordered typed-null flip qty kind = %q, want i32", kind)
	}
	assertFrameValue(t, frame, "qty", 0, int32(1))
	assertFrameValue(t, frame, "qty", 1, data.NullValue)

	got, err = Eval("`price`qty`sym!(100 101;1i 0Ni;`AAPL`MSFT)")
	if err != nil {
		t.Fatalf("Eval plain dict returned error: %v", err)
	}
	dict, ok := got.(EvalDict)
	if !ok {
		t.Fatalf("plain dict = %#v, want EvalDict", got)
	}
	if !reflect.DeepEqual(dict.Keys, []any{data.Symbol("price"), data.Symbol("qty"), data.Symbol("sym")}) {
		t.Fatalf("plain dict keys = %#v, want source order", dict.Keys)
	}
	qty, ok := dict.Values[1].(data.Array)
	if !ok {
		t.Fatalf("plain dict qty = %#v, want data.Array", dict.Values[1])
	}
	if qty.Kind() != data.KindI32 {
		t.Fatalf("plain dict qty kind = %q, want i32", qty.Kind())
	}
}

func TestEvalQSQLStyleTableLiteral(t *testing.T) {
	got, err := Eval("([] sym:`AAPL`MSFT; price:100.5 101; size:10 20)")
	if err != nil {
		t.Fatalf("Eval(table literal) returned error: %v", err)
	}
	frame, ok := got.(data.Frame)
	if !ok {
		t.Fatalf("Eval(table literal) = %#v, want data.Frame", got)
	}
	if frame.Len() != 2 {
		t.Fatalf("len = %d, want 2", frame.Len())
	}
	if names := frame.Schema().Names(); !reflect.DeepEqual(names, []data.Symbol{"sym", "price", "size"}) {
		t.Fatalf("names = %#v", names)
	}
	assertFrameValue(t, frame, "sym", 0, data.Symbol("AAPL"))
	assertFrameValue(t, frame, "price", 1, 101.0)
	assertFrameValue(t, frame, "size", 1, int64(20))
	assertEvalArray(t, "cols ([] sym:`AAPL`MSFT; price:100.5 101; size:10 20)", data.KindSymbol, []any{
		data.Symbol("sym"),
		data.Symbol("price"),
		data.Symbol("size"),
	})
	assertEvalArray(t, "([] sym:`AAPL`MSFT; price:100.5 101; size:10 20)`price", data.KindF64, []any{100.5, 101.0})
	got, err = Eval("([] sym:`AAPL`MSFT; price:100.5 101; size:10 20)`sym`size")
	if err != nil {
		t.Fatalf("Eval(table multi-column projection) returned error: %v", err)
	}
	projected, ok := got.(data.Frame)
	if !ok {
		t.Fatalf("table multi-column projection = %#v, want data.Frame", got)
	}
	if names := projected.Schema().Names(); !reflect.DeepEqual(names, []data.Symbol{"sym", "size"}) {
		t.Fatalf("projected names = %#v, want [sym size]", names)
	}
	assertFrameValue(t, projected, "sym", 1, data.Symbol("MSFT"))
	assertFrameValue(t, projected, "size", 0, int64(10))
	assertEvalErrorContains(t, "([] sym:`AAPL`MSFT)`missing", "table column")
	assertEvalErrorContains(t, "([] sym:`AAPL`MSFT; price:100)", "must be a vector")
}

func TestEvalKeyedTableLiteral(t *testing.T) {
	got, err := Eval("(flip `sym!enlist `AAPL`MSFT)!flip `price`size!(100 101;10 20)")
	if err != nil {
		t.Fatalf("Eval(keyed table) returned error: %v", err)
	}
	keyed, ok := got.(data.KeyedFrame)
	if !ok {
		t.Fatalf("Eval(keyed table) = %#v, want data.KeyedFrame", got)
	}
	if keys := keyed.Keys(); !reflect.DeepEqual(keys, []data.Symbol{"sym"}) {
		t.Fatalf("keyed keys = %#v, want sym", keys)
	}
	assertEvalArray(t, "keys (flip `sym!enlist `AAPL`MSFT)!flip `price`size!(100 101;10 20)", data.KindSymbol, []any{data.Symbol("sym")})
	assertEvalArray(t, "cols (flip `sym!enlist `AAPL`MSFT)!flip `price`size!(100 101;10 20)", data.KindSymbol, []any{
		data.Symbol("sym"),
		data.Symbol("price"),
		data.Symbol("size"),
	})

	valueFrame, err := value(keyed)
	if err != nil {
		t.Fatalf("value(keyed) returned error: %v", err)
	}
	frame, ok := valueFrame.(data.Frame)
	if !ok {
		t.Fatalf("value(keyed) = %#v, want data.Frame", valueFrame)
	}
	if names := frame.Schema().Names(); !reflect.DeepEqual(names, []data.Symbol{"price", "size"}) {
		t.Fatalf("value keyed names = %#v", names)
	}
	assertFrameValue(t, frame, "price", 0, int64(100))
	assertFrameValue(t, frame, "size", 1, int64(20))

	got, err = Eval("((flip `sym!enlist `AAPL`MSFT)!flip `price`size!(100 101;10 20))[`MSFT]")
	if err != nil {
		t.Fatalf("Eval(keyed lookup) returned error: %v", err)
	}
	// Canonical: a single-key lookup yields the value-row dictionary.
	lookupDict, ok := got.(EvalDict)
	if !ok {
		t.Fatalf("keyed lookup = %#v, want EvalDict", got)
	}
	if !reflect.DeepEqual(lookupDict.Keys, []any{data.Symbol("price"), data.Symbol("size")}) {
		t.Fatalf("keyed lookup keys = %#v", lookupDict.Keys)
	}
	if !reflect.DeepEqual(lookupDict.Values, []any{int64(101), int64(20)}) {
		t.Fatalf("keyed lookup values = %#v", lookupDict.Values)
	}
	assertEvalArray(t, "((flip `sym!enlist `AAPL`MSFT)!flip `price`size!(100 101;10 20))`price", data.KindI64, []any{int64(100), int64(101)})
	assertEvalErrorContains(t, "([] sym:`AAPL`MSFT)!([] price:100 101 102)", "row length mismatch")

	got, err = Eval("1!flip `sym`price`size!(`AAPL`MSFT;100 101;10 20)")
	if err != nil {
		t.Fatalf("Eval(1!flip) returned error: %v", err)
	}
	keyed, ok = got.(data.KeyedFrame)
	if !ok {
		t.Fatalf("Eval(1!flip) = %#v, want data.KeyedFrame", got)
	}
	if keys := keyed.Keys(); !reflect.DeepEqual(keys, []data.Symbol{"sym"}) {
		t.Fatalf("1!flip keys = %#v, want sym", keys)
	}
	valueFrame, err = value(keyed)
	if err != nil {
		t.Fatalf("value(1!flip) returned error: %v", err)
	}
	frame, ok = valueFrame.(data.Frame)
	if !ok {
		t.Fatalf("value(1!flip) = %#v, want data.Frame", valueFrame)
	}
	if names := frame.Schema().Names(); !reflect.DeepEqual(names, []data.Symbol{"price", "size"}) {
		t.Fatalf("value 1!flip names = %#v", names)
	}
	assertFrameValue(t, frame, "price", 1, int64(101))

	got, err = Eval("2!flip `sym`venue`price!(`AAPL`AAPL`MSFT;`XNYS`XNAS`XNYS;100 101 80)")
	if err != nil {
		t.Fatalf("Eval(2!flip) returned error: %v", err)
	}
	keyed, ok = got.(data.KeyedFrame)
	if !ok {
		t.Fatalf("Eval(2!flip) = %#v, want data.KeyedFrame", got)
	}
	if keys := keyed.Keys(); !reflect.DeepEqual(keys, []data.Symbol{"sym", "venue"}) {
		t.Fatalf("2!flip keys = %#v, want sym venue", keys)
	}
	// Canonical: a composite single-key lookup yields the value-row dict.
	assertKeyedLookupDict(t, "(2!flip `sym`venue`price!(`AAPL`AAPL`MSFT;`XNYS`XNAS`XNYS;100 101 80))[`AAPL`XNAS]",
		[]any{data.Symbol("price")}, []any{int64(101)})

	got, err = Eval("0!flip `sym`price!(`AAPL`MSFT;100 101)")
	if err != nil {
		t.Fatalf("Eval(0!flip) returned error: %v", err)
	}
	frame, ok = got.(data.Frame)
	if !ok {
		t.Fatalf("Eval(0!flip) = %#v, want data.Frame", got)
	}
	if names := frame.Schema().Names(); !reflect.DeepEqual(names, []data.Symbol{"sym", "price"}) {
		t.Fatalf("0!flip names = %#v", names)
	}
	assertEvalErrorContains(t, "3!flip `sym`price!(`AAPL`MSFT;100 101)", "key count")
	assertEvalErrorContains(t, "(flip `sym!enlist `AAPL)!`price`size!(100;10)", "key and value tables")
}

func TestEvalQSQLStyleKeyedTableLiteral(t *testing.T) {
	got, err := Eval("([sym:`AAPL`MSFT] price:100 101; size:10 20)")
	if err != nil {
		t.Fatalf("Eval(keyed table literal) returned error: %v", err)
	}
	keyed, ok := got.(data.KeyedFrame)
	if !ok {
		t.Fatalf("Eval(keyed table literal) = %#v, want data.KeyedFrame", got)
	}
	if keys := keyed.Keys(); !reflect.DeepEqual(keys, []data.Symbol{"sym"}) {
		t.Fatalf("keyed keys = %#v, want sym", keys)
	}
	assertEvalArray(t, "keys ([sym:`AAPL`MSFT] price:100 101; size:10 20)", data.KindSymbol, []any{data.Symbol("sym")})
	assertEvalArray(t, "cols ([sym:`AAPL`MSFT] price:100 101; size:10 20)", data.KindSymbol, []any{
		data.Symbol("sym"),
		data.Symbol("price"),
		data.Symbol("size"),
	})

	valueFrame, err := value(keyed)
	if err != nil {
		t.Fatalf("value(keyed table literal) returned error: %v", err)
	}
	frame, ok := valueFrame.(data.Frame)
	if !ok {
		t.Fatalf("value(keyed table literal) = %#v, want data.Frame", valueFrame)
	}
	if names := frame.Schema().Names(); !reflect.DeepEqual(names, []data.Symbol{"price", "size"}) {
		t.Fatalf("value keyed names = %#v", names)
	}
	assertFrameValue(t, frame, "price", 0, int64(100))
	assertFrameValue(t, frame, "size", 1, int64(20))

	// Canonical: single-key lookups yield the value-row dictionary.
	assertKeyedLookupDict(t, "(([sym:`AAPL`MSFT] price:100 101; size:10 20))[`MSFT]",
		[]any{data.Symbol("price"), data.Symbol("size")}, []any{int64(101), int64(20)})
	assertKeyedLookupDict(t, "(([sym:`AAPL`MSFT; bucket:1 2] price:100 101))[(`MSFT;2)]",
		[]any{data.Symbol("price")}, []any{int64(101)})
	assertKeyedLookupDict(t, "(([sym:`AAPL`MSFT; bucket:1 2] price:100 101))[`bucket`sym!(2;`MSFT)]",
		[]any{data.Symbol("price")}, []any{int64(101)})
}

func assertKeyedLookupDict(t *testing.T, src string, keys []any, values []any) {
	t.Helper()
	got, err := Eval(src)
	if err != nil {
		t.Fatalf("Eval(%q) returned error: %v", src, err)
	}
	dict, ok := got.(EvalDict)
	if !ok {
		t.Fatalf("Eval(%q) = %#v, want EvalDict", src, got)
	}
	if !reflect.DeepEqual(dict.Keys, keys) {
		t.Fatalf("Eval(%q) keys = %#v, want %#v", src, dict.Keys, keys)
	}
	if !reflect.DeepEqual(dict.Values, values) {
		t.Fatalf("Eval(%q) values = %#v, want %#v", src, dict.Values, values)
	}
}

func TestEvalMetadataVerbs(t *testing.T) {
	assertEvalArray(t, "keys `sym`price!(`AAPL`MSFT;100 101)", data.KindSymbol, []any{
		data.Symbol("sym"),
		data.Symbol("price"),
	})
	assertEvalArray(t, "value `sym`price!(`AAPL`MSFT;100 101)", data.KindAny, []any{
		data.NewSymbols([]string{"AAPL", "MSFT"}),
		data.NewI64([]int64{100, 101}),
	})
	assertEvalArray(t, "cols flip `sym`price!(`AAPL`MSFT;100 101)", data.KindSymbol, []any{
		data.Symbol("sym"),
		data.Symbol("price"),
	})

	got, err := Eval("meta flip `sym`price!(`AAPL`MSFT;100 101)")
	if err != nil {
		t.Fatalf("Eval(meta) returned error: %v", err)
	}
	frame, ok := got.(data.Frame)
	if !ok {
		t.Fatalf("meta = %#v, want data.Frame", got)
	}
	if frame.Len() != 2 {
		t.Fatalf("meta rows = %d, want 2", frame.Len())
	}
	assertFrameValue(t, frame, "c", 0, data.Symbol("sym"))
	assertFrameValue(t, frame, "t", 0, "symbol")
	assertFrameValue(t, frame, "c", 1, data.Symbol("price"))
	assertFrameValue(t, frame, "t", 1, "i64")
}

func TestEvalAttributeMarkersAreTransparentVectors(t *testing.T) {
	for _, marker := range []string{"`s", "`g", "`p", "`u"} {
		src := marker + "#10 20 30"
		assertEvalArray(t, src, data.KindI64, []any{int64(10), int64(20), int64(30)})
		assertEvalValue(t, "count "+src, int64(3))
		assertEvalValue(t, "("+src+")[1]", int64(20))
		assertEvalArray(t, "2#"+src, data.KindI64, []any{int64(10), int64(20)})
		assertEvalArray(t, "value "+src, data.KindI64, []any{int64(10), int64(20), int64(30)})
		wantKeys := []any{data.Symbol("attribute"), data.Symbol("value")}
		if marker == "`g" || marker == "`u" {
			wantKeys = append(wantKeys, data.Symbol("index"))
		}
		assertEvalArray(t, "keys "+src, data.KindSymbol, wantKeys)

		got, err := Eval("meta " + src)
		if err != nil {
			t.Fatalf("Eval(meta %s) returned error: %v", src, err)
		}
		dict, ok := got.(EvalDict)
		if !ok {
			t.Fatalf("meta %s = %#v, want EvalDict", src, got)
		}
		if !reflect.DeepEqual(dict.Keys, []any{data.Symbol("attribute"), data.Symbol("type"), data.Symbol("count"), data.Symbol("index")}) {
			t.Fatalf("meta %s keys = %#v", src, dict.Keys)
		}
		indexValue := data.Symbol("")
		if marker == "`g" || marker == "`u" {
			indexValue = data.Symbol(marker[1:])
		}
		if !reflect.DeepEqual(dict.Values, []any{data.Symbol(marker[1:]), int64(7), int64(3), indexValue}) {
			t.Fatalf("meta %s values = %#v", src, dict.Values)
		}
	}
}

func TestEvalGroupedAndUniqueAttributesExposeReusableIndexes(t *testing.T) {
	assertEvalGroupedIndexes(t, "group `g#`AAPL`MSFT`AAPL", []any{data.Symbol("AAPL"), data.Symbol("MSFT")}, [][]any{
		{int64(0), int64(2)},
		{int64(1)},
	})
	assertEvalGroupedIndexes(t, "group `u#`AAPL`MSFT`TSLA", []any{data.Symbol("AAPL"), data.Symbol("MSFT"), data.Symbol("TSLA")}, [][]any{
		{int64(0)},
		{int64(1)},
		{int64(2)},
	})
	assertEvalArray(t, "keys `g#`AAPL`MSFT`AAPL", data.KindSymbol, []any{data.Symbol("attribute"), data.Symbol("value"), data.Symbol("index")})
	got, err := Eval("meta `g#`AAPL`MSFT`AAPL")
	if err != nil {
		t.Fatalf("Eval(meta grouped attribute) returned error: %v", err)
	}
	dict, ok := got.(EvalDict)
	if !ok {
		t.Fatalf("meta grouped attribute = %#v, want EvalDict", got)
	}
	if !reflect.DeepEqual(dict.Values, []any{data.Symbol("g"), int64(11), int64(3), data.Symbol("g")}) {
		t.Fatalf("meta grouped attribute values = %#v", dict.Values)
	}
}

func TestEvalSortedAttributeFeedsDataMetadata(t *testing.T) {
	got, err := Eval("`s#10 20 30")
	if err != nil {
		t.Fatalf("Eval(sorted attribute) returned error: %v", err)
	}
	array, ok := got.(data.Array)
	if !ok {
		t.Fatalf("sorted attribute = %#v, want data.Array", got)
	}
	if !data.ArrayHasAttribute(array, data.ArrayAttributeSorted) {
		t.Fatalf("sorted attribute metadata = %#v, want sorted", data.ArrayMetadataOf(array))
	}
	assertEvalValue(t, "(`s#10 20 30) bin 25", int64(1))
	assertEvalArray(t, "(`s#10 20 30) bin 5 10 29 30 31", data.KindI64, []any{int64(-1), int64(0), int64(1), int64(2), int64(2)})

	table, err := Eval("meta flip `ts`price!(`s#10 20 30;100 200 300)")
	if err != nil {
		t.Fatalf("Eval(meta table with sorted attribute) returned error: %v", err)
	}
	frame, ok := table.(data.Frame)
	if !ok {
		t.Fatalf("meta table = %#v, want data.Frame", table)
	}
	assertFrameValue(t, frame, "a", 0, data.Symbol("s"))
	assertFrameValue(t, frame, "a", 1, data.NullValue)
}

func TestEvalTypeStringAndCaseVerbs(t *testing.T) {
	assertEvalValue(t, "type 42", int64(-7))
	assertEvalValue(t, "type 1 2 3", int64(7))
	assertEvalValue(t, "type 1.5 2.5", int64(9))
	assertEvalValue(t, "type `AAPL", int64(-11))
	assertEvalValue(t, "type `AAPL`MSFT", int64(11))
	assertEvalValue(t, "type 2024.01", int64(-13))
	assertEvalValue(t, "type 2024.01.02", int64(-14))
	assertEvalValue(t, "type 1970.01.02T00:00:00", int64(-15))
	assertEvalValue(t, "type 1D00:00:00", int64(-16))
	assertEvalValue(t, "type 09:30", int64(-17))
	assertEvalValue(t, "type 09:30:00", int64(-18))
	assertEvalValue(t, "type 09:30:00.000", int64(-19))
	assertEvalValue(t, "type 1970.01.02D00:00:00", int64(-12))
	assertEvalValue(t, "type 0Nm 2024.01", int64(13))
	assertEvalValue(t, "type 0Nz 1970.01.02T00:00:00", int64(15))
	assertEvalValue(t, "type 0Nn 1D00:00:00", int64(16))
	assertEvalValue(t, "type 0Nu 09:30", int64(17))
	assertEvalValue(t, "type 0Nv 09:30:00", int64(18))
	assertEvalValue(t, "type flip `sym`price!(`AAPL`MSFT;100 101)", int64(98))
	assertEvalValue(t, "type `sym`price!(`AAPL`MSFT;100 101)", int64(99))

	assertEvalValue(t, "string `AAPL", "AAPL")
	assertEvalArray(t, "string `AAPL`MSFT", data.KindString, []any{"AAPL", "MSFT"})
	assertEvalArray(t, `string "Alpha" "Beta"`, data.KindString, []any{"Alpha", "Beta"})
	assertEvalValue(t, "string 2024.01.02", "2024-01-02")

	assertEvalValue(t, `lower "Straße"`, "straße")
	assertEvalValue(t, `upper "Straße"`, "STRAßE")
	assertEvalArray(t, "lower `AAPL`MSFT", data.KindSymbol, []any{data.Symbol("aapl"), data.Symbol("msft")})
	assertEvalArray(t, `upper "alpha" "Beta"`, data.KindString, []any{"ALPHA", "BETA"})
}

func TestEvalDyadicNumericOps(t *testing.T) {
	assertEvalValue(t, "2+3", int64(5))
	assertEvalValue(t, "2-3", int64(-1))
	assertEvalValue(t, "2*3", int64(6))
	assertEvalValue(t, "6%4", 1.5)
	assertEvalValue(t, "7 mod 4", int64(3))
	assertEvalValue(t, "7 div 4", int64(1))
	assertEvalValue(t, "-7 div 4", int64(-2))
	assertEvalValue(t, "7 div 0", data.NullValue)
	assertEvalValue(t, "2=2", true)
	assertEvalValue(t, "2<3", true)
	assertEvalValue(t, "3>2", true)
	assertEvalArray(t, "1 2 3+10", data.KindI64, []any{int64(11), int64(12), int64(13)})
	assertEvalArray(t, "10 20 30-1 2 3", data.KindI64, []any{int64(9), int64(18), int64(27)})
	assertEvalArray(t, "2*1.5 2.5", data.KindF64, []any{3.0, 5.0})
	assertEvalArray(t, "1 2 3%2", data.KindF64, []any{0.5, 1.0, 1.5})
	assertEvalArray(t, "10 11 12 mod 5", data.KindI64, []any{int64(0), int64(1), int64(2)})
	assertEvalArray(t, "5.5 -1.5 7.25 mod 2", data.KindF64, []any{1.5, 0.5, 1.25})
	assertEvalArray(t, "10 11 12 div 5", data.KindI64, []any{int64(2), int64(2), int64(2)})
	assertEvalArray(t, "100 101.5 99.5>100", data.KindBool, []any{false, true, false})
}

func TestEvalDyadicConformAndPromotion(t *testing.T) {
	assertEvalArray(t, "10+1 2 3", data.KindI64, []any{int64(11), int64(12), int64(13)})
	assertEvalArray(t, "1 2 3+10", data.KindI64, []any{int64(11), int64(12), int64(13)})
	assertEvalArray(t, "1 2 3+10 20 30", data.KindI64, []any{int64(11), int64(22), int64(33)})
	assertEvalArray(t, "1 2 3+enlist 10", data.KindI64, []any{int64(11), int64(12), int64(13)})
	assertEvalArray(t, "(enlist 10)+1 2 3", data.KindI64, []any{int64(11), int64(12), int64(13)})
	assertEvalArray(t, "1h 2h 0Nh+1", data.KindI64, []any{int64(2), int64(3), data.NullValue})
	assertEvalArray(t, "1i 0Ni 3i+1.5", data.KindF64, []any{2.5, data.NullValue, 4.5})
	assertEvalArray(t, "1.5e 0Ne 3.5e+1", data.KindF64, []any{2.5, data.NullValue, 4.5})
	assertEvalArray(t, "1 0N 3*2.5 10 0N", data.KindF64, []any{2.5, data.NullValue, data.NullValue})
	assertEvalArray(t, "0N 1=0N 1", data.KindBool, []any{true, true})
	assertEvalArray(t, "1 2 3<2 2 2", data.KindBool, []any{true, false, false})

	assertEvalErrorContains(t, "1 2+10 20 30", "vector length mismatch")

	if got, err := applyDyadic('<', data.NewI64([]int64{1, 2, 3}), data.NewI64([]int64{2, 2, 2})); err != nil {
		t.Fatalf("typed vector compare returned error: %v", err)
	} else {
		array, ok := got.(data.Array)
		if !ok || array.Kind() != data.KindBool || !reflect.DeepEqual(array.Values(), []any{true, false, false}) {
			t.Fatalf("typed vector compare = %#v", got)
		}
	}
	if got, err := applyDyadic('=', data.NewSymbols([]string{"AAPL", "MSFT"}), "AAPL"); err != nil {
		t.Fatalf("symbol/string vector compare returned error: %v", err)
	} else {
		array, ok := got.(data.Array)
		if !ok || array.Kind() != data.KindBool || !reflect.DeepEqual(array.Values(), []any{true, false}) {
			t.Fatalf("symbol/string vector compare = %#v", got)
		}
	}
	if got, err := applyDyadic('+', data.NewI64([]int64{10}), data.NewI64([]int64{1, 2, 3})); err != nil {
		t.Fatalf("singleton-vector left conform returned error: %v", err)
	} else {
		array, ok := got.(data.Array)
		if !ok || !reflect.DeepEqual(array.Values(), []any{int64(11), int64(12), int64(13)}) {
			t.Fatalf("singleton-vector left conform = %#v", got)
		}
	}
	if got, err := applyDyadic('+', data.NewI64([]int64{1, 2, 3}), data.NewI64([]int64{10})); err != nil {
		t.Fatalf("singleton-vector right conform returned error: %v", err)
	} else {
		array, ok := got.(data.Array)
		if !ok || !reflect.DeepEqual(array.Values(), []any{int64(11), int64(12), int64(13)}) {
			t.Fatalf("singleton-vector right conform = %#v", got)
		}
	}
}

func TestEvalFillDyadicVerb(t *testing.T) {
	assertEvalValue(t, "0^0N", int64(0))
	assertEvalValue(t, "0^42", int64(42))
	assertEvalValue(t, "0N^0N", data.NullValue)
	assertEvalArray(t, "0^1 0N 3", data.KindI64, []any{int64(1), int64(0), int64(3)})
	assertEvalArray(t, "100 200 300^1 0N 0N", data.KindI64, []any{int64(1), int64(200), int64(300)})
	assertEvalArray(t, "2026.06.06^0Nd 2026.06.07", data.KindDate, []any{
		data.DateFromDays(20610),
		data.DateFromDays(20611),
	})
	assertEvalArray(t, "0 fill 1 0N 3", data.KindI64, []any{int64(1), int64(0), int64(3)})

	assertEvalErrorContains(t, "1 2^10 20 30", "vector length mismatch")
}

func TestEvalFillAdverbs(t *testing.T) {
	assertEvalValue(t, "^/0N 1 0N 2", int64(2))
	assertEvalArray(t, "^\\0N 1 0N 2", data.KindI64, []any{data.NullValue, int64(1), int64(1), int64(2)})
	assertEvalArray(t, "100^':10 0N 30", data.KindI64, []any{int64(100), int64(10), int64(30)})
	assertEvalArray(t, "0^'1 0N 3", data.KindI64, []any{int64(1), int64(0), int64(3)})
	assertEvalArray(t, "0^\\:1 0N 3", data.KindI64, []any{int64(1), int64(0), int64(3)})
	assertEvalArray(t, "100 200 300^/:0N", data.KindI64, []any{int64(100), int64(200), int64(300)})
	assertEvalArray(t, "(^\\)[0N 1 0N 2]", data.KindI64, []any{data.NullValue, int64(1), int64(1), int64(2)})
}

func TestEvalMatchVerb(t *testing.T) {
	assertEvalValue(t, "1 2 3~1 2 3", true)
	assertEvalValue(t, "1 2 3~1 2", false)
	assertEvalValue(t, "1 2~1", false)
	assertEvalValue(t, "0N 1~0N 1", true)
	assertEvalValue(t, "`a`b!1 2~`a`b!1 2", true)
	assertEvalValue(t, "`a`b!1 2~`b`a!2 1", false)
	assertEvalValue(t, "(1 2;`a`b)~(1 2;`a`b)", true)
	assertEvalValue(t, "1 2 match 1 2", true)
	assertEvalValue(t, "`AAPL like `AA*", true)
	assertEvalValue(t, `"MSFT" like "AA*"`, false)
	assertEvalArray(t, "`AAPL`MSFT`AMD like `A*", data.KindBool, []any{true, false, true})
	assertEvalArray(t, `("AAPL";"MSFT") like ("AA*";"M?FT")`, data.KindBool, []any{true, true})
}

func TestEvalWordDyadicVerbsBroadcastAndPromotion(t *testing.T) {
	assertEvalArray(t, "1 2 3 plus 10", data.KindI64, []any{int64(11), int64(12), int64(13)})
	assertEvalArray(t, "10 minus 1 2 3", data.KindI64, []any{int64(9), int64(8), int64(7)})
	assertEvalArray(t, "2 times 1.5 2.5 3.5", data.KindF64, []any{3.0, 5.0, 7.0})
	assertEvalArray(t, "6 8 divide 4", data.KindF64, []any{1.5, 2.0})
	assertEvalArray(t, "1 0N 3 plus 1.5", data.KindF64, []any{2.5, data.NullValue, 4.5})
	assertEvalArray(t, "1 2 3 equal 1", data.KindBool, []any{true, false, false})

	assertEvalArray(t, "`AAPL`MSFT max `IBM", data.KindSymbol, []any{
		data.Symbol("IBM"),
		data.Symbol("MSFT"),
	})
	assertEvalArray(t, `"alpha" "zeta" min "beta"`, data.KindString, []any{"alpha", "beta"})

	date0, err := parseQTemporal("date", "2026.06.06")
	if err != nil {
		t.Fatal(err)
	}
	date1, err := parseQTemporal("date", "2026.06.07")
	if err != nil {
		t.Fatal(err)
	}
	assertEvalArray(t, "2026.06.06 2026.06.07 max 2026.06.06", data.KindDate, []any{date0, date1})
	assertEvalArray(t, "09:30 09:31>09:30", data.KindBool, []any{false, true})
}

func TestEvalPercentSymbolDivideBroadcastAndReciprocal(t *testing.T) {
	assertEvalValue(t, "10%2", 5.0)
	assertEvalArray(t, "10 20 30%10", data.KindF64, []any{1.0, 2.0, 3.0})
	assertEvalArray(t, "10%2 5 4", data.KindF64, []any{5.0, 2.0, 2.5})
	assertEvalArray(t, "10 20 30%2 4 5", data.KindF64, []any{5.0, 5.0, 6.0})
	assertEvalValue(t, "reciprocal 4", 0.25)
}

func TestEvalCoreNumericMonads(t *testing.T) {
	assertEvalValue(t, "neg 5", int64(-5))
	assertEvalArray(t, "neg 1 -2 0N", data.KindI64, []any{int64(-1), int64(2), data.NullValue})
	assertEvalValue(t, "abs -2.5", 2.5)
	assertEvalArray(t, "abs -2 0N 3", data.KindI64, []any{int64(2), data.NullValue, int64(3)})
	assertEvalValue(t, "exp 0", 1.0)
	assertEvalArray(t, "exp 0 1 0N", data.KindF64, []any{1.0, math.E, data.NullValue})
	assertEvalValue(t, "reciprocal 4", 0.25)
	assertEvalArray(t, "reciprocal 2 4 0", data.KindF64, []any{0.5, 0.25, math.Inf(1)})
	assertEvalValue(t, "signum -7", int64(-1))
	assertEvalArray(t, "signum -2 0 3 0N", data.KindI64, []any{int64(-1), int64(0), int64(1), data.NullValue})
	assertEvalValue(t, "floor 2.9", int64(2))
	assertEvalValue(t, "ceiling 2.1", int64(3))
	assertEvalArray(t, "floor 1.9 -1.2 0N", data.KindI64, []any{int64(1), int64(-2), data.NullValue})
	assertEvalArray(t, "ceiling 1.1 -1.8 0N", data.KindI64, []any{int64(2), int64(-1), data.NullValue})
}

func TestEvalDyadicNumericNullPropagation(t *testing.T) {
	assertEvalValue(t, "0N+10", data.NullValue)
	assertEvalArray(t, "10+0N 1 2", data.KindI64, []any{data.NullValue, int64(11), int64(12)})
	assertEvalArray(t, "1 0N 3+10 20 0N", data.KindI64, []any{int64(11), data.NullValue, data.NullValue})
	assertEvalArray(t, "1.5+0N 2", data.KindF64, []any{data.NullValue, 3.5})
	assertEvalArray(t, "0N 1=0N 2", data.KindBool, []any{true, false})

	assertEvalArray(t, "100-':10 0N 15", data.KindI64, []any{int64(-90), data.NullValue, data.NullValue})
	assertEvalArray(t, "1 0N 3+'10 20 30", data.KindI64, []any{int64(11), data.NullValue, int64(33)})
	assertEvalValue(t, "+/1 0N 2", int64(3))
	assertEvalArray(t, "+\\1 0N 2", data.KindI64, []any{int64(1), int64(1), int64(3)})
}

func TestEvalTypedNullOnlyVectorResultsPreserveKind(t *testing.T) {
	ts, err := parseQTemporal("timestamp", "2026.06.06D09:30:00")
	if err != nil {
		t.Fatal(err)
	}

	assertEvalArray(t, "0Ni+1 2 3", data.KindI64, []any{data.NullValue, data.NullValue, data.NullValue})
	assertEvalArray(t, "0Nf*1 2 3", data.KindF64, []any{data.NullValue, data.NullValue, data.NullValue})
	assertEvalArray(t, "0Np=2026.06.06D09:30:00 0Np", data.KindBool, []any{false, true})
	assertEvalArray(t, "1 2 0Ni", data.KindI32, []any{int32(1), int32(2), data.NullValue})
	assertEvalArray(t, "1 2 0Ne", data.KindF32, []any{float32(1), float32(2), data.NullValue})
	assertEvalArray(t, "1 2 0Nf", data.KindF64, []any{1.0, 2.0, data.NullValue})

	assertEvalArray(t, "-\\0Ni 0Ni 0Ni", data.KindI32, []any{data.NullValue, data.NullValue, data.NullValue})
	assertEvalArray(t, "-':0Ni 0Ni 0Ni", data.KindI32, []any{data.NullValue, data.NullValue, data.NullValue})
	assertEvalArray(t, "2026.06.06D09:30:00^0Np 0Np", data.KindTimestamp, []any{
		ts,
		ts,
	})
	assertEvalArray(t, "0Nf^1 2 0Nf", data.KindF64, []any{1.0, 2.0, data.NullValue})
	assertEvalArray(t, "0Ne^1 2 0Ne", data.KindF32, []any{float32(1), float32(2), data.NullValue})
}

func TestEvalTypedNullPromotionBoundaries(t *testing.T) {
	assertEvalValue(t, "0Ni+1h", data.NullForKind(data.KindI64))
	assertEvalValue(t, "0Ne+1i", data.NullForKind(data.KindF64))
	assertEvalValue(t, "0Nf*1e", data.NullForKind(data.KindF64))

	tests := []struct {
		name        string
		left, right any
		kind        data.Kind
		want        []any
	}{
		{
			name:  "i16 i32 promotes to i64",
			left:  data.NewI16([]int16{1, 2}),
			right: data.NewI32([]int32{10, 20}),
			kind:  data.KindI64,
			want:  []any{int64(11), int64(22)},
		},
		{
			name:  "i32 i64 promotes to i64",
			left:  data.NewI32([]int32{1, 2}),
			right: data.NewI64([]int64{10, 20}),
			kind:  data.KindI64,
			want:  []any{int64(11), int64(22)},
		},
		{
			name:  "i64 f32 promotes to f64",
			left:  data.NewI64([]int64{1, 2}),
			right: data.NewF32([]float32{1.5, 2.5}),
			kind:  data.KindF64,
			want:  []any{2.5, 4.5},
		},
		{
			name:  "f32 f64 promotes to f64",
			left:  data.NewF32([]float32{1.5, 2.5}),
			right: data.NewF64([]float64{10, 20}),
			kind:  data.KindF64,
			want:  []any{11.5, 22.5},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := applyDyadic('+', tt.left, tt.right)
			if err != nil {
				t.Fatalf("applyDyadic returned error: %v", err)
			}
			array, ok := got.(data.Array)
			if !ok {
				t.Fatalf("applyDyadic = %#v, want data.Array", got)
			}
			if array.Kind() != tt.kind {
				t.Fatalf("applyDyadic kind = %s, want %s", array.Kind(), tt.kind)
			}
			if values := array.Values(); !reflect.DeepEqual(values, tt.want) {
				t.Fatalf("applyDyadic values = %#v, want %#v", values, tt.want)
			}
		})
	}
}

func TestEvalSymbolStringCompareConsistency(t *testing.T) {
	assertEvalValue(t, "`AAPL=\"AAPL\"", true)
	assertEvalValue(t, "\"AAPL\"=`AAPL", true)
	assertEvalValue(t, "`AAPL<\"MSFT\"", true)
	assertEvalValue(t, "\"MSFT\">`AAPL", true)
	assertEvalArray(t, "`AAPL`MSFT=\"MSFT\"", data.KindBool, []any{false, true})
	assertEvalArray(t, "(\"AAPL\";\"MSFT\")>`IBM", data.KindBool, []any{false, true})
}

func TestEvalCountSumSumsTakeWhereAndPlusAdverbs(t *testing.T) {
	assertEvalValue(t, "count 10 20 30", int64(3))
	assertEvalValue(t, "count 42", int64(1))
	assertEvalValue(t, `count "abcd"`, int64(4))
	assertEvalValue(t, "count ()", int64(0))
	assertEvalValue(t, "count `a`b!10 20", int64(2))
	assertEvalValue(t, "sum 10 20 30", int64(60))
	// Canonical empty-sum identity: sum () -> 0.
	assertEvalValue(t, "sum ()", int64(0))
	assertEvalValue(t, "avg 10 20 30", 20.0)
	assertEvalValue(t, "avg ()", data.NullValue)
	assertEvalValue(t, "var 10 20 30", 66.66666666666669)
	assertEvalValue(t, "dev 10 20 30", 8.16496580927726)
	assertEvalValue(t, "med 10 30 20 40", 25.0)
	assertEvalValue(t, "1 2 3 wavg 10 20 30", 23.333333333333332)
	assertEvalValue(t, "prd 2 3 4", int64(24))
	assertEvalValue(t, "min 10 5 30", int64(5))
	assertEvalValue(t, "max 10 5 30", int64(30))
	// Canonical empty-extrema identities: min () -> 0W, max () -> -0W.
	assertEvalValue(t, "min ()", int64(math.MaxInt64))
	assertEvalValue(t, "max ()", -int64(math.MaxInt64))
	assertEvalValue(t, "first 10 20 30", int64(10))
	assertEvalValue(t, "last 10 20 30", int64(30))
	assertEvalValue(t, "first ()", data.NullValue)
	assertEvalValue(t, "last ()", data.NullValue)
	assertEvalArray(t, "sums 10 20 30", data.KindI64, []any{int64(10), int64(30), int64(60)})
	assertEvalValue(t, "count sums 10 20 30", int64(3))
	assertEvalValue(t, "count (sums 10 20 30)", int64(3))
	assertEvalErrorContains(t, "count sums `a`b", "sums expects a numeric vector")
	assertEvalArray(t, "prds 2 3 4", data.KindI64, []any{int64(2), int64(6), int64(24)})
	assertEvalValue(t, "count prds 2 3 4", int64(3))
	assertEvalArray(t, "ratios 2 4 10", data.KindF64, []any{2.0, 2.0, 2.5})
	assertEvalArray(t, "ratios 2 0N 8 16", data.KindF64, []any{2.0, data.NullValue, 8.0, 2.0})
	assertEvalValue(t, "ratios 0N", data.NullForKind(data.KindF64))
	assertEvalArray(t, "mins 30 20 25 10", data.KindI64, []any{int64(30), int64(20), int64(20), int64(10)})
	assertEvalArray(t, "maxs 30 20 35 10", data.KindI64, []any{int64(30), int64(30), int64(35), int64(35)})
	assertEvalValue(t, "count mins 30 20 25 10", int64(4))
	assertEvalValue(t, "count maxs 30 20 35 10", int64(4))
	assertEvalArray(t, "mins 0N 30 20", data.KindI64, []any{data.NullValue, int64(30), int64(20)})
	assertEvalArray(t, "maxs 0N 30 20", data.KindI64, []any{data.NullValue, int64(30), int64(30)})
	assertEvalArray(t, "avgs 10 20 30 40", data.KindF64, []any{10.0, 15.0, 20.0, 25.0})
	assertEvalValue(t, "count avgs 10 20 30 40", int64(4))
	assertEvalArray(t, "avgs 10 0N 30", data.KindF64, []any{10.0, 10.0, 20.0})
	assertEvalArray(t, "avgs 0N 0N", data.KindF64, []any{data.NullValue, data.NullValue})
	assertEvalArray(t, "3 msum 10 20 30 40", data.KindI64, []any{int64(10), int64(30), int64(60), int64(90)})
	assertEvalArray(t, "3 mavg 10 20 30 40", data.KindF64, []any{10.0, 15.0, 20.0, 30.0})
	assertEvalArray(t, "2#10 20 30", data.KindI64, []any{int64(10), int64(20)})
	assertEvalArray(t, "-2#10 20 30", data.KindI64, []any{int64(20), int64(30)})
	assertEvalArray(t, "5#10 20 30", data.KindI64, []any{int64(10), int64(20), int64(30), int64(10), int64(20)})
	assertEvalArray(t, "-5#10 20 30", data.KindI64, []any{int64(20), int64(30), int64(10), int64(20), int64(30)})
	assertEvalArray(t, "take 2 10 20 30", data.KindI64, []any{int64(10), int64(20)})
	assertEvalArray(t, "take -2 10 20 30", data.KindI64, []any{int64(20), int64(30)})
	assertEvalArray(t, "take 5 `AAPL`MSFT`NVDA", data.KindSymbol, []any{
		data.Symbol("AAPL"),
		data.Symbol("MSFT"),
		data.Symbol("NVDA"),
		data.Symbol("AAPL"),
		data.Symbol("MSFT"),
	})
	assertEvalArray(t, "take 0 10 20 30", data.KindI64, []any{})
	assertEvalArray(t, "4#1", data.KindI64, []any{int64(1), int64(1), int64(1), int64(1)})
	assertEvalArray(t, "2 3#1 2 3 4 5 6", data.KindAny, []any{
		data.NewI64([]int64{1, 2, 3}),
		data.NewI64([]int64{4, 5, 6}),
	})
	assertEvalArray(t, "2 3#1 2 3 4", data.KindAny, []any{
		data.NewI64([]int64{1, 2, 3}),
		data.NewI64([]int64{4, 1, 2}),
	})
	assertEvalArray(t, "flip 2 3#1 2 3 4 5 6", data.KindAny, []any{
		data.NewI64([]int64{1, 4}),
		data.NewI64([]int64{2, 5}),
		data.NewI64([]int64{3, 6}),
	})
	assertEvalArray(t, "flip (1 2 3;4 5 6)", data.KindAny, []any{
		data.NewI64([]int64{1, 4}),
		data.NewI64([]int64{2, 5}),
		data.NewI64([]int64{3, 6}),
	})
	assertEvalArray(t, "(2 3#1 2 3 4 5 6) . 1", data.KindI64, []any{int64(4), int64(5), int64(6)})
	assertEvalValue(t, "(2 3#1 2 3 4 5 6) . 1 2", int64(6))
	assertEvalArray(t, "(2 3#1 2 3 4 5 6) mmu 3 2#10 20 30 40 50 60", data.KindAny, []any{
		data.NewF64([]float64{220, 280}),
		data.NewF64([]float64{490, 640}),
	})
	assertEvalArray(t, "inv 2 2#1 0 0 1", data.KindAny, []any{
		data.NewF64([]float64{1, 0}),
		data.NewF64([]float64{0, 1}),
	})
	assertEvalErrorContains(t, "inv 2 2#1 2 2 4", "non-singular")
	assertEvalArray(t, "take 3 `AAPL", data.KindSymbol, []any{data.Symbol("AAPL"), data.Symbol("AAPL"), data.Symbol("AAPL")})
	assertEvalValue(t, `take 5 "ab"`, "ababa")
	assertEvalValue(t, `take -5 "ab"`, "babab")
	assertEvalValue(t, `take 0 "ab"`, "")
	assertEvalArray(t, "2 rotate 10 20 30 40", data.KindI64, []any{int64(30), int64(40), int64(10), int64(20)})
	assertEvalArray(t, "-1 rotate `AAPL`MSFT`NVDA", data.KindSymbol, []any{
		data.Symbol("NVDA"),
		data.Symbol("AAPL"),
		data.Symbol("MSFT"),
	})
	assertEvalValue(t, `2 rotate "abcd"`, "cdab")
	assertEvalArray(t, "rotate[1;10 20 30]", data.KindI64, []any{int64(20), int64(30), int64(10)})
	assertEvalErrorContains(t, "1.5 rotate 10 20", "integer count")
	assertEvalArray(t, "til 4", data.KindI64, []any{int64(0), int64(1), int64(2), int64(3)})
	assertEvalArray(t, "til 0", data.KindI64, []any{})
	assertEvalErrorContains(t, "til 1.5", "til expects an integer")
	assertEvalValue(t, "+/til 5", int64(10))
	assertEvalArray(t, "where (til 5)>=3", data.KindI64, []any{int64(3), int64(4)})
	assertEvalArray(t, "where true false true", data.KindI64, []any{int64(0), int64(2)})
	assertEvalArray(t, "where 2 0 3", data.KindI64, []any{int64(0), int64(0), int64(2), int64(2), int64(2)})
	assertEvalArray(t, "where 0N 1 0", data.KindI64, []any{int64(1)})
	assertEvalArray(t, "where true", data.KindI64, []any{int64(0)})
	assertEvalArray(t, "where false", data.KindI64, []any{})
	assertEvalArray(t, "where 100 101.5 99.5>100", data.KindI64, []any{int64(1)})
	assertEvalValue(t, "all true 1 2", true)
	assertEvalValue(t, "all true 1 0", false)
	assertEvalValue(t, "any false 0 0N 3", true)
	assertEvalValue(t, "any false 0 0N", false)
	assertEvalValue(t, "all ()", true)
	assertEvalValue(t, "any ()", false)
	assertEvalValue(t, "+/1 2 3", int64(6))
	assertEvalArray(t, "+\\1 2 3", data.KindI64, []any{int64(1), int64(3), int64(6)})
	assertEvalErrorContains(t, "til -1", "non-negative")
	assertEvalErrorContains(t, "where -1 2", "non-negative")
}

func TestEvalBoolAggregateUsesTypedRuntime(t *testing.T) {
	ClearRuntimeKernelExecutionStats()
	t.Cleanup(ClearRuntimeKernelExecutionStats)

	assertEvalValue(t, "all 1 2 3", true)
	assertEvalValue(t, "any 0 0 3", true)
	assertEvalValue(t, "all true 0N true", true)
	assertEvalValue(t, "all not 0 0 0", true)

	seenAllNumeric := false
	seenAnyNumeric := false
	seenAllBool := false
	for _, stat := range RuntimeKernelExecutionStats() {
		if stat.Kernel == "ArrayBoolAggregate" && (stat.Outcome == "fallback" || stat.Outcome == "error") {
			t.Fatalf("unexpected bool aggregate typed runtime fallback/error: %#v stats=%#v", stat, RuntimeKernelExecutionStats())
		}
		if stat.Kernel == "ArrayBoolAggregate" && stat.Shape == "vector-reduce/all/i64" && stat.Outcome == "hit" {
			seenAllNumeric = true
		}
		if stat.Kernel == "ArrayBoolAggregate" && stat.Shape == "vector-reduce/any/i64" && stat.Outcome == "hit" {
			seenAnyNumeric = true
		}
		if stat.Kernel == "ArrayBoolAggregate" && stat.Shape == "vector-reduce/all/bool" && stat.Outcome == "hit" {
			seenAllBool = true
		}
	}
	if !seenAllNumeric || !seenAnyNumeric || !seenAllBool {
		t.Fatalf("missing bool aggregate typed hits allNumeric=%v anyNumeric=%v allBool=%v stats=%#v", seenAllNumeric, seenAnyNumeric, seenAllBool, RuntimeKernelExecutionStats())
	}
}

func TestEvalNumericLogicalSkipsBoolLogicalFallback(t *testing.T) {
	ClearRuntimeKernelExecutionStats()
	t.Cleanup(ClearRuntimeKernelExecutionStats)

	assertEvalArray(t, "1 0 1 and 1 1 0", data.KindI64, []any{int64(1), int64(0), int64(0)})
	assertEvalArray(t, "1 0 1 or 0 1 0", data.KindI64, []any{int64(1), int64(1), int64(1)})
	assertEvalArray(t, "(til 8) and 3", data.KindI64, []any{int64(0), int64(1), int64(2), int64(3), int64(3), int64(3), int64(3), int64(3)})

	seenMinMax := false
	for _, stat := range RuntimeKernelExecutionStats() {
		if stat.Kernel == "ArrayBoolLogical" {
			t.Fatalf("numeric logical recorded bool-logical runtime stat: %#v all=%#v", stat, RuntimeKernelExecutionStats())
		}
		if stat.Kernel == "ArrayDyadicMinMax" && stat.Outcome == "hit" {
			seenMinMax = true
		}
	}
	if !seenMinMax {
		t.Fatalf("missing ArrayDyadicMinMax hit for numeric logical: %#v", RuntimeKernelExecutionStats())
	}
}

func TestEvalWhereRecordsTypedRuntimeKernel(t *testing.T) {
	ClearRuntimeKernelExecutionStats()
	t.Cleanup(ClearRuntimeKernelExecutionStats)

	assertEvalArray(t, "where 10 20 30>=20", data.KindI64, []any{int64(1), int64(2)})
	assertEvalArray(t, "where true false true", data.KindI64, []any{int64(0), int64(2)})
	assertEvalArray(t, "where (10 20 30>=20) and 10 20 30<30", data.KindI64, []any{int64(1)})
	assertEvalValue(t, "m:(10 20 30>=20) and 10 20 30<30;count where m", int64(1))
	assertEvalValue(t, "x:8#1 0Ni 3 0Ni;+/0^x", int64(8))
	assertEvalValue(t, "x:reverse til 5;idx:iasc x;y:x[idx];(first y)+last y+count y", int64(9))
	assertEvalValue(t, "count (sum 1 2 3 4 fby `a`a`b`b)", int64(4))
	assertEvalValue(t, `count where "AAPL" "MSFT" "AMD" "ASK" like "A*"`, int64(3))
	assertEvalValue(t, "count where 8#`AAPL`MSFT`NVDA`TSLA in `AAPL`MSFT", int64(4))
	assertEvalArray(t, "where 8#`AAPL`MSFT`NVDA`TSLA in `AAPL`MSFT", data.KindI64, []any{int64(0), int64(1), int64(4), int64(5)})
	assertEvalValue(t, `count reverse "AAPL" "MSFT" "AMD"`, int64(3))
	assertEvalArray(t, "x:til 8;x[where x>=4]", data.KindI64, []any{int64(4), int64(5), int64(6), int64(7)})
	assertEvalValue(t, "x:til 8;(count where x>=4)+(+/where x>=4)", int64(26))
	assertEvalValue(t, "syms:8#`AAPL`MSFT`NVDA`TSLA;(+/where syms in `AAPL`MSFT)+count where syms in `AAPL`MSFT", int64(14))
	assertEvalValue(t, "x:til 8;y:(x*3)+7;idx:where x>=4;+/y[idx]", int64(94))
	assertEvalValue(t, "x:til 8;y:(x*3)+7;+/y[where x>=4]", int64(94))
	assertEvalValue(t, "x:til 8;y:(x*3)+7;+/y where x>=4", int64(94))
	assertEvalValue(t, "syms:8#`AAPL`MSFT`NVDA`AAPL;px:til 8;idx:where syms=`AAPL;(+/px[idx])+count idx", int64(18))
	assertEvalValue(t, "syms:16#`aapl`amd`xnys`bats`buy`msft;v:til 16;idx:where syms in `aapl`amd`buy;(+/v[idx])+count idx", int64(61))
	assertEvalValue(t, "count distinct 8#`AAPL`MSFT`AAPL`NVDA", int64(3))
	assertEvalValue(t, "x:til 8;y:(x*3)+7;lo:0;hi:4;idx:where (x>=lo) and x<hi;+/y[idx]", int64(46))
	assertEvalValue(t, "x:reverse til 8;idx:iasc x;+/x[idx]", int64(28))
	assertEvalArray(t, "sum 1 2 3 4 fby `a`a`b`b", data.KindI64, []any{int64(3), int64(3), int64(7), int64(7)})
	assertEvalValue(t, "count (sum 1 2 3 4 fby `a`a`b`b)", int64(4))
	assertEvalValue(t, "last ({x+y}\\[10;1 2 3])", int64(16))
	assertEvalValue(t, "count sums 1 2 3 4", int64(4))
	assertEvalValue(t, "count mins 4 3 2 1", int64(4))
	assertEvalValue(t, "count maxs 1 2 3 4", int64(4))
	assertEvalValue(t, "count avgs 1 2 3 4", int64(4))
	assertEvalArray(t, "x:6#0;@[x;1 4;+;2 3]", data.KindI64, []any{int64(0), int64(2), int64(0), int64(0), int64(3), int64(0)})
	assertEvalValue(t, "d:(count distinct)'`a`b!(1 1 2;3 3 3);a:d`a;b:d`b;a+b", int64(3))

	seenWhereCompare := false
	seenWhereMask := false
	seenLikeCount := false
	seenInCount := false
	seenInMask := false
	seenBoolLogical := false
	seenTrueCount := false
	seenScalarFill := false
	seenSortIndexes := false
	seenCountReverse := false
	seenGather := false
	seenFbySum := false
	seenLastCallableScan := false
	seenAmend := false
	seenEachCountDistinct := false
	seenWhereCompareStats := false
	seenWhereCompareCountSum := false
	seenCountSums := false
	seenCountMins := false
	seenCountMaxs := false
	seenCountAvgs := false
	seenCountFby := false
	seenGatherReduce := false
	seenWhereReduce := false
	seenCompareIndexView := false
	seenCompareIndexViewReduce := false
	seenDistinctCount := false
	seenWhereIn := false
	seenWhereInStats := false
	for _, stat := range RuntimeKernelExecutionStats() {
		if stat.Kernel == "ArrayWhereCompare" && stat.Shape == "compare-to-index/>=/i64/i64" && stat.Outcome == "hit" && stat.ReasonCode == "typed_kernel" && stat.Count > 0 {
			seenWhereCompare = true
		}
		if stat.Kernel == "ArrayWhere" && stat.Shape == "mask-to-index/i64" && stat.Outcome == "hit" && stat.ReasonCode == "typed_kernel" && stat.Count > 0 {
			seenWhereMask = true
		}
		if stat.Kernel == "ArrayStringLikeCount" && stat.Shape == "like-count/string/string" && stat.Outcome == "hit" && stat.ReasonCode == "typed_kernel" && stat.Count > 0 {
			seenLikeCount = true
		}
		if stat.Kernel == "ArrayInCount" && stat.Shape == "in-count/symbol/symbol" && stat.Outcome == "hit" && stat.ReasonCode == "typed_kernel" && stat.Count > 0 {
			seenInCount = true
		}
		if stat.Kernel == "ArrayInMask" && stat.Shape == "in-mask/symbol/symbol" && stat.Outcome == "hit" && stat.ReasonCode == "typed_kernel" && stat.Count > 0 {
			seenInMask = true
		}
		if stat.Kernel == "ArrayBoolLogical" && stat.Shape == "bool-logical/and/bool/bool" && stat.Outcome == "hit" && stat.ReasonCode == "typed_kernel" && stat.Count > 0 {
			seenBoolLogical = true
		}
		if stat.Kernel == "ArrayTrueCount" && stat.Shape == "true-count/bool" && stat.Outcome == "hit" && stat.ReasonCode == "typed_kernel" && stat.Count > 0 {
			seenTrueCount = true
		}
		if stat.Kernel == "ArrayScalarFill" && strings.HasPrefix(stat.Shape, "scalar-fill/i64/") && stat.Outcome == "hit" && stat.ReasonCode == "typed_kernel" && stat.Count > 0 {
			seenScalarFill = true
		}
		if stat.Kernel == "ArraySortIndexes" && stat.Shape == "sort-index/i64/asc" && stat.Outcome == "hit" && stat.ReasonCode == "typed_kernel" && stat.Count > 0 {
			seenSortIndexes = true
		}
		if stat.Kernel == "SequenceTransformCount" && stat.Shape == "reverse-count/string" && stat.Outcome == "hit" && stat.ReasonCode == "typed_kernel" && stat.Count > 0 {
			seenCountReverse = true
		}
		if stat.Kernel == "ArrayGatherI64Indexes" && stat.Shape == "gather/i64/i64" && stat.Outcome == "hit" && stat.ReasonCode == "typed_kernel" && stat.Count > 0 {
			seenGather = true
		}
		if stat.Kernel == "ArrayFbySum" && stat.Shape == "fby-sum/i64/symbol" && stat.Outcome == "hit" && stat.ReasonCode == "typed_kernel" && stat.Count > 0 {
			seenFbySum = true
		}
		if stat.Kernel == "CallableLastScan" && stat.Shape == "last-scan/i64" && stat.Outcome == "hit" && stat.ReasonCode == "typed_kernel" && stat.Count > 0 {
			seenLastCallableScan = true
		}
		if stat.Kernel == "ArrayAmendIndexes" && stat.Shape == "amend-indexes/i64" && stat.Outcome == "hit" && stat.ReasonCode == "typed_kernel" && stat.Count > 0 {
			seenAmend = true
		}
		if stat.Kernel == "ArrayAmendAddIndexArray" && stat.Shape == "amend-add-index-array/i64/i64/i64" && stat.Outcome == "hit" && stat.ReasonCode == "typed_kernel" && stat.Count > 0 {
			seenAmend = true
		}
		if stat.Kernel == "ArrayAmendAddIndexes" && stat.Shape == "amend-add-indexes/i64/i64" && stat.Outcome == "hit" && stat.ReasonCode == "typed_kernel" && stat.Count > 0 {
			seenAmend = true
		}
		if stat.Kernel == "CallableEachCountDistinct" && stat.Shape == "dict" && stat.Outcome == "hit" && stat.ReasonCode == "typed_kernel" && stat.Count > 0 {
			seenEachCountDistinct = true
		}
		if stat.Kernel == "ArrayWhereCompareStats" && strings.HasPrefix(stat.Shape, "compare-to-index-count-sum-stats/>=/i64/i64") && stat.Outcome == "hit" && stat.ReasonCode == "typed_kernel" && stat.Count > 0 {
			seenWhereCompareStats = true
		}
		if stat.Kernel == "ArrayWhereCompareCountSum" && stat.Shape == "count-sum" && stat.Outcome == "hit" && stat.ReasonCode == "typed_kernel" && stat.Count > 0 {
			seenWhereCompareCountSum = true
		}
		if stat.Kernel == "ArrayCountSums" && stat.Shape == "vector-count/sums/i64" && stat.Outcome == "hit" && stat.ReasonCode == "typed_kernel" && stat.Count > 0 {
			seenCountSums = true
		}
		if stat.Kernel == "ArrayCountMins" && stat.Shape == "vector-count/mins/i64" && stat.Outcome == "hit" && stat.ReasonCode == "typed_kernel" && stat.Count > 0 {
			seenCountMins = true
		}
		if stat.Kernel == "ArrayCountMaxs" && stat.Shape == "vector-count/maxs/i64" && stat.Outcome == "hit" && stat.ReasonCode == "typed_kernel" && stat.Count > 0 {
			seenCountMaxs = true
		}
		if stat.Kernel == "ArrayCountAvgs" && stat.Shape == "vector-count/avgs/i64" && stat.Outcome == "hit" && stat.ReasonCode == "typed_kernel" && stat.Count > 0 {
			seenCountAvgs = true
		}
		if stat.Kernel == "ArrayCountFby" && stat.Shape == "vector-count/fby-sum/i64/symbol" && stat.Outcome == "hit" && stat.ReasonCode == "typed_kernel" && stat.Count > 0 {
			seenCountFby = true
		}
		if stat.Kernel == "ArrayGatherReduceSum" && stat.Shape == "gather-reduce/i64/i64" && stat.Outcome == "hit" && stat.ReasonCode == "typed_kernel" && stat.Count > 0 {
			seenGatherReduce = true
		}
		if stat.Kernel == "ArrayWhereReduceSum" && strings.HasPrefix(stat.Shape, "where") && stat.Outcome == "hit" && stat.ReasonCode == "typed_kernel" && stat.Count > 0 {
			seenWhereReduce = true
		}
		if stat.Kernel == "ArrayWhereCompareIndexView" && strings.HasPrefix(stat.Shape, "compare-to-index-view/=/symbol/symbol") && stat.Outcome == "hit" && stat.ReasonCode == "typed_kernel" && stat.Count > 0 {
			seenCompareIndexView = true
		}
		if stat.Kernel == "ArrayWhereCompareStats" && strings.HasPrefix(stat.Shape, "compare-to-index-count-sum-stats/=/symbol/symbol") && stat.Outcome == "hit" && stat.ReasonCode == "typed_kernel" && stat.Count > 0 {
			seenCompareIndexView = true
		}
		if stat.Kernel == "ArrayGatherReduceSum" && stat.Shape == "gather-reduce/i64-range/compare-index-view" && stat.Outcome == "hit" && stat.ReasonCode == "typed_kernel" && stat.Count > 0 {
			seenCompareIndexViewReduce = true
		}
		if stat.Kernel == "QScriptPipelinePlan" && stat.Shape == "script-pipeline/gather-reduce/sum-count/assignments" && stat.Outcome == "hit" && stat.Count > 0 {
			seenCompareIndexViewReduce = true
		}
		if stat.Kernel == "ArrayDistinctCount" && stat.Shape == "distinct-count/symbol" && stat.Outcome == "hit" && stat.ReasonCode == "typed_kernel" && stat.Count > 0 {
			seenDistinctCount = true
		}
		if stat.Kernel == "ArrayWhereIn" && strings.HasPrefix(stat.Shape, "in-to-index/symbol/symbol") && stat.Outcome == "hit" && stat.ReasonCode == "typed_kernel" && stat.PipelineShape == "membership_index" && stat.Count > 0 {
			seenWhereIn = true
		}
		if stat.Kernel == "ArrayWhereInStats" && strings.HasPrefix(stat.Shape, "in-to-index-sum/symbol/symbol") && stat.Outcome == "hit" && stat.ReasonCode == "typed_kernel" && stat.PipelineShape == "membership_index_stats" && stat.Count > 0 {
			seenWhereInStats = true
		}
	}
	if !seenWhereCompare || !seenWhereMask || !seenLikeCount || !seenInCount || !seenInMask || !seenBoolLogical || !seenTrueCount || !seenScalarFill || !seenSortIndexes || !seenCountReverse || !seenGather || !seenFbySum || !seenLastCallableScan || !seenAmend || !seenEachCountDistinct || !seenWhereCompareStats || !seenWhereCompareCountSum || !seenCountSums || !seenCountMins || !seenCountMaxs || !seenCountAvgs || !seenCountFby || !seenGatherReduce || !seenWhereReduce || !seenCompareIndexView || !seenCompareIndexViewReduce || !seenDistinctCount || !seenWhereIn || !seenWhereInStats {
		t.Fatalf("missing where typed runtime stats: compare=%v mask=%v like=%v in=%v inMask=%v whereIn=%v whereInStats=%v logical=%v trueCount=%v scalarFill=%v sortIndexes=%v reverse=%v gather=%v fbySum=%v lastScan=%v amend=%v eachDistinct=%v compareStats=%v compareCountSum=%v countSums=%v countMins=%v countMaxs=%v countAvgs=%v countFby=%v gatherReduce=%v whereReduce=%v compareIndexView=%v compareIndexViewReduce=%v distinctCount=%v stats=%#v", seenWhereCompare, seenWhereMask, seenLikeCount, seenInCount, seenInMask, seenWhereIn, seenWhereInStats, seenBoolLogical, seenTrueCount, seenScalarFill, seenSortIndexes, seenCountReverse, seenGather, seenFbySum, seenLastCallableScan, seenAmend, seenEachCountDistinct, seenWhereCompareStats, seenWhereCompareCountSum, seenCountSums, seenCountMins, seenCountMaxs, seenCountAvgs, seenCountFby, seenGatherReduce, seenWhereReduce, seenCompareIndexView, seenCompareIndexViewReduce, seenDistinctCount, RuntimeKernelExecutionStats())
	}
}

func TestEvalFrameRuntimePrimitivesRecordStats(t *testing.T) {
	ClearRuntimeKernelExecutionStats()
	t.Cleanup(ClearRuntimeKernelExecutionStats)

	assertEvalValue(t, "count take 2 flip `sym`price!(`AAPL`MSFT`NVDA;100 101 102)", int64(2))
	assertEvalValue(t, "count drop 1 flip `sym`price!(`AAPL`MSFT`NVDA;100 101 102)", int64(2))
	assertEvalValue(t, "count 1 rotate flip `sym`price!(`AAPL`MSFT`NVDA;100 101 102)", int64(3))
	assertEvalValue(t, "count `price xasc flip `sym`price!(`MSFT`AAPL`NVDA;80 101 210)", int64(3))

	seen := map[string]bool{}
	for _, stat := range RuntimeKernelExecutionStats() {
		if stat.Route != "frame_runtime_primitive" || stat.Outcome != "hit" || stat.ReasonCode != "frame_runtime" || stat.Count == 0 {
			continue
		}
		switch {
		case stat.Kernel == "FrameGather" && stat.PipelineShape == "frame_gather" && strings.HasPrefix(stat.Shape, "frame-gather/take/"):
			seen["take"] = true
		case stat.Kernel == "FrameGather" && stat.PipelineShape == "frame_gather" && strings.HasPrefix(stat.Shape, "frame-gather/drop/"):
			seen["drop"] = true
		case stat.Kernel == "FrameGather" && stat.PipelineShape == "frame_gather" && strings.HasPrefix(stat.Shape, "frame-gather/rotate/"):
			seen["rotate"] = true
		case stat.Kernel == "FrameSort" && stat.PipelineShape == "frame_sort" && strings.HasPrefix(stat.Shape, "frame-sort/xasc/"):
			seen["sort"] = true
		}
	}
	for _, op := range []string{"take", "drop", "rotate", "sort"} {
		if !seen[op] {
			t.Fatalf("missing frame runtime primitive stat for %s: %#v", op, RuntimeKernelExecutionStats())
		}
	}
}

func TestEvalBinRecordsTypedRuntimeKernel(t *testing.T) {
	ClearRuntimeKernelExecutionStats()
	t.Cleanup(ClearRuntimeKernelExecutionStats)

	assertEvalValue(t, "x:10*til 8;probe:til 80;y:x bin probe;+/y", int64(280))
	assertEvalValue(t, "d:2026.06.06 2026.06.07 2026.06.07 2026.06.09;probe:2026.06.05 2026.06.06 2026.06.07 2026.06.08;y:d bin probe;+/y", int64(3))
	assertEvalValue(t, "s:`AAPL`MSFT`MSFT`NVDA;probe:`A`AAPL`MSFT`TSLA;y:s bin probe;+/y", int64(4))

	seenI64 := false
	seenDate := false
	seenSymbol := false
	for _, stat := range RuntimeKernelExecutionStats() {
		if stat.Kernel != "ArrayBin" || stat.Route != "typed_data_kernel" || stat.PipelineShape != "search_index" || stat.Outcome != "hit" || stat.ReasonCode != "typed_kernel" || stat.Count == 0 {
			continue
		}
		switch stat.Shape {
		case "bin/i64/i64":
			seenI64 = true
		case "bin/date/date":
			seenDate = true
		case "bin/symbol/symbol":
			seenSymbol = true
		}
	}
	if !seenI64 || !seenDate || !seenSymbol {
		t.Fatalf("missing bin typed runtime stats: i64=%v date=%v symbol=%v stats=%#v", seenI64, seenDate, seenSymbol, RuntimeKernelExecutionStats())
	}
}

func TestEvalBinReduceUsesTypedPipelineKernel(t *testing.T) {
	state := NewEvalState(nil)
	if plan := state.qPipelinePlan("+/x bin probe"); plan.kind != qPipelineSumBin {
		t.Fatalf("pipeline plan kind = %v, want qPipelineSumBin", plan.kind)
	}

	ClearRuntimeKernelExecutionStats()
	t.Cleanup(ClearRuntimeKernelExecutionStats)

	assertEvalValue(t, "x:10*til 8;probe:til 80;+/x bin probe", int64(280))
	assertEvalValue(t, "d:2026.06.06 2026.06.07 2026.06.07 2026.06.09;probe:2026.06.05 2026.06.06 2026.06.07 2026.06.08;+/d bin probe", int64(3))
	assertEvalValue(t, "s:`AAPL`MSFT`MSFT`NVDA;probe:`A`AAPL`MSFT`TSLA;+/s bin probe", int64(4))

	seenReduce := map[string]bool{}
	seenPipelineHit := false
	seenMaterializedBin := false
	for _, stat := range RuntimeKernelExecutionStats() {
		if stat.Kernel == "ArrayBin" && stat.Outcome == "hit" {
			seenMaterializedBin = true
		}
		if stat.Kernel == "QPipelinePlan" && stat.Shape == "bin-reduce/sum" && stat.PipelineShape == "search_index_reduce" && stat.Outcome == "hit" && stat.ReasonCode == "typed_pipeline" {
			seenPipelineHit = true
		}
		if stat.Kernel != "ArrayBinReduceSum" || stat.Route != "typed_data_kernel" || stat.PipelineShape != "search_index_reduce" || stat.Outcome != "hit" || stat.ReasonCode != "typed_kernel" {
			continue
		}
		seenReduce[stat.Shape] = true
	}
	if seenMaterializedBin {
		t.Fatalf("bin reduce expression materialized ArrayBin path: %#v", RuntimeKernelExecutionStats())
	}
	for _, shape := range []string{"bin-reduce/sum/i64/i64", "bin-reduce/sum/date/date", "bin-reduce/sum/symbol/symbol"} {
		if !seenReduce[shape] {
			t.Fatalf("missing bin reduce stat %s: %#v", shape, RuntimeKernelExecutionStats())
		}
	}
	if !seenPipelineHit {
		t.Fatalf("missing bin reduce pipeline hit: %#v", RuntimeKernelExecutionStats())
	}
}

func TestEvalTemporalCompareRecordsTypedRuntimeKernel(t *testing.T) {
	ClearRuntimeKernelExecutionStats()
	t.Cleanup(ClearRuntimeKernelExecutionStats)

	assertEvalValue(t, "dates:9#2026.06.06 2026.06.07 2026.06.08;count where dates>=2026.06.07", int64(6))
	assertEvalValue(t, "times:8#09:30 09:31 09:32 09:33;count where times within 09:31 09:32", int64(4))
	assertEvalValue(t, "times:8#09:30 09:31 09:32 09:33;v:til 8;idx:where times within 09:31 09:32;(+/v[idx])+count idx", int64(18))

	seenCompareCount := false
	seenWithinCount := false
	seenWithinIndex := false
	seenPipeline := false
	seenDateFallback := false
	for _, stat := range RuntimeKernelExecutionStats() {
		if strings.Contains(stat.Shape, "/date/date") && (stat.Outcome == "fallback" || stat.Outcome == "error") {
			seenDateFallback = true
		}
		if stat.Outcome != "hit" || stat.ReasonCode != "typed_kernel" || stat.Count == 0 {
			if stat.Kernel == "QPipelinePlan" && stat.Shape == "compare-to-index-count" && stat.PipelineShape == "mask_reduce" && stat.Outcome == "hit" && stat.ReasonCode == "typed_pipeline" && stat.Count > 0 {
				seenPipeline = true
			}
			continue
		}
		if stat.Kernel == "ArrayWhereCompareCount" && stat.Shape == "compare-count/>=/date/date" && stat.PipelineShape == "compare_count" {
			seenCompareCount = true
		}
		if stat.Kernel == "ArrayWhereWithinCount" && strings.HasPrefix(stat.Shape, "within-count/minute/minute/minute") && stat.PipelineShape == "within_count" {
			seenWithinCount = true
		}
		if stat.Kernel == "ArrayWhereWithin" && strings.HasPrefix(stat.Shape, "within-to-index/minute/minute/minute") && stat.PipelineShape == "within_index" {
			seenWithinIndex = true
		}
		if stat.Kernel == "ArrayWhereWithinStats" && strings.HasPrefix(stat.Shape, "within-to-index-count-sum-stats/minute/minute/minute") && stat.PipelineShape == "within_index_stats" {
			seenWithinIndex = true
		}
	}
	if !seenCompareCount || !seenWithinCount || !seenWithinIndex || !seenPipeline || seenDateFallback {
		t.Fatalf("missing temporal compare typed runtime stats: count=%v withinCount=%v withinIndex=%v pipeline=%v dateFallback=%v allStats=%#v", seenCompareCount, seenWithinCount, seenWithinIndex, seenPipeline, seenDateFallback, RuntimeKernelExecutionStats())
	}
}

func TestEvalVectorArithmeticRecordsTypedRuntimeKernel(t *testing.T) {
	ClearRuntimeKernelExecutionStats()
	t.Cleanup(ClearRuntimeKernelExecutionStats)

	assertEvalValue(t, "x:til 8;y:(x*3)+7;+/y", int64(140))
	counts := map[string]uint64{}
	for _, stat := range RuntimeKernelExecutionStats() {
		if stat.Kernel == "ArrayDyadicArithmetic" && stat.Outcome == "hit" {
			counts[stat.Kernel] += stat.Count
		}
	}
	if counts["ArrayDyadicArithmetic"] != 2 {
		t.Fatalf("ArrayDyadicArithmetic hits = %d, want 2; stats=%#v", counts["ArrayDyadicArithmetic"], RuntimeKernelExecutionStats())
	}
}

func TestEvalFloatModuloRecordsTypedRuntimeKernel(t *testing.T) {
	ClearRuntimeKernelExecutionStats()
	t.Cleanup(ClearRuntimeKernelExecutionStats)

	assertEvalValue(t, "x:(til 8)*0.5;y:x mod 2;+/y", 6.0)
	seenFloatMod := false
	for _, stat := range RuntimeKernelExecutionStats() {
		if stat.Kernel == "ArrayDyadicArithmetic" && stat.Shape == "vector-dyadic/r/f64/i64" && stat.Outcome == "hit" {
			seenFloatMod = true
		}
		if stat.Kernel == "ArrayDyadicArithmetic" && stat.Shape == "vector-dyadic/r/f64/i64" && stat.Outcome == "fallback" {
			t.Fatalf("float mod fell back: %#v", RuntimeKernelExecutionStats())
		}
	}
	if !seenFloatMod {
		t.Fatalf("missing typed float mod hit: %#v", RuntimeKernelExecutionStats())
	}
}

func TestEvalDyadicMinMaxReduceUsesTypedPipeline(t *testing.T) {
	ClearRuntimeKernelExecutionStats()
	t.Cleanup(ClearRuntimeKernelExecutionStats)

	assertEvalValue(t, "x:til 8;y:reverse x;(+/x min y)+(+/x max y)", int64(56))
	assertEvalValue(t, "x:10 20 30 40;(+/x min 25)+(+/25 max x)", int64(200))

	seenPlan := map[string]bool{}
	seenKernel := map[string]bool{}
	for _, stat := range RuntimeKernelExecutionStats() {
		if stat.Outcome == "fallback" || stat.Outcome == "error" {
			t.Fatalf("unexpected dyadic min/max reduce fallback/error: %#v stats=%#v", stat, RuntimeKernelExecutionStats())
		}
		if stat.Kernel == "QPipelinePlan" && stat.Outcome == "hit" && stat.ReasonCode == "typed_pipeline" {
			seenPlan[stat.Shape] = true
		}
		if stat.Kernel == "ArrayDyadicMinMaxSum" && stat.Outcome == "hit" && stat.ReasonCode == "typed_kernel" {
			seenKernel[stat.Shape] = true
		}
	}
	for _, shape := range []string{"vector-reduce/sum-dyadic-min", "vector-reduce/sum-dyadic-max"} {
		if !seenPlan[shape] {
			t.Fatalf("missing dyadic min/max reduce pipeline %s: %#v", shape, RuntimeKernelExecutionStats())
		}
	}
	if !seenKernel["vector-reduce/sum-dyadic-min/i64/i64"] || !seenKernel["vector-reduce/sum-dyadic-max/i64/i64"] {
		t.Fatalf("missing dyadic min/max reduce typed kernels: %#v", RuntimeKernelExecutionStats())
	}
}

func TestEvalRatiosUsesTypedCarrier(t *testing.T) {
	ClearRuntimeKernelExecutionStats()
	t.Cleanup(ClearRuntimeKernelExecutionStats)

	assertEvalArray(t, "ratios 2 0N 8 16", data.KindF64, []any{2.0, data.NullValue, 8.0, 2.0})
	assertEvalValue(t, "+/ratios 2 0N 8 16", 12.0)

	seenRatios := false
	seenSum := false
	for _, stat := range RuntimeKernelExecutionStats() {
		if stat.Outcome == "fallback" || stat.Outcome == "error" {
			t.Fatalf("unexpected ratios typed runtime fallback/error: %#v stats=%#v", stat, RuntimeKernelExecutionStats())
		}
		if stat.Kernel == "ArrayRatios" && stat.Shape == "vector-scan/ratios/i64" && stat.Outcome == "hit" {
			seenRatios = true
		}
		if stat.Kernel == "SequenceTransformSum" && stat.Shape == "vector-reduce/sum-ratios/i64" && stat.Outcome == "hit" {
			seenSum = true
		}
	}
	if !seenRatios || !seenSum {
		t.Fatalf("missing ratios typed carrier/pipeline sum hits: %#v", RuntimeKernelExecutionStats())
	}
}

func TestEvalDyadicFloatReduceUsesTypedPipeline(t *testing.T) {
	ClearRuntimeKernelExecutionStats()
	t.Cleanup(ClearRuntimeKernelExecutionStats)

	assertEvalValue(t, "+/2 xexp 0 1 2 3", 15.0)
	assertEvalValue(t, "+/2 xlog 2 4 8 0N", 6.0)

	seenKernel := map[string]bool{}
	for _, stat := range RuntimeKernelExecutionStats() {
		if stat.Outcome == "fallback" || stat.Outcome == "error" {
			t.Fatalf("unexpected dyadic float reduce fallback/error: %#v stats=%#v", stat, RuntimeKernelExecutionStats())
		}
		if stat.Kernel == "ArrayNumericDyadicFloatSum" && stat.Outcome == "hit" && stat.ReasonCode == "typed_kernel" {
			seenKernel[stat.Shape] = true
		}
	}
	if !seenKernel["vector-reduce/sum-dyadic-float-xexp/i64/i64"] || !seenKernel["vector-reduce/sum-dyadic-float-xlog/i64/i64"] {
		t.Fatalf("missing dyadic float reduce typed kernels: %#v", RuntimeKernelExecutionStats())
	}
}

func TestEvalTranscendentalUnarySumsUseTypedRuntime(t *testing.T) {
	tests := []struct {
		expr string
		want float64
	}{
		{expr: "+/sqrt 1 4 9 16", want: 10},
		{expr: "+/log 1 2.718281828459045 7.38905609893065", want: 3},
		{expr: "+/sin 0 1.5707963267948966 3.141592653589793", want: 1},
		{expr: "+/cos 0 1.5707963267948966 3.141592653589793", want: 0},
		{expr: "+/tan 0 0.25 0.5", want: math.Tan(0) + math.Tan(0.25) + math.Tan(0.5)},
		{expr: "+/asin -0.5 0 0.5", want: 0},
		{expr: "+/acos -0.5 0 0.5", want: math.Acos(-0.5) + math.Acos(0) + math.Acos(0.5)},
		{expr: "+/atan -1 0 1", want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			ClearRuntimeKernelExecutionStats()
			got, err := Eval(tt.expr)
			if err != nil {
				t.Fatalf("Eval(%q) returned error: %v", tt.expr, err)
			}
			value, ok := got.(float64)
			if !ok || math.Abs(value-tt.want) > 1e-12 {
				t.Fatalf("Eval(%q) = %#v (%T), want %.17g", tt.expr, got, got, tt.want)
			}
			seen := false
			for _, stat := range RuntimeKernelExecutionStats() {
				if stat.Outcome == "fallback" || stat.Outcome == "error" {
					t.Fatalf("unexpected unary math reduce fallback/error: %#v stats=%#v", stat, RuntimeKernelExecutionStats())
				}
				if stat.Kernel == "ArrayNumericUnarySum" && stat.Outcome == "hit" {
					seen = true
				}
			}
			if !seen {
				t.Fatalf("missing ArrayNumericUnarySum hit: %#v", RuntimeKernelExecutionStats())
			}
		})
	}
}

func TestEvalFusesDyadicFloatMathSumDescriptor(t *testing.T) {
	tests := []struct {
		expr  string
		shape string
		want  float64
	}{
		{expr: "+/2 xexp 0 1 2 3", shape: "vector-reduce/sum-dyadic-float-xexp", want: 15},
		{expr: "+/2 xlog 2 4 8", shape: "vector-reduce/sum-dyadic-float-xlog", want: 6},
		{expr: "+/2 3 4 xexp 2", shape: "vector-reduce/sum-dyadic-float-xexp", want: 29},
	}
	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			ClearRuntimeKernelExecutionStats()
			t.Cleanup(ClearRuntimeKernelExecutionStats)

			descriptor, ok := DescribeEvalPipeline(tt.expr)
			if !ok {
				t.Fatalf("DescribeEvalPipeline(%q) did not recognize dyadic float sum", tt.expr)
			}
			if descriptor.Shape != tt.shape || descriptor.PipelineShape != "vector_reduce" || descriptor.ShapeTransform != strings.TrimPrefix(tt.shape, "vector-reduce/sum-") {
				t.Fatalf("descriptor = %#v, want shape=%q vector_reduce", descriptor, tt.shape)
			}
			out, handled, err := ExecuteEvalPipelineDescriptor(descriptor)
			if err != nil || !handled {
				t.Fatalf("ExecuteEvalPipelineDescriptor = %#v,%v,%v", out, handled, err)
			}
			got, ok := out.(float64)
			if !ok || math.Abs(got-tt.want) > 1e-12 {
				t.Fatalf("ExecuteEvalPipelineDescriptor = %#v (%T), want %.17g", out, out, tt.want)
			}

			seen := false
			for _, stat := range RuntimeKernelExecutionStats() {
				if stat.Kernel == "ArrayNumericDyadicFloatSum" && stat.Outcome == "hit" {
					seen = true
				}
				if stat.Outcome == "fallback" || stat.Outcome == "error" {
					t.Fatalf("unexpected runtime fallback/error: %#v all=%#v", stat, RuntimeKernelExecutionStats())
				}
			}
			if !seen {
				t.Fatalf("missing ArrayNumericDyadicFloatSum hit: %#v", RuntimeKernelExecutionStats())
			}
		})
	}
}

func TestEvalMatrixDotIndexRecordsTypedPrimitive(t *testing.T) {
	ClearRuntimeKernelExecutionStats()
	t.Cleanup(ClearRuntimeKernelExecutionStats)

	assertEvalValue(t, "m:2 4#til 8;row:m . 1;(+/row)+(m . 1 2)", int64(28))

	seenRow := false
	seenCell := false
	for _, stat := range RuntimeKernelExecutionStats() {
		if stat.Kernel != "MatrixIndex" || stat.Outcome != "hit" || stat.ReasonCode != "typed_kernel" {
			continue
		}
		if strings.HasSuffix(stat.Shape, "/1-indexes") {
			seenRow = true
		}
		if strings.HasSuffix(stat.Shape, "/2-indexes") {
			seenCell = true
		}
	}
	if !seenRow || !seenCell {
		t.Fatalf("missing matrix index typed primitive stats: row=%v cell=%v stats=%#v", seenRow, seenCell, RuntimeKernelExecutionStats())
	}
}

func TestEvalWhereGatherReduceCompositeMaskStats(t *testing.T) {
	ClearRuntimeKernelExecutionStats()
	t.Cleanup(ClearRuntimeKernelExecutionStats)

	assertEvalValue(t, "x:til 8192;y:(x*3)+7;lo:0;hi:4096;idx:where (x>=lo) and x<hi;+/y[idx]", int64(25188352))
	seenPipelinePlan := false
	seenGatherReduce := false
	for _, stat := range RuntimeKernelExecutionStats() {
		if stat.Outcome == "fallback" || stat.Outcome == "error" {
			t.Fatalf("unexpected runtime fallback/error for composite where gather reduce: %#v stats=%#v", stat, RuntimeKernelExecutionStats())
		}
		if stat.Kernel == "QPipelinePlan" && stat.Shape == "gather-reduce/sum" && stat.PipelineShape == "gather_reduce" && stat.Outcome == "hit" && stat.Count > 0 {
			seenPipelinePlan = true
		}
		if stat.Kernel == "ArrayGatherReduceSum" && stat.Shape == "gather-reduce/i64/i64" && stat.Outcome == "hit" && stat.Count > 0 {
			seenGatherReduce = true
		}
	}
	if !seenPipelinePlan {
		t.Fatalf("missing q pipeline plan typed hit for composite where gather reduce: %#v", RuntimeKernelExecutionStats())
	}
	if !seenGatherReduce {
		t.Fatalf("missing gather reduce typed hit for composite where gather reduce: %#v", RuntimeKernelExecutionStats())
	}
}

func TestEvalModuloWherePipelinesUseFusedTypedKernel(t *testing.T) {
	ClearRuntimeKernelExecutionStats()
	t.Cleanup(ClearRuntimeKernelExecutionStats)

	assertEvalValue(t, "x:til 64;count where (x mod 4)=2", int64(16))
	assertEvalValue(t, "x:til 64;count where (x mod 5)<>3", int64(51))
	assertEvalValue(t, "x:til 64;y:(x*2)+1;idx:where (x mod 3)=0;+/y[idx]", int64(1408))

	seenStats := false
	seenReduce := false
	for _, stat := range RuntimeKernelExecutionStats() {
		if stat.Outcome == "fallback" || stat.Outcome == "error" {
			t.Fatalf("unexpected modulo pipeline fallback/error: %#v stats=%#v", stat, RuntimeKernelExecutionStats())
		}
		if stat.Kernel == "ArrayModuloCompareStats" && stat.Outcome == "hit" && stat.Count > 0 {
			seenStats = true
		}
		if stat.Kernel == "ArrayModuloCompareReduceSum" && stat.Outcome == "hit" && stat.Count > 0 {
			seenReduce = true
		}
	}
	if !seenStats || !seenReduce {
		t.Fatalf("missing modulo fused kernel stats: stats=%v reduce=%v all=%#v", seenStats, seenReduce, RuntimeKernelExecutionStats())
	}
}

func TestEvalVectorExtremaRecordsTypedRuntimeKernel(t *testing.T) {
	ClearRuntimeKernelExecutionStats()
	t.Cleanup(ClearRuntimeKernelExecutionStats)

	assertEvalValue(t, "x:til 8;min x", int64(0))
	assertEvalValue(t, "x:til 8;max x", int64(7))
	counts := map[string]uint64{}
	for _, stat := range RuntimeKernelExecutionStats() {
		if stat.Outcome == "hit" {
			counts[stat.Kernel] += stat.Count
		}
	}
	if counts["ArrayMin"] != 1 || counts["ArrayMax"] != 1 {
		t.Fatalf("typed extrema hits = min %d max %d; stats=%#v", counts["ArrayMin"], counts["ArrayMax"], RuntimeKernelExecutionStats())
	}
}

func TestEvalGenericOverAndScanAdverbs(t *testing.T) {
	assertEvalValue(t, "+/1 0N 2", int64(3))
	assertEvalArray(t, "+\\1 0N 2", data.KindI64, []any{int64(1), int64(1), int64(3)})
	assertEvalValue(t, "(+/)[1 0N 2]", int64(3))
	assertEvalArray(t, "(+\\)[1 0N 2]", data.KindI64, []any{int64(1), int64(1), int64(3)})
	assertEvalValue(t, "*/2 3 4", int64(24))
	assertEvalArray(t, "*\\2 3 4", data.KindI64, []any{int64(2), int64(6), int64(24)})
	assertEvalValue(t, "-/10 2 3", int64(5))
	assertEvalArray(t, "-\\10 2 3", data.KindI64, []any{int64(10), int64(8), int64(5)})
	assertEvalValue(t, "100-/10 2 3", int64(85))
	assertEvalArray(t, "100-\\10 2 3", data.KindI64, []any{int64(90), int64(88), int64(85)})
	assertEvalValue(t, "max/3 7 2 7", int64(7))
	assertEvalArray(t, "max\\3 7 2 9", data.KindI64, []any{int64(3), int64(7), int64(7), int64(9)})
	assertEvalValue(t, "min/3 7 2 9", int64(2))
	assertEvalArray(t, "min\\3 7 2 1", data.KindI64, []any{int64(3), int64(3), int64(2), int64(1)})
	assertEvalValue(t, "max/`AAPL`MSFT`NVDA", data.Symbol("NVDA"))
	assertEvalArray(t, "min\\`MSFT`AAPL`NVDA", data.KindSymbol, []any{
		data.Symbol("MSFT"),
		data.Symbol("AAPL"),
		data.Symbol("AAPL"),
	})
	assertEvalValue(t, "mod/10 4 3", int64(2))
	assertEvalArray(t, "mod\\10 4 3", data.KindI64, []any{int64(10), int64(2), int64(2)})
	assertEvalArray(t, "(-':)[100;10 0N 15]", data.KindI64, []any{int64(-90), data.NullValue, data.NullValue})
	assertEvalArray(t, "(+')[1 0N 3;10 20 30]", data.KindI64, []any{int64(11), data.NullValue, int64(33)})
	assertEvalArray(t, "(+\\:)[10;1 0N 3]", data.KindI64, []any{int64(11), data.NullValue, int64(13)})
	assertEvalArray(t, "(+/:)[1 0N 3;10]", data.KindI64, []any{int64(11), data.NullValue, int64(13)})
}

func TestEvalWordDyadicVerbAdverbs(t *testing.T) {
	assertEvalValue(t, "plus/1 2 3", int64(6))
	assertEvalArray(t, "plus\\1 2 3", data.KindI64, []any{int64(1), int64(3), int64(6)})
	assertEvalValue(t, "(plus/)[1 2 3]", int64(6))
	assertEvalArray(t, "(max\\)[3 7 2 9]", data.KindI64, []any{int64(3), int64(7), int64(7), int64(9)})
	assertEvalArray(t, "minus':10 15 14 20", data.KindI64, []any{int64(10), int64(5), int64(-1), int64(6)})
	assertEvalArray(t, "(minus':)[100;10 15 14]", data.KindI64, []any{int64(-90), int64(5), int64(-1)})
	assertEvalArray(t, "(plus')[1 2 3;10 20 30]", data.KindI64, []any{int64(11), int64(22), int64(33)})
	assertEvalArray(t, "(plus\\:)[10;1 2 3]", data.KindI64, []any{int64(11), int64(12), int64(13)})
	assertEvalArray(t, "(plus/:)[1 2 3;10]", data.KindI64, []any{int64(11), int64(12), int64(13)})
	assertEvalErrorContains(t, "(plus')[1 2;10 20 30]", "each length mismatch")
}

func TestEvalAdverbFunctionProjections(t *testing.T) {
	assertEvalValue(t, "p:(+/);p[1 2 3 4]", int64(10))
	assertEvalArray(t, "p:(+\\);p[1 2 3 4]", data.KindI64, []any{int64(1), int64(3), int64(6), int64(10)})
	assertEvalArray(t, "p:(+');q:p[1 2 3;];q[10 20 30]", data.KindI64, []any{int64(11), int64(22), int64(33)})
	assertEvalArray(t, "p:(+')[;];q:p[1 2 3];q[10 20 30]", data.KindI64, []any{int64(11), int64(22), int64(33)})
	assertEvalArray(t, "p:(-':)[100;];p[10 15 14]", data.KindI64, []any{int64(-90), int64(5), int64(-1)})
	assertEvalArray(t, "p:(-':)[;10 15 14];p[100]", data.KindI64, []any{int64(-90), int64(5), int64(-1)})
	assertEvalArray(t, "p:(-\\:)[10;];p[1 2 3]", data.KindI64, []any{int64(9), int64(8), int64(7)})
	assertEvalArray(t, "p:(-/:)[10 20 30;];p[1]", data.KindI64, []any{int64(9), int64(19), int64(29)})
	assertEvalValue(t, "p:(-/)[;1 2 3];p[100]", int64(94))
	assertEvalArray(t, "p:(-\\)[;1 2 3];p[100]", data.KindI64, []any{int64(99), int64(97), int64(94)})
	assertEvalArray(t, "p:{x+y}[1;];p'[10 20 30]", data.KindI64, []any{int64(11), int64(21), int64(31)})
	assertEvalArray(t, "p:{x+y}[1;];q:p';q[10 20 30]", data.KindI64, []any{int64(11), int64(21), int64(31)})
	assertEvalValue(t, "p:{x+y+z}[1;;];q:p/;q[10 20 30]", int64(62))
	assertEvalArray(t, "p:{x+y+z}[1;;];q:p\\;q[10 20 30]", data.KindI64, []any{int64(10), int64(31), int64(62)})
	assertEvalArray(t, "p:{x+y+z}[1;;];q:p[2];q'[10 20 30]", data.KindI64, []any{int64(13), int64(23), int64(33)})
	assertEvalValue(t, "p:{x+y+z}[;2;];q:p/;q[10 20 30]", int64(64))
	assertEvalArray(t, "p:{x+y+z}[;2;];q:p\\;q[10 20 30]", data.KindI64, []any{int64(10), int64(32), int64(64)})
}

func TestEvalQWordVerbAdverbs(t *testing.T) {
	assertEvalValue(t, "in[2;1 2 3]", true)
	assertEvalValue(t, "within[10;10 20]", true)
	assertEvalValue(t, "bin[10 20 30;25]", int64(1))
	assertEvalValue(t, "binr[10 20 20 30;20]", int64(1))
	assertEvalArray(t, "xbar[10;9 10 21]", data.KindI64, []any{int64(0), int64(10), int64(20)})
	assertEvalArray(t, "xrank[4;til 9]", data.KindI64, []any{int64(0), int64(0), int64(0), int64(1), int64(1), int64(2), int64(2), int64(3), int64(3)})
	assertEvalArray(t, "xrank[3;1 37 5 4 0 3]", data.KindI64, []any{int64(0), int64(2), int64(2), int64(1), int64(0), int64(1)})
	assertEvalArray(t, "in'[1 2 3;2 2 4]", data.KindBool, []any{false, true, false})
	assertEvalArray(t, "(in')[`AAPL`MSFT;`AAPL`IBM]", data.KindBool, []any{true, false})
	assertEvalArray(t, "(within')[9 10 11;(10 20;10 20;10 20)]", data.KindBool, []any{false, true, true})
	// Canonical adverb direction: each-right (/:) iterates the RIGHT operand
	// against the whole left; each-left (\:) iterates the LEFT operand
	// against the whole right (migrated from the pre-canonical inversion).
	assertEvalArray(t, "(bin/:)[10 20 30;5 10 25]", data.KindI64, []any{int64(-1), int64(0), int64(1)})
	assertEvalArray(t, "(binr/:)[10 20 20 30;5 20 25 35]", data.KindI64, []any{int64(0), int64(1), int64(3), int64(4)})
	assertEvalArray(t, "(xbar\\:)[10;9 10 21]", data.KindI64, []any{int64(0), int64(10), int64(20)})
	assertEvalArray(t, "(within\\:)[9 10 11;10 20]", data.KindBool, []any{false, true, true})
	assertEvalArray(t, "in':[1 2;1 1 2]", data.KindBool, []any{true, true, false})
	assertEvalErrorContains(t, "(in/)[1 2 3]", "in cannot be used with over")
	assertEvalErrorContains(t, "xbar\\1 2 3", "xbar cannot be used with scan")
}

func TestEvalCallableMonadicVerbRegistration(t *testing.T) {
	assertEvalArray(t, "key[`a`b!10 20]", data.KindSymbol, []any{data.Symbol("a"), data.Symbol("b")})
	assertEvalArray(t, "keys[`a`b!10 20]", data.KindSymbol, []any{data.Symbol("a"), data.Symbol("b")})
	assertEvalArray(t, "raze[(1 2;3 4;5)]", data.KindI64, []any{int64(1), int64(2), int64(3), int64(4), int64(5)})
	assertEvalArray(t, "sqrt'[4 9 16]", data.KindF64, []any{2.0, 3.0, 4.0})
	assertEvalArray(t, "sin'[0 0]", data.KindF64, []any{0.0, 0.0})
	assertEvalGroupedIndexes(t, "group[`AAPL`MSFT`AAPL]", []any{data.Symbol("AAPL"), data.Symbol("MSFT")}, [][]any{{int64(0), int64(2)}, {int64(1)}})
	assertEvalArray(t, "domain[`sym$`AAPL`MSFT`AAPL]", data.KindSymbol, []any{data.Symbol("AAPL"), data.Symbol("MSFT")})
	assertEvalArray(t, "codes[`sym$`AAPL`MSFT`AAPL]", data.KindI64, []any{int64(0), int64(1), int64(0)})

	assertEvalArray(t, "key'(`a`b!10 20;`x`y!30 40)", data.KindAny, []any{
		data.NewSymbols([]string{"a", "b"}),
		data.NewSymbols([]string{"x", "y"}),
	})
	assertEvalArray(t, "raze'((1 2;3);(4;5 6))", data.KindAny, []any{
		data.NewI64([]int64{1, 2, 3}),
		data.NewI64([]int64{4, 5, 6}),
	})
}

func TestEvalLeftRightDyadicVerbsAndAdverbs(t *testing.T) {
	assertEvalValue(t, "10 left 20", int64(10))
	assertEvalValue(t, "10 right 20", int64(20))
	assertEvalArray(t, "10 left 1 2 3", data.KindI64, []any{int64(10), int64(10), int64(10)})
	assertEvalArray(t, "1 2 3 right 10", data.KindI64, []any{int64(10), int64(10), int64(10)})

	assertEvalValue(t, "left/10 20 30", int64(10))
	assertEvalValue(t, "right/10 20 30", int64(30))
	assertEvalArray(t, "left\\10 20 30", data.KindI64, []any{int64(10), int64(10), int64(10)})
	assertEvalArray(t, "right\\10 20 30", data.KindI64, []any{int64(10), int64(20), int64(30)})
	assertEvalArray(t, "left':10 20 30", data.KindI64, []any{int64(10), int64(20), int64(30)})
	assertEvalArray(t, "right':10 20 30", data.KindI64, []any{int64(10), int64(10), int64(20)})
	assertEvalArray(t, "(left':)[100;10 20 30]", data.KindI64, []any{int64(10), int64(20), int64(30)})
	assertEvalArray(t, "(right':)[100;10 20 30]", data.KindI64, []any{int64(100), int64(10), int64(20)})

	assertEvalArray(t, "(left')[1 2 3;10 20 30]", data.KindI64, []any{int64(1), int64(2), int64(3)})
	assertEvalArray(t, "(right')[1 2 3;10 20 30]", data.KindI64, []any{int64(10), int64(20), int64(30)})
	assertEvalArray(t, "(left\\:)[10;1 2 3]", data.KindI64, []any{int64(10), int64(10), int64(10)})
	assertEvalArray(t, "(right\\:)[10;1 2 3]", data.KindI64, []any{int64(1), int64(2), int64(3)})
	assertEvalArray(t, "(left/:)[1 2 3;10]", data.KindI64, []any{int64(1), int64(2), int64(3)})
	assertEvalArray(t, "(right/:)[1 2 3;10]", data.KindI64, []any{int64(10), int64(10), int64(10)})
}

func TestEvalXbarUsesDataBucketSemantics(t *testing.T) {
	assertEvalArray(t, "10 xbar 0 9 10 19 20", data.KindI64, []any{int64(0), int64(0), int64(10), int64(10), int64(20)})
	assertEvalArray(t, "xbar[10;-21 -20 -19 -10 -1 0 1]", data.KindI64, []any{int64(-30), int64(-20), int64(-20), int64(-10), int64(-10), int64(0), int64(0)})
	assertEvalArray(t, "0.5 xbar 0.1 0.5 0.9 1.0", data.KindF64, []any{0.0, 0.5, 0.5, 1.0})
	assertEvalArray(t, "xbar[0.5;-1.25 -1.0 -0.75 0 0.74 0.75]", data.KindF64, []any{-1.5, -1.0, -1.0, 0.0, 0.5, 0.5})

	ts0, err := parseQTemporal("timestamp", "2026.06.06D09:30:00")
	if err != nil {
		t.Fatal(err)
	}
	ts1, err := parseQTemporal("timestamp", "2026.06.06D09:31:00")
	if err != nil {
		t.Fatal(err)
	}
	assertEvalArray(t, "60000000000 xbar 2026.06.06D09:30:15 2026.06.06D09:31:45", data.KindTimestamp, []any{ts0, ts1})
	assertEvalArray(t, "0D00:01:00 xbar 2026.06.06D09:30:15 0Np 2026.06.06D09:31:45", data.KindTimestamp, []any{ts0, data.NullValue, ts1})
	assertEvalValue(t, "60000000000 xbar 2026.06.06D09:30:15", ts0)
	assertEvalArray(t, "3 xbar 2024.01 2024.02 2024.03 2024.04", data.KindMonth, []any{
		data.MonthFromMonths(648),
		data.MonthFromMonths(648),
		data.MonthFromMonths(648),
		data.MonthFromMonths(651),
	})
	assertEvalArray(t, "7 xbar 2024.01.02 0Nd 2024.01.08", data.KindDate, []any{
		data.DateFromDays(19719),
		data.NullValue,
		data.DateFromDays(19726),
	})
	assertEvalArray(t, "00:01 xbar 09:30 09:30:59 09:31:00", data.KindSecond, []any{
		data.SecondFromSeconds(34_200),
		data.SecondFromSeconds(34_200),
		data.SecondFromSeconds(34_260),
	})
	assertEvalErrorContains(t, "0Np xbar 2026.06.06D09:30:15", "bucket floor interval must be an integer width")
}

func TestEvalFbyAggregateVectors(t *testing.T) {
	assertEvalArray(t, "avg 10 20 30 40 fby `a`a`b`b", data.KindF64, []any{15.0, 15.0, 35.0, 35.0})
	assertEvalArray(t, "var 10 20 30 40 fby `a`a`b`b", data.KindF64, []any{25.0, 25.0, 25.0, 25.0})
	assertEvalArray(t, "med 10 20 30 40 fby `a`a`b`b", data.KindF64, []any{15.0, 15.0, 35.0, 35.0})
	assertEvalArray(t, "sum 10 20 30 40 fby `a`a`b`b", data.KindI64, []any{int64(30), int64(30), int64(70), int64(70)})
	assertEvalArray(t, "count 10 20 30 40 fby `a`a`b`b", data.KindI64, []any{int64(2), int64(2), int64(2), int64(2)})
	assertEvalArray(t, "min 10 20 30 40 fby `a`a`b`b", data.KindI64, []any{int64(10), int64(10), int64(30), int64(30)})
	assertEvalArray(t, "max 10 20 30 40 fby `a`a`b`b", data.KindI64, []any{int64(20), int64(20), int64(40), int64(40)})
	assertEvalArray(t, "first 10 20 30 40 fby `a`a`b`b", data.KindI64, []any{int64(10), int64(10), int64(30), int64(30)})
	assertEvalArray(t, "last 10 20 30 40 fby `a`a`b`b", data.KindI64, []any{int64(20), int64(20), int64(40), int64(40)})
	assertEvalArray(t, "sum 10 0N 30 0N fby `a`a`b`b", data.KindI64, []any{int64(10), int64(10), int64(30), int64(30)})
	assertEvalArray(t, "avg 0N 0N 30 0N fby `a`a`b`b", data.KindF64, []any{data.NullValue, data.NullValue, 30.0, 30.0})
	assertEvalArray(t, "min 0N 0N 30 0N fby `a`a`b`b", data.KindI64, []any{data.NullValue, data.NullValue, int64(30), int64(30)})
	assertEvalArray(t, "first 0N 20 30 40 fby `a`a`b`b", data.KindI64, []any{data.NullValue, data.NullValue, int64(30), int64(30)})
	assertEvalArray(t, "last 0N 20 0N 40 fby `a`a`b`b", data.KindI64, []any{int64(20), int64(20), int64(40), int64(40)})
}

func TestEvalGroupFbyTerminalTypedKernels(t *testing.T) {
	ClearRuntimeKernelExecutionStats()
	t.Cleanup(ClearRuntimeKernelExecutionStats)
	assertEvalValue(t, "v:til 8;g:8#`a`b;s:sum v fby g;+/s", int64(112))
	assertEvalValue(t, "count group (til 16) mod 4", int64(4))
	seenFbySum := false
	seenGroupCount := false
	for _, stat := range RuntimeKernelExecutionStats() {
		if stat.Kernel == "ArrayFbySum" && stat.Outcome == "hit" && stat.ReasonCode == "typed_kernel" && stat.Count > 0 {
			seenFbySum = true
		}
		// count group <array> lowers to the distinct-count typed kernel
		// (group keys are exactly the distinct values).
		if stat.Kernel == "ArrayDistinctCount" && stat.Outcome == "hit" && stat.ReasonCode == "typed_kernel" && stat.Count > 0 {
			seenGroupCount = true
		}
	}
	if !seenFbySum || !seenGroupCount {
		t.Fatalf("missing group/fby typed stats: fby=%v group=%v stats=%#v", seenFbySum, seenGroupCount, RuntimeKernelExecutionStats())
	}
}

func TestEvalSetVerbsAndWhere(t *testing.T) {
	assertEvalArray(t, "?1 2 1 3 2", data.KindI64, []any{int64(1), int64(2), int64(3)})
	assertEvalArray(t, "?`AAPL`MSFT`AAPL", data.KindSymbol, []any{
		data.Symbol("AAPL"),
		data.Symbol("MSFT"),
	})
	assertEvalValue(t, "10 20 30?20", int64(1))
	assertEvalValue(t, "10 20 30?99", int64(3))
	assertEvalArray(t, "10 20 30?30 10 99", data.KindI64, []any{int64(2), int64(0), int64(3)})
	assertEvalArray(t, "`AAPL`MSFT`NVDA?`NVDA`IBM", data.KindI64, []any{int64(2), int64(3)})
	assertEvalValue(t, `"abcd"?"c"`, int64(2))
	assertEvalArray(t, `"abcd"?"dbx"`, data.KindI64, []any{int64(3), int64(1), int64(4)})

	assertEvalArray(t, "1 2 3 2 except 2 4", data.KindI64, []any{int64(1), int64(3)})
	assertEvalArray(t, "1 2 1 3 inter 3 1 9", data.KindI64, []any{int64(1), int64(3)})
	assertEvalArray(t, "1 2 1 3 intersect 3 1 9", data.KindI64, []any{int64(1), int64(3)})
	assertEvalArray(t, "1 2 1 union 2 3 0N", data.KindI64, []any{int64(1), int64(2), int64(3), data.NullValue})
	assertEvalArray(t, "1 2 3 0N in 2 3 0N", data.KindBool, []any{false, true, true, true})
	assertEvalValue(t, "`MSFT in `AAPL`MSFT", true)
	assertEvalValue(t, "10 20 30 bin 25", int64(1))
	assertEvalValue(t, "10 20 30 bin 5", int64(-1))
	assertEvalArray(t, "10 20 30 bin 5 10 29 30 31", data.KindI64, []any{int64(-1), int64(0), int64(1), int64(2), int64(2)})
	assertEvalValue(t, "10 20 20 20 30 binr 20", int64(1))
	assertEvalArray(t, "10 20 20 20 30 binr 5 15 20 25 35", data.KindI64, []any{int64(0), int64(1), int64(1), int64(4), int64(5)})
	assertEvalArray(t, "2026.06.06 2026.06.07 2026.06.09 bin 2026.06.05 2026.06.08", data.KindI64, []any{int64(-1), int64(1)})
	assertEvalArray(t, "2026.06.06 2026.06.07 2026.06.09 binr 2026.06.05 2026.06.08", data.KindI64, []any{int64(0), int64(2)})
	assertEvalArray(t, "10 20 30 0N within 15 30", data.KindBool, []any{false, true, true, false})
	assertEvalValue(t, "2026.06.07 within 2026.06.06 2026.06.08", true)
	assertEvalArray(t, "2 xrank 40 10 30 20", data.KindI64, []any{int64(1), int64(0), int64(1), int64(0)})
	assertEvalArray(t, "3 xrank 10 10 20 30 0N", data.KindI64, []any{int64(0), int64(0), int64(1), int64(2), data.NullValue})
	assertEvalArray(t, "2 xrank `MSFT`AAPL`NVDA`AAPL", data.KindI64, []any{int64(1), int64(0), int64(1), int64(0)})
	assertEvalValue(t, "4 xrank 100", int64(0))
	assertEvalErrorContains(t, "0 xrank 10 20", "positive bucket count")
	assertEvalArray(t, "not true false true", data.KindBool, []any{false, true, false})
	assertEvalArray(t, "not 0 2 0N", data.KindBool, []any{true, false, true})
	assertEvalArray(t, "not true false 0 2 0N", data.KindBool, []any{false, true, true, false, true})
	assertEvalArray(t, "null 0N 1 0N", data.KindBool, []any{true, false, true})
	assertEvalArray(t, "true false true and true true false", data.KindBool, []any{true, false, false})
	assertEvalArray(t, "false true false or false false true", data.KindBool, []any{false, true, true})

	assertEvalArray(t, "`AAPL`MSFT`AAPL`NVDA except `MSFT", data.KindSymbol, []any{
		data.Symbol("AAPL"),
		data.Symbol("AAPL"),
		data.Symbol("NVDA"),
	})
	assertEvalArray(t, "`AAPL`MSFT`AAPL inter `AAPL`GOOG", data.KindSymbol, []any{
		data.Symbol("AAPL"),
	})
	assertEvalArray(t, "`AAPL`MSFT union `MSFT`NVDA`AAPL", data.KindSymbol, []any{
		data.Symbol("AAPL"),
		data.Symbol("MSFT"),
		data.Symbol("NVDA"),
	})

	assertEvalArray(t, "0N 1 0N 2 except 0N", data.KindI64, []any{int64(1), int64(2)})
	assertEvalArray(t, "0N 1 0N inter 0N 2", data.KindI64, []any{data.NullValue})
	assertEvalArray(t, "0N 1 union 0N 2", data.KindI64, []any{data.NullValue, int64(1), int64(2)})
	assertEvalArray(t, "where false false", data.KindI64, []any{})
	assertEvalArray(t, "where 3", data.KindI64, []any{int64(0), int64(0), int64(0)})

	grouped, err := Eval("group 1 2 1 3 2")
	if err != nil {
		t.Fatalf("Eval(group ints) returned error: %v", err)
	}
	groupDict, ok := grouped.(EvalDict)
	if !ok {
		t.Fatalf("group ints = %#v, want EvalDict", grouped)
	}
	if !reflect.DeepEqual(groupDict.Keys, []any{int64(1), int64(2), int64(3)}) {
		t.Fatalf("group keys = %#v", groupDict.Keys)
	}
	assertDictIndexArray(t, groupDict, 0, []any{int64(0), int64(2)})
	assertDictIndexArray(t, groupDict, 1, []any{int64(1), int64(4)})
	assertDictIndexArray(t, groupDict, 2, []any{int64(3)})

	grouped, err = Eval("group `AAPL`MSFT`AAPL")
	if err != nil {
		t.Fatalf("Eval(group symbols) returned error: %v", err)
	}
	groupDict, ok = grouped.(EvalDict)
	if !ok {
		t.Fatalf("group symbols = %#v, want EvalDict", grouped)
	}
	if !reflect.DeepEqual(groupDict.Keys, []any{data.Symbol("AAPL"), data.Symbol("MSFT")}) {
		t.Fatalf("symbol group keys = %#v", groupDict.Keys)
	}
	assertDictIndexArray(t, groupDict, 0, []any{int64(0), int64(2)})

	grouped, err = Eval("group 0N 1 0N")
	if err != nil {
		t.Fatalf("Eval(group nulls) returned error: %v", err)
	}
	groupDict, ok = grouped.(EvalDict)
	if !ok {
		t.Fatalf("group nulls = %#v, want EvalDict", grouped)
	}
	if len(groupDict.Keys) != 2 || !data.IsNull(groupDict.Keys[0]) || groupDict.Keys[1] != int64(1) {
		t.Fatalf("null group keys = %#v", groupDict.Keys)
	}
	assertDictIndexArray(t, groupDict, 0, []any{int64(0), int64(2)})
}

func TestEvalBooleanAndNullVerbs(t *testing.T) {
	assertEvalValue(t, "not true", false)
	assertEvalValue(t, "not 0", true)
	assertEvalArray(t, "not true false true", data.KindBool, []any{false, true, false})
	assertEvalArray(t, "not 0 2 0N", data.KindBool, []any{true, false, true})
	assertEvalArray(t, "(not 0 1 0) and true true false", data.KindBool, []any{true, false, false})
	assertEvalValue(t, "x:8#0 1 2 3;r:x mod 2;count where not r", int64(4))
	assertEvalValue(t, "null 0N", true)
	assertEvalArray(t, "null 10 0N 20 0n", data.KindBool, []any{false, true, false, true})
	assertEvalValue(t, "count where null 10 0N 20 0n", int64(2))
	assertEvalValue(t, "count where null 0N", int64(1))
	assertEvalValue(t, "count where null 42", int64(0))
	assertEvalErrorContains(t, "not `AAPL", "bool or numeric")
	assertEvalErrorContains(t, "null[1;2]", "unary function null expected 1 argument")
	// Canonical and/or: min/max with bool->int promotion on mixed operands.
	assertEvalValue(t, "true and 1", int64(1))
	assertEvalValue(t, "false or 2", int64(2))
	assertEvalArray(t, "true false true and true true false", data.KindBool, []any{true, false, false})
	assertEvalArray(t, "0 1 2 or false", data.KindI64, []any{int64(0), int64(1), int64(2)})
}

func TestEvalLogicalVerbAdverbs(t *testing.T) {
	// Canonical and/or are min/max: nulls sort smallest (null&x is null,
	// null|x is x) and mixed bool/numeric operands promote per q rules.
	assertEvalValue(t, "null true and 0N", true)
	assertEvalValue(t, "0N or 2", int64(2))
	assertEvalValue(t, "(1 0N 2 and'1 1 0)~1 0N 0", true)
	assertEvalValue(t, "(1 0N 2 or'0 0 0N)~1 0 2", true)
	assertEvalValue(t, "(true and\\:1 0N 2)~1 0N 1", true)
	assertEvalValue(t, "(0 0N 2 or/:false)~0 0 2", true)
	assertEvalValue(t, "(and/true 1 0N)~0N", true)
	assertEvalValue(t, "or/false 0 0N 2", int64(2))
	assertEvalValue(t, "(and\\true 1 0N 2)~(1b;1;0N;0N)", true)
	assertEvalValue(t, "(or\\false 0 0N 2)~(0b;0;0;2)", true)
	assertEvalValue(t, "((and/)[true 1 0N])~0N", true)
	assertEvalValue(t, "((or\\)[false 0 0N 2])~(0b;0;0;2)", true)
	// Empty-vector reducer identities per element type.
	assertEvalValue(t, "(&/)[til 0]", int64(math.MaxInt64))
	assertEvalValue(t, "(|/)[til 0]", int64(-math.MaxInt64))
	assertEvalValue(t, "(&/)[0#0.0]", math.Inf(1))
	assertEvalValue(t, "(|/)[0#0.0]", math.Inf(-1))
	assertEvalValue(t, "(&/)[0#0b]", true)
	assertEvalValue(t, "(|/)[0#0b]", false)
}

func TestEvalNonNumericComparisons(t *testing.T) {
	assertEvalArray(t, "`AAPL`MSFT`AAPL=`AAPL", data.KindBool, []any{true, false, true})
	assertEvalArray(t, `"alpha" "beta" "alpha"="alpha"`, data.KindBool, []any{true, false, true})
	assertEvalValue(t, "`AAPL=\"AAPL\"", true)
	assertEvalValue(t, "\"AAPL\"=`AAPL", true)
	assertEvalArray(t, "`AAPL`MSFT=\"AAPL\"", data.KindBool, []any{true, false})
	assertEvalArray(t, "\"AAPL\" \"MSFT\"=`MSFT", data.KindBool, []any{false, true})
	assertEvalArray(t, "`AAPL`MSFT<`BABA", data.KindBool, []any{true, false})
	assertEvalArray(t, `"alpha" "beta">"aardvark"`, data.KindBool, []any{true, true})

	assertEvalArray(t, "where `AAPL`MSFT`AAPL=`AAPL", data.KindI64, []any{int64(0), int64(2)})
	assertEvalArray(t, "where \"alpha\" \"beta\" \"alpha\"=\"alpha\"", data.KindI64, []any{int64(0), int64(2)})

	date0, err := parseQTemporal("date", "2026.06.06")
	if err != nil {
		t.Fatal(err)
	}
	date1, err := parseQTemporal("date", "2026.06.07")
	if err != nil {
		t.Fatal(err)
	}
	assertEvalArray(t, "2026.06.06 2026.06.07=2026.06.06", data.KindBool, []any{true, false})
	assertEvalArray(t, "2026.06.06 2026.06.07<2026.06.07", data.KindBool, []any{true, false})
	assertEvalArray(t, "2026.06.06 2026.06.07 inter 2026.06.07 2026.06.08", data.KindDate, []any{date1})
	assertEvalArray(t, "2026.06.06 2026.06.07 except 2026.06.07", data.KindDate, []any{date0})
}

func TestEvalCompositeComparisons(t *testing.T) {
	assertEvalArray(t, "10 20 30<>20", data.KindBool, []any{true, false, true})
	assertEvalArray(t, "10 20 30<=20", data.KindBool, []any{true, true, false})
	ClearRuntimeKernelExecutionStats()
	assertEvalArray(t, "10 20 30>=20", data.KindBool, []any{false, true, true})
	stats := RuntimeKernelExecutionStats()
	foundCompositeCompare := false
	for _, stat := range stats {
		if stat.Kernel == "ArrayDyadicCompare" && stat.Shape == "vector-dyadic/>=/i64/i64" && stat.Outcome == "hit" {
			foundCompositeCompare = true
			break
		}
	}
	if !foundCompositeCompare {
		t.Fatalf("10 20 30>=20 did not record direct typed composite compare; stats=%#v", stats)
	}
	assertEvalArray(t, "where 10 20 30>=20", data.KindI64, []any{int64(1), int64(2)})

	assertEvalArray(t, "`AAPL`MSFT`NVDA<>`MSFT", data.KindBool, []any{true, false, true})
	assertEvalArray(t, "`AAPL`MSFT<>\"MSFT\"", data.KindBool, []any{true, false})
	assertEvalArray(t, "`AAPL`MSFT`NVDA>=`MSFT", data.KindBool, []any{false, true, true})
	assertEvalArray(t, "where `AAPL`MSFT`NVDA<>`MSFT", data.KindI64, []any{int64(0), int64(2)})

	assertEvalArray(t, "2026.06.06 2026.06.07>=2026.06.07", data.KindBool, []any{false, true})
	assertEvalArray(t, "2026.06.06 2026.06.07<>2026.06.07", data.KindBool, []any{true, false})
}

func TestEvalDrop(t *testing.T) {
	assertEvalArray(t, "2_10 20 30 40", data.KindI64, []any{int64(30), int64(40)})
	assertEvalArray(t, "-2_10 20 30 40", data.KindI64, []any{int64(10), int64(20)})
	assertEvalArray(t, "0_10 20 30", data.KindI64, []any{int64(10), int64(20), int64(30)})
	assertEvalArray(t, "drop 1 `AAPL`MSFT`NVDA", data.KindSymbol, []any{
		data.Symbol("MSFT"),
		data.Symbol("NVDA"),
	})
	assertEvalArray(t, "9_10 20", data.KindI64, []any{})
	assertEvalArray(t, "-9_10 20", data.KindI64, []any{})
	assertEvalValue(t, `2_"abcdef"`, "cdef")
	assertEvalValue(t, `-2_"abcdef"`, "abcd")
	assertEvalValue(t, `9_"abcdef"`, "")
	assertEvalValue(t, `1_"åßcd"`, "ßcd")

	got, err := Eval("1_flip `sym`price!(`AAPL`MSFT`NVDA;100 101 102)")
	if err != nil {
		t.Fatalf("Eval(drop frame) returned error: %v", err)
	}
	frame, ok := got.(data.Frame)
	if !ok {
		t.Fatalf("Eval(drop frame) = %#v, want data.Frame", got)
	}
	if frame.Len() != 2 {
		t.Fatalf("drop frame len = %d, want 2", frame.Len())
	}
	assertFrameValue(t, frame, "sym", 0, data.Symbol("MSFT"))
	assertFrameValue(t, frame, "price", 1, int64(102))
}

func TestEvalCut(t *testing.T) {
	got, err := Eval("0 2 4_10 20 30 40 50")
	if err != nil {
		t.Fatalf("Eval(cut vector) returned error: %v", err)
	}
	segments, ok := got.(data.Array)
	if !ok || segments.Len() != 3 {
		t.Fatalf("cut vector = %#v, want 3 segments", got)
	}
	assertNestedArray(t, segments, 0, data.KindI64, []any{int64(10), int64(20)})
	assertNestedArray(t, segments, 1, data.KindI64, []any{int64(30), int64(40)})
	assertNestedArray(t, segments, 2, data.KindI64, []any{int64(50)})

	got, err = Eval(`0 2_"åßcd"`)
	if err != nil {
		t.Fatalf("Eval(cut string) returned error: %v", err)
	}
	stringSegments, ok := got.(data.Array)
	if !ok || stringSegments.Len() != 2 {
		t.Fatalf("cut string = %#v, want 2 segments", got)
	}
	if item, _ := stringSegments.At(0); item != "åß" {
		t.Fatalf("string segment 0 = %#v, want åß", item)
	}
	if item, _ := stringSegments.At(1); item != "cd" {
		t.Fatalf("string segment 1 = %#v, want cd", item)
	}

	got, err = Eval("0 2_flip `sym`price!(`AAPL`MSFT`NVDA;100 101 102)")
	if err != nil {
		t.Fatalf("Eval(cut frame) returned error: %v", err)
	}
	frameSegments, ok := got.(data.Array)
	if !ok || frameSegments.Len() != 2 {
		t.Fatalf("cut frame = %#v, want 2 segments", got)
	}
	first, _ := frameSegments.At(0)
	firstFrame, ok := first.(data.Frame)
	if !ok || firstFrame.Len() != 2 {
		t.Fatalf("first frame segment = %#v", first)
	}
	assertFrameValue(t, firstFrame, "sym", 1, data.Symbol("MSFT"))
	second, _ := frameSegments.At(1)
	secondFrame, ok := second.(data.Frame)
	if !ok || secondFrame.Len() != 1 {
		t.Fatalf("second frame segment = %#v", second)
	}
	assertFrameValue(t, secondFrame, "price", 0, int64(102))

	assertEvalErrorContains(t, "(1;`a)_10 20 30", "integer cut indexes")
	assertEvalErrorContains(t, "-1 2_10 20 30", "non-negative")
}

func TestEvalListStringAndBoolGaps(t *testing.T) {
	assertEvalValue(t, "1b", true)
	assertEvalValue(t, "0b", false)
	assertEvalArray(t, "101b", data.KindBool, []any{true, false, true})
	assertEvalArray(t, "0 1b", data.KindBool, []any{false, true})
	assertEvalValue(t, "count where 101001b", int64(3))

	got, err := Eval("cut[0 2 4;10 20 30 40 50]")
	if err != nil {
		t.Fatalf("Eval(cut function) returned error: %v", err)
	}
	segments, ok := got.(data.Array)
	if !ok || segments.Len() != 3 {
		t.Fatalf("cut function = %#v, want 3 segments", got)
	}
	assertNestedArray(t, segments, 0, data.KindI64, []any{int64(10), int64(20)})
	assertNestedArray(t, segments, 1, data.KindI64, []any{int64(30), int64(40)})
	assertNestedArray(t, segments, 2, data.KindI64, []any{int64(50)})

	assertEvalArray(t, "1 3 sublist 10 20 30 40 50", data.KindI64, []any{int64(20), int64(30), int64(40)})
	assertEvalArray(t, "sublist[1 3;10 20 30 40 50]", data.KindI64, []any{int64(20), int64(30), int64(40)})
	assertEvalValue(t, `1 2 sublist "åßcd"`, "ßc")

	crossed, err := Eval("1 2 cross `a`b")
	if err != nil {
		t.Fatalf("Eval(cross) returned error: %v", err)
	}
	crossArray, ok := crossed.(data.Array)
	if !ok || crossArray.Len() != 4 {
		t.Fatalf("cross = %#v, want 4 pairs", crossed)
	}
	assertNestedArray(t, crossArray, 0, data.KindAny, []any{int64(1), data.Symbol("a")})
	assertNestedArray(t, crossArray, 3, data.KindAny, []any{int64(2), data.Symbol("b")})

	assertEvalValue(t, `trim "  AAPL  "`, "AAPL")
	assertEvalValue(t, `ltrim "  AAPL  "`, "AAPL  ")
	assertEvalValue(t, `rtrim "  AAPL  "`, "  AAPL")
	assertEvalArray(t, `trim " a " " b"`, data.KindString, []any{"a", "b"})
	assertEvalValue(t, `count "åß"`, int64(2))
	assertEvalValue(t, `count trim " åß "`, int64(2))

	assertEvalArray(t, `"banana" ss "an"`, data.KindI64, []any{int64(1), int64(3)})
	assertEvalValue(t, `ssr["banana";"an";"ON"]`, "bONONa")
	assertEvalValue(t, `"a-b-c" ssr ("-";"+")`, "a+b+c")
	assertEvalValue(t, `"," sv "AAPL" "MSFT" "NVDA"`, "AAPL,MSFT,NVDA")
	assertEvalArray(t, `"," vs "AAPL,MSFT,NVDA"`, data.KindString, []any{"AAPL", "MSFT", "NVDA"})
	assertEvalArray(t, `"" vs "åß"`, data.KindString, []any{"å", "ß"})
}

func TestEvalEnlistAndRaze(t *testing.T) {
	enlisted, err := Eval("enlist 42")
	if err != nil {
		t.Fatalf("Eval(enlist) error: %v", err)
	}
	array, ok := enlisted.(data.Array)
	if !ok || array.Len() != 1 {
		t.Fatalf("enlist = %#v, want single item array", enlisted)
	}
	assertEvalArray(t, "count' enlist' 1 2 3", data.KindI64, []any{int64(1), int64(1), int64(1)})
	assertEvalArray(t, "raze (1 2;3 4;5)", data.KindI64, []any{int64(1), int64(2), int64(3), int64(4), int64(5)})
	assertEvalArray(t, "raze (`AAPL`MSFT;`NVDA)", data.KindSymbol, []any{data.Symbol("AAPL"), data.Symbol("MSFT"), data.Symbol("NVDA")})
	// Canonical raze of an empty list is the list itself: raze () -> ().
	assertEvalArray(t, "raze ()", data.KindAny, []any{})
	assertEvalValue(t, "raze 42", int64(42))
	assertEvalValue(t, "count enlist 10 20 30", int64(1))
}

func TestEvalSortAndSortIndexes(t *testing.T) {
	assertEvalArray(t, "asc 3 1 2 1", data.KindI64, []any{int64(1), int64(1), int64(2), int64(3)})
	assertEvalArray(t, "desc 3 1 2 1", data.KindI64, []any{int64(3), int64(2), int64(1), int64(1)})
	assertEvalArray(t, "iasc 3 1 2 1", data.KindI64, []any{int64(1), int64(3), int64(2), int64(0)})
	assertEvalArray(t, "idesc 3 1 2 1", data.KindI64, []any{int64(0), int64(2), int64(1), int64(3)})
	assertEvalArray(t, "rank 2 7 3 2 5", data.KindI64, []any{int64(0), int64(4), int64(2), int64(1), int64(3)})
	assertEvalArray(t, "rank 50 20 30 10 40", data.KindI64, []any{int64(4), int64(1), int64(2), int64(0), int64(3)})
	assertEvalArray(t, "2 xprev 10 20 30 40", data.KindI64, []any{data.NullValue, data.NullValue, int64(10), int64(20)})
	assertEvalArray(t, "iasc ()", data.KindI64, []any{})
	assertEvalArray(t, "rank ()", data.KindI64, []any{})
	assertEvalArray(t, "idesc 42", data.KindI64, []any{int64(0)})
	assertEvalArray(t, "rank 42", data.KindI64, []any{int64(0)})
	assertEvalArray(t, "iasc `MSFT`AAPL`AAPL", data.KindI64, []any{int64(1), int64(2), int64(0)})
	assertEvalValue(t, "s:10#`b`a`b`c;+/iasc s", int64(45))
	assertEvalValue(t, "s:10#`b`a`b`c;+/idesc s", int64(45))
	assertEvalArray(t, "rank `x`a`b`z`c", data.KindI64, []any{int64(3), int64(0), int64(1), int64(4), int64(2)})

	assertEvalArray(t, "asc 10.5 9 10", data.KindF64, []any{9.0, 10.0, 10.5})
	assertEvalArray(t, "desc `MSFT`AAPL`NVDA", data.KindSymbol, []any{
		data.Symbol("NVDA"),
		data.Symbol("MSFT"),
		data.Symbol("AAPL"),
	})
	assertEvalArray(t, `asc "beta" "alpha" "gamma"`, data.KindString, []any{"alpha", "beta", "gamma"})

	date0, err := parseQTemporal("date", "2026.06.06")
	if err != nil {
		t.Fatal(err)
	}
	date1, err := parseQTemporal("date", "2026.06.07")
	if err != nil {
		t.Fatal(err)
	}
	assertEvalArray(t, "asc 2026.06.07 0Nd 2026.06.06", data.KindDate, []any{data.NullValue, date0, date1})
	assertEvalArray(t, "iasc 2026.06.07 0Nd 2026.06.06", data.KindI64, []any{int64(1), int64(2), int64(0)})
	assertEvalArray(t, "rank 2026.06.07 0Nd 2026.06.06", data.KindI64, []any{int64(2), int64(0), int64(1)})
}

func TestEvalXPrevSumUsesTypedRuntime(t *testing.T) {
	ClearRuntimeKernelExecutionStats()
	t.Cleanup(ClearRuntimeKernelExecutionStats)

	assertEvalValue(t, "+/2 xprev 10 20 30 40", int64(30))
	seenSum := false
	for _, stat := range RuntimeKernelExecutionStats() {
		if stat.Kernel == "ArraySum" && stat.Outcome == "hit" && stat.ReasonCode == "typed_kernel" {
			seenSum = true
		}
		if stat.Kernel == "ArraySum" && (stat.Outcome == "fallback" || stat.Outcome == "error") {
			t.Fatalf("unexpected ArraySum fallback/error: %#v all=%#v", stat, RuntimeKernelExecutionStats())
		}
	}
	if !seenSum {
		t.Fatalf("missing typed ArraySum hit: %#v", RuntimeKernelExecutionStats())
	}
}

func TestEvalSortRankReducerBundleRecordsTypedRuntimeKernel(t *testing.T) {
	ClearRuntimeKernelExecutionStats()
	t.Cleanup(ClearRuntimeKernelExecutionStats)
	assertEvalValue(t, "(+/iasc 3 1 2 1)+(+/rank `x`a`b`z`c)+(first asc 3 1 2)+(first desc 3 1 2)", int64(20))
	seenBundle := false
	seenRank := false
	seenEdge := false
	for _, stat := range RuntimeKernelExecutionStats() {
		if stat.Outcome == "fallback" || stat.Outcome == "error" {
			t.Fatalf("unexpected sort/rank reducer fallback/error: %#v all=%#v", stat, RuntimeKernelExecutionStats())
		}
		if stat.Kernel == "SortRankReducerBundle" && stat.Outcome == "hit" && stat.ReasonCode == "typed_kernel" {
			seenBundle = true
		}
		if stat.Kernel == "ArrayRankSum" && stat.Outcome == "hit" && stat.Shape == "rank-sum/symbol" {
			seenRank = true
		}
		if stat.Kernel == "ArraySortedEdge" && stat.Outcome == "hit" && strings.HasPrefix(stat.Shape, "sort-edge/i64/") {
			seenEdge = true
		}
	}
	if !seenBundle || !seenRank || !seenEdge {
		t.Fatalf("missing sort/rank reducer stats: bundle=%v rank=%v edge=%v all=%#v", seenBundle, seenRank, seenEdge, RuntimeKernelExecutionStats())
	}
}

func TestEvalCallableOverScanSumPipelineRecordsTypedRuntimeKernel(t *testing.T) {
	ClearRuntimeKernelExecutionStats()
	t.Cleanup(ClearRuntimeKernelExecutionStats)
	assertEvalValue(t, "x:1+til 8;s:+\\x;({x+y}/[10;x])+last s+count s", int64(90))
	seen := false
	seenSummary := false
	for _, stat := range RuntimeKernelExecutionStats() {
		if stat.Outcome == "fallback" || stat.Outcome == "error" {
			t.Fatalf("unexpected callable-over scan fallback/error: %#v all=%#v", stat, RuntimeKernelExecutionStats())
		}
		if stat.Kernel == "CallableOverScanSum" && stat.Outcome == "hit" && stat.Count > 0 {
			seen = true
			if strings.Contains(stat.Shape, "summary/i64") {
				seenSummary = true
			}
		}
	}
	if !seen {
		t.Fatalf("missing CallableOverScanSum hit: %#v", RuntimeKernelExecutionStats())
	}
	if !seenSummary {
		t.Fatalf("missing CallableOverScanSum summary hit: %#v", RuntimeKernelExecutionStats())
	}
}

func TestQSortRankReducerPlusTerms(t *testing.T) {
	terms := qSortRankReducerPlusTerms("(+/iasc 3 1 2 1)+(+/rank `x`a`b`z`c)+(first asc 3 1 2)+(first desc 3 1 2)")
	want := []string{"+/iasc 3 1 2 1", "+/rank `x`a`b`z`c", "first asc 3 1 2", "first desc 3 1 2"}
	if !reflect.DeepEqual(terms, want) {
		t.Fatalf("terms = %#v, want %#v", terms, want)
	}
}

func TestQSortRankReducerPlanHoistsOnlyStaticArgs(t *testing.T) {
	terms := buildQSortRankReducerBundlePlan("(+/iasc 3 1 2 1)+(+/rank x)")
	if len(terms) != 2 {
		t.Fatalf("plan terms = %#v, want 2", terms)
	}
	if !terms[0].hasArgValue {
		t.Fatalf("literal sort/rank arg was not hoisted: %#v", terms[0])
	}
	if terms[1].hasArgValue {
		t.Fatalf("dynamic sort/rank arg was hoisted: %#v", terms[1])
	}
	assertEvalValue(t, "x:4 1 3;(+/iasc 3 1 2 1)+(+/rank x)", int64(9))
}

func TestEvalConditionalSpecialForms(t *testing.T) {
	assertEvalValue(t, "$[1;2;3]", int64(2))
	assertEvalValue(t, "$[0;2;3]", int64(3))
	assertEvalValue(t, "?[1;2;3]", int64(2))
	assertEvalValue(t, "?[0;2;3]", int64(3))
	assertEvalValue(t, "a:$[1;2;3];a+10", int64(12))
	assertEvalValue(t, "a:?[0;2;3];a+10", int64(13))
	assertEvalValue(t, "$[1;$[0;9;8];3]", int64(8))
	assertEvalValue(t, "?[0;2;?[1;4;5]]", int64(4))
	assertEvalValue(t, "f:{$[x>0;1;-1]};f[2]", int64(1))
	assertEvalValue(t, "f:{?[x>0;1;-1]};f[-2]", int64(-1))
	assertEvalValue(t, "f:{[x]a:$[x>0;1;2];a+10};f[-2]", int64(12))
	assertEvalValue(t, "f:{[x;y]$[x>y;x;y]};f[2;5]", int64(5))
	assertEvalValue(t, "f:{[x;y]x+y};f[$[1;2;3];?[0;4;5]]", int64(7))
	assertEvalValue(t, "$[1;42;1%0]", int64(42))
}

func TestQSemicolonSplitRespectsNestedBracketForms(t *testing.T) {
	got := splitQScriptStatements("a:$[1;2;3];b:?[0;4;5];a+b")
	want := []string{"a:$[1;2;3]", "b:?[0;4;5]", "a+b"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("script statements = %#v, want %#v", got, want)
	}

	got = splitQBracketFormArgs("x>0;$[y;1;2];?[z;3;4]")
	want = []string{"x>0", "$[y;1;2]", "?[z;3;4]"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("bracket args = %#v, want %#v", got, want)
	}
}

func TestEvalControlSpecialForms(t *testing.T) {
	assertEvalValue(t, "a:0;if[1;a:42];a", int64(42))
	assertEvalValue(t, "a:0;if[0;a:42];a", int64(0))
	assertEvalValue(t, "a:0;do[3;a:a+2];a", int64(6))
	assertEvalValue(t, "i:0;while[i<3;i:i+1];i", int64(3))
	assertEvalValue(t, "a:0;b:0;if[1;a:10;b:a+5];b", int64(15))
	assertEvalValue(t, "a:0;b:0;if[0;a:10;b:a+5];b", int64(0))
	assertEvalValue(t, "a:0;b:0;do[3;a:a+1;b:b+a];b", int64(6))
	assertEvalValue(t, "i:0;s:0;while[i<3;i:i+1;s:s+i];s", int64(6))
}

func TestEvalSqrtLogUnaryVerbs(t *testing.T) {
	assertEvalValue(t, "sqrt 4", 2.0)
	assertEvalValue(t, "log 1", 0.0)
	assertEvalArray(t, "sqrt 4 9 16", data.KindF64, []any{2.0, 3.0, 4.0})
}

func TestEvalStatsWindowVerbs(t *testing.T) {
	assertEvalValue(t, "svar 1 2 3", 1.0)
	assertEvalValue(t, "sdev 1 2 3", 1.0)
	assertEvalValue(t, "wsum 1 2 3", int64(6))
	assertEvalValue(t, "1 2 3 wsum 10 20 30", int64(140))
	assertEvalValue(t, "wsum[1 2 3;10 20 30]", int64(140))
	assertEvalValue(t, "1 2 3 cov 1 2 3", float64(2)/3)
	assertEvalValue(t, "1 2 3 scov 1 2 3", 1.0)
	assertEvalValue(t, "1 2 3 cor 1 2 3", 1.0)
	assertEvalArray(t, "2 mdev 1 2 3", data.KindF64, []any{0.0, 0.5, 0.5})
	assertEvalArray(t, "mdev[2;1 2 3]", data.KindF64, []any{0.0, 0.5, 0.5})
	assertEvalArray(t, "0.5 ema 1 2 3", data.KindF64, []any{1.0, 1.5, 2.25})
	assertEvalArray(t, "ema[0.5;1 2 3]", data.KindF64, []any{1.0, 1.5, 2.25})
	assertEvalErrorContains(t, "1 2 cov `a`b", "numeric vectors")
}

func TestEvalAdditionalMathVerbs(t *testing.T) {
	assertEvalValue(t, "sin 0", 0.0)
	assertEvalValue(t, "cos 0", 1.0)
	assertEvalValue(t, "tan 0", 0.0)
	assertEvalValue(t, "asin 0", 0.0)
	assertEvalValue(t, "acos 1", 0.0)
	assertEvalValue(t, "atan 0", 0.0)
	assertEvalArray(t, "sin 0 0N 0", data.KindF64, []any{0.0, data.NullValue, 0.0})

	assertEvalValue(t, "2 xexp 3", 8.0)
	assertEvalValue(t, "xexp[2;3]", 8.0)
	assertEvalValue(t, "2 xlog 8", 3.0)
	assertEvalValue(t, "xlog[2;8]", 3.0)
	assertEvalArray(t, "2 3 xexp 3", data.KindF64, []any{8.0, 27.0})
	assertEvalArray(t, "2 xexp 3 4", data.KindF64, []any{8.0, 16.0})
	assertEvalArray(t, "2 0N 4 xexp 3 3 0N", data.KindF64, []any{8.0, data.NullValue, data.NullValue})
}

func TestEvalSortSupportsDataStringArrays(t *testing.T) {
	got, err := asc(data.NewString([]string{"beta", "alpha", "gamma"}))
	if err != nil {
		t.Fatalf("asc returned error: %v", err)
	}
	array, ok := got.(data.Array)
	if !ok {
		t.Fatalf("asc = %#v, want data.Array", got)
	}
	if array.Kind() != data.KindString {
		t.Fatalf("asc kind = %s, want %s", array.Kind(), data.KindString)
	}
	if values := array.Values(); !reflect.DeepEqual(values, []any{"alpha", "beta", "gamma"}) {
		t.Fatalf("asc values = %#v", values)
	}
}

func TestEvalExtremaOverSymbolsAndTemporalValues(t *testing.T) {
	assertEvalValue(t, "min `MSFT`AAPL`NVDA", data.Symbol("AAPL"))
	assertEvalValue(t, "max `MSFT`AAPL`NVDA", data.Symbol("NVDA"))

	date, err := parseQTemporal("date", "2026.06.06")
	if err != nil {
		t.Fatal(err)
	}
	ts, err := parseQTemporal("timestamp", "2026.06.07D09:31:00")
	if err != nil {
		t.Fatal(err)
	}
	assertEvalValue(t, "min 2026.06.07 0Nd 2026.06.06", date)
	assertEvalValue(t, "max 2026.06.06D09:30:00 0Np 2026.06.07D09:31:00", ts)
}

func TestEvalEachAdverbs(t *testing.T) {
	assertEvalArray(t, "1 2 3+'10 20 30", data.KindI64, []any{int64(11), int64(22), int64(33)})
	assertEvalArray(t, "1 2 3+'10", data.KindI64, []any{int64(11), int64(12), int64(13)})
	assertEvalArray(t, "10+'1 2 3", data.KindI64, []any{int64(11), int64(12), int64(13)})
	assertEvalErrorContains(t, "1 2 3+'()", "length mismatch")
	assertEvalArray(t, "1 2 3 plus'10 20 30", data.KindI64, []any{int64(11), int64(22), int64(33)})
	assertEvalArray(t, "2*'1.5 2.5 3.5", data.KindF64, []any{3.0, 5.0, 7.0})
	assertEvalArray(t, "1 2 3 equal'1 0 3", data.KindBool, []any{true, false, true})
	assertEvalArray(t, "1 2 3 max'3 1 2", data.KindI64, []any{int64(3), int64(2), int64(3)})
	assertEvalArray(t, "`AAPL`MSFT max'`IBM`IBM", data.KindSymbol, []any{
		data.Symbol("IBM"),
		data.Symbol("MSFT"),
	})

	assertEvalArray(t, "count' (1 2;3 4 5;6)", data.KindI64, []any{int64(2), int64(3), int64(1)})
	assertEvalArray(t, "type' (1 2;`a`b;\"xy\")", data.KindI64, []any{int64(7), int64(11), int64(10)})
	assertEvalArray(t, "string' (`AAPL`MSFT;2024.01.02 0Nd)", data.KindAny, []any{
		data.NewString([]string{"AAPL", "MSFT"}),
		data.NewString([]string{"2024-01-02", ""}),
	})
	// upper/lower are type-preserving on symbols (canonical q).
	assertEvalArray(t, "lower' (`AAPL`MSFT;\"Alpha\" \"Beta\")", data.KindAny, []any{
		data.NewSymbols([]string{"aapl", "msft"}),
		data.NewString([]string{"alpha", "beta"}),
	})
	assertEvalArray(t, "upper' (`aapl`msft;\"alpha\" \"Beta\")", data.KindAny, []any{
		data.NewSymbols([]string{"AAPL", "MSFT"}),
		data.NewString([]string{"ALPHA", "BETA"}),
	})
	assertEvalArray(t, "f:+';f[1 2 3;10 20 30]", data.KindI64, []any{int64(11), int64(22), int64(33)})
	assertEvalArray(t, "f:+';g:f[;10];g[1 2 3]", data.KindI64, []any{int64(11), int64(12), int64(13)})
	assertEvalArray(t, "f:+';g:f[1 2 3;];g[10]", data.KindI64, []any{int64(11), int64(12), int64(13)})
	assertEvalArray(t, "f:{x+y}';f[1 2 3;10 20 30]", data.KindI64, []any{int64(11), int64(22), int64(33)})
	assertEvalErrorContains(t, "1 2 3+'10 20", "length mismatch")
	assertEvalErrorContains(t, "(+')[1;2;3]", "adverb function expected")
}

func TestEvalEachPriorAdverb(t *testing.T) {
	assertEvalArray(t, "-':()", data.KindNull, nil)
	assertEvalArray(t, "-':0#10 20", data.KindI64, []any{})
	assertEvalArray(t, "f:-':;f[0#10 20]", data.KindI64, []any{})
	assertEvalArray(t, "f:{x-y}':;f[0#10 20]", data.KindI64, []any{})
	assertEvalValue(t, "-':10", int64(10))
	assertEvalArray(t, "-':10 15", data.KindI64, []any{int64(10), int64(5)})
	assertEvalArray(t, "-':10 15 14 20", data.KindI64, []any{int64(10), int64(5), int64(-1), int64(6)})
	assertEvalArray(t, "minus':10 15 14 20", data.KindI64, []any{int64(10), int64(5), int64(-1), int64(6)})
	assertEvalValue(t, "100-':10", int64(-90))
	assertEvalArray(t, "100-':10 15 14", data.KindI64, []any{int64(-90), int64(5), int64(-1)})
	assertEvalArray(t, "max':`AAPL`MSFT`IBM", data.KindSymbol, []any{
		data.Symbol("AAPL"),
		data.Symbol("MSFT"),
		data.Symbol("MSFT"),
	})
	assertEvalArray(t, "f:-':;f[10 15 14 20]", data.KindI64, []any{int64(10), int64(5), int64(-1), int64(6)})
	assertEvalArray(t, "f:-':;f[100;10 15 14]", data.KindI64, []any{int64(-90), int64(5), int64(-1)})
	assertEvalArray(t, "f:-':;g:f[100;];g[10 15]", data.KindI64, []any{int64(-90), int64(5)})
	assertEvalArray(t, "f:{x-y}':;f[10 15 14 20]", data.KindI64, []any{int64(10), int64(5), int64(-1), int64(6)})
	assertEvalErrorContains(t, "count':10 20", "cannot be used with each-prior")
	assertEvalErrorContains(t, "(-':)[1;2;3]", "expected 1 or 2 arguments")
}

func TestEvalEachLeftAndEachRightAdverbs(t *testing.T) {
	assertEvalValue(t, "10-\\:1", int64(9))
	assertEvalValue(t, "10-/:1", int64(9))
	assertEvalArray(t, "10-\\:()", data.KindNull, nil)
	assertEvalArray(t, "()-/:10", data.KindNull, nil)
	assertEvalArray(t, "10-\\:0#1 2 3", data.KindI64, []any{})
	assertEvalArray(t, "(0#10 20 30)-/:1", data.KindI64, []any{})
	assertEvalArray(t, "fl:-\\:;fl[10;0#1 2 3]", data.KindI64, []any{})
	assertEvalArray(t, "fr:-/:;fr[0#10 20 30;1]", data.KindI64, []any{})
	assertEvalArray(t, "fl:{x-y}\\:;fl[10;0#1 2 3]", data.KindI64, []any{})
	assertEvalArray(t, "fr:{x-y}/:;fr[0#10 20 30;1]", data.KindI64, []any{})
	assertEvalArray(t, "10-\\:1 2 3", data.KindI64, []any{int64(9), int64(8), int64(7)})
	assertEvalArray(t, "10 minus\\:1 2 3", data.KindI64, []any{int64(9), int64(8), int64(7)})
	assertEvalArray(t, "10 20 30-/:1", data.KindI64, []any{int64(9), int64(19), int64(29)})
	assertEvalArray(t, "10 20 30 minus/:1", data.KindI64, []any{int64(9), int64(19), int64(29)})
	assertEvalArray(t, "2 3 4>/:3", data.KindBool, []any{false, false, true})
	assertEvalArray(t, "3 less\\:2 3 4", data.KindBool, []any{false, false, true})
	assertEvalArray(t, "`IBM max\\:`AAPL`MSFT", data.KindSymbol, []any{
		data.Symbol("IBM"),
		data.Symbol("MSFT"),
	})
	assertEvalArray(t, "`AAPL`MSFT min/:`IBM", data.KindSymbol, []any{
		data.Symbol("AAPL"),
		data.Symbol("IBM"),
	})
	assertEvalArray(t, "fl:-\\:;fl[10;1 2 3]", data.KindI64, []any{int64(9), int64(8), int64(7)})
	assertEvalArray(t, "fr:-/:;fr[10 20 30;1]", data.KindI64, []any{int64(9), int64(19), int64(29)})
	assertEvalArray(t, "fl:-\\:;g:fl[10;];g[1 2 3]", data.KindI64, []any{int64(9), int64(8), int64(7)})
	assertEvalArray(t, "fr:-/:;g:fr[;1];g[10 20 30]", data.KindI64, []any{int64(9), int64(19), int64(29)})
	assertEvalArray(t, "fl:{x-y}\\:;fl[10;1 2 3]", data.KindI64, []any{int64(9), int64(8), int64(7)})
	assertEvalArray(t, "fr:{x-y}/:;fr[10 20 30;1]", data.KindI64, []any{int64(9), int64(19), int64(29)})
	assertEvalErrorContains(t, "(-\\:)[10]", "expected 2 arguments")
	assertEvalErrorContains(t, "(-/:)[10]", "expected 2 arguments")
}

func TestEvalOverAndScanAdverbs(t *testing.T) {
	// Canonical empty-fold identity: (+/)() -> 0.
	assertEvalValue(t, "+/()", int64(0))
	assertEvalArray(t, "+\\()", data.KindI64, []any{})
	assertEvalValue(t, "count (+\\())", int64(0))
	assertEvalValue(t, "count sums ()", int64(0))
	assertEvalValue(t, "10+/()", int64(10))
	assertEvalArray(t, "10+\\()", data.KindNull, nil)
	assertEvalValue(t, "+/1 2 3 4", int64(10))
	assertEvalValue(t, "plus/1 2 3 4", int64(10))
	assertEvalValue(t, "10+/1 2 3", int64(16))
	assertEvalValue(t, "10 plus/1 2 3", int64(16))
	assertEvalValue(t, "f:+/;f[1 2 3 4]", int64(10))
	assertEvalValue(t, "f:+/;f[10;1 2 3]", int64(16))
	assertEvalValue(t, "f:+/;g:f[10;];g[1 2 3]", int64(16))
	assertEvalValue(t, "f:{x+y}/;f[1 2 3 4]", int64(10))
	assertEvalValue(t, "f:{x+y}/;f[10;1 2 3]", int64(16))

	assertEvalArray(t, "+\\1 2 3 4", data.KindI64, []any{int64(1), int64(3), int64(6), int64(10)})
	assertEvalValue(t, "+\\42", int64(42))
	assertEvalArray(t, "plus\\1 2 3 4", data.KindI64, []any{int64(1), int64(3), int64(6), int64(10)})
	assertEvalArray(t, "10+\\1 2 3", data.KindI64, []any{int64(11), int64(13), int64(16)})
	assertEvalArray(t, "10 plus\\1 2 3", data.KindI64, []any{int64(11), int64(13), int64(16)})
	assertEvalArray(t, "f:+\\;f[1 2 3 4]", data.KindI64, []any{int64(1), int64(3), int64(6), int64(10)})
	assertEvalArray(t, "f:+\\;f[10;1 2 3]", data.KindI64, []any{int64(11), int64(13), int64(16)})
	assertEvalArray(t, "f:+\\;g:f[10;];g[1 2 3]", data.KindI64, []any{int64(11), int64(13), int64(16)})
	assertEvalArray(t, "f:{x+y}\\;f[1 2 3 4]", data.KindI64, []any{int64(1), int64(3), int64(6), int64(10)})
	assertEvalArray(t, "f:{x+y}\\;f[10;1 2 3]", data.KindI64, []any{int64(11), int64(13), int64(16)})
	assertEvalArray(t, "-\\0#10 20", data.KindI64, []any{})
	assertEvalArray(t, "*\\0#1.5 2.5", data.KindF64, []any{})

	assertEvalErrorContains(t, "inter/`a`b`c", "cannot be used with over")
	assertEvalErrorContains(t, "inter\\`a`b`c", "cannot be used with scan")
	assertEvalErrorContains(t, "(+/)[1;2;3]", "expected 1 or 2 arguments")
}

func TestEvalDistinctReversePrevDeltasAndFills(t *testing.T) {
	assertEvalArray(t, "distinct 10 20 10 30 20", data.KindI64, []any{int64(10), int64(20), int64(30)})
	assertEvalArray(t, "distinct `AAPL`MSFT`AAPL", data.KindSymbol, []any{
		data.Symbol("AAPL"),
		data.Symbol("MSFT"),
	})
	assertEvalArray(t, "reverse 10 20 30", data.KindI64, []any{int64(30), int64(20), int64(10)})
	assertEvalArray(t, "reverse `AAPL`MSFT`NVDA", data.KindSymbol, []any{
		data.Symbol("NVDA"),
		data.Symbol("MSFT"),
		data.Symbol("AAPL"),
	})
	assertEvalValue(t, `reverse "åßcd"`, "dcßå")
	assertEvalArray(t, "prior 10 20 30", data.KindI64, []any{data.NullValue, int64(10), int64(20)})
	assertEvalArray(t, "prior[10 20 30]", data.KindI64, []any{data.NullValue, int64(10), int64(20)})
	assertEvalArray(t, "prior' (10 20 30;100 200)", data.KindAny, []any{
		data.InferArray([]any{data.NullValue, int64(10), int64(20)}),
		data.InferArray([]any{data.NullValue, int64(100)}),
	})
	assertEvalArray(t, "prev 10 20 30", data.KindI64, []any{data.NullValue, int64(10), int64(20)})
	assertEvalArray(t, "next 10 20 30", data.KindI64, []any{int64(20), int64(30), data.NullValue})
	assertEvalArray(t, "next 0Ni 1i 0Ni", data.KindI32, []any{int32(1), data.NullValue, data.NullValue})
	assertEvalArray(t, "deltas 10 15 14 20", data.KindI64, []any{int64(10), int64(5), int64(-1), int64(6)})
	assertEvalArray(t, "deltas til 4", data.KindI64, []any{int64(0), int64(1), int64(1), int64(1)})
	assertEvalArray(t, "deltas 10 12.5 13", data.KindF64, []any{10.0, 2.5, 0.5})
	assertEvalArray(t, "deltas 1 0N 3", data.KindI64, []any{int64(1), data.NullValue, data.NullValue})
	assertEvalArray(t, "differ 10 10 20 20 10", data.KindBool, []any{true, false, true, false, true})
	assertEvalArray(t, "differ `AAPL`AAPL`MSFT`MSFT", data.KindBool, []any{true, false, true, false})
	assertEvalArray(t, "differ 0N 0N 1 0N", data.KindBool, []any{true, false, true, true})
	assertEvalArray(t, "fills prev 10 20 30", data.KindI64, []any{data.NullValue, int64(10), int64(20)})
	assertEvalArray(t, "prev 0Ni 0Ni", data.KindI32, []any{data.NullValue, data.NullValue})
	assertEvalArray(t, "deltas 1i 0Ni 3i", data.KindI32, []any{int32(1), data.NullValue, data.NullValue})
	assertEvalArray(t, "fills 0Nf 1.5f 0Nf", data.KindF64, []any{data.NullValue, 1.5, 1.5})
	assertEvalArray(t, "fills 0Ne 1.5e 0Ne", data.KindF32, []any{data.NullValue, float32(1.5), float32(1.5)})
	assertEvalArray(t, "next[10 20 30]", data.KindI64, []any{int64(20), int64(30), data.NullValue})
	assertEvalArray(t, "differ[`AAPL`AAPL`MSFT]", data.KindBool, []any{true, false, true})
}

func TestEvalDeltasRecordsTypedRuntimeKernel(t *testing.T) {
	ClearRuntimeKernelExecutionStats()
	t.Cleanup(ClearRuntimeKernelExecutionStats)

	assertEvalArray(t, "deltas til 4", data.KindI64, []any{int64(0), int64(1), int64(1), int64(1)})
	assertEvalValue(t, "sum deltas til 8", int64(7))
	assertEvalValue(t, "+/deltas 10 15 14 20", int64(20))
	assertEvalValue(t, "sum deltas 1 0N 3", int64(1))
	seenDeltas := false
	seenDeltasSum := false
	for _, stat := range RuntimeKernelExecutionStats() {
		if stat.Kernel == "ArrayDeltas" && stat.Outcome == "hit" && stat.Count > 0 {
			seenDeltas = true
		}
		if stat.Kernel == "ArrayDeltasSum" && stat.Shape == "vector-reduce/sum-deltas/i64" && stat.Outcome == "hit" && stat.Count > 0 {
			seenDeltasSum = true
		}
	}
	if !seenDeltas || !seenDeltasSum {
		t.Fatalf("missing deltas runtime stats: deltas=%v deltasSum=%v stats=%#v", seenDeltas, seenDeltasSum, RuntimeKernelExecutionStats())
	}
}

func TestEvalSequenceTransformSumAndCountRecordsRuntimeKernel(t *testing.T) {
	cases := []struct {
		name string
		expr string
		want any
	}{
		{name: "reverse_sum", expr: "+/reverse til 8", want: int64(28)},
		{name: "rotate_sum", expr: "+/3 rotate til 8", want: int64(28)},
		{name: "sublist_sum", expr: "+/2 4 sublist til 8", want: int64(14)},
		{name: "ratios_sum", expr: "+/ratios 2 4 8 16", want: float64(8)},
		{name: "reverse_count", expr: "count reverse til 8", want: int64(8)},
		{name: "ratios_count", expr: "count ratios 2 4 8 16", want: int64(4)},
		{name: "deltas_count_after_fills", expr: "count deltas fills 1 0N 3", want: int64(3)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ClearRuntimeKernelExecutionStats()
			t.Cleanup(ClearRuntimeKernelExecutionStats)

			assertEvalValue(t, tc.expr, tc.want)

			seen := false
			for _, stat := range RuntimeKernelExecutionStats() {
				if stat.Outcome == "fallback" || stat.Outcome == "error" {
					t.Fatalf("unexpected sequence transform fallback/error: %#v all=%#v", stat, RuntimeKernelExecutionStats())
				}
				if stat.Kernel == "SequenceTransformSum" && stat.Outcome == "hit" {
					seen = true
				}
				if stat.Kernel == "SequenceTransformCount" && stat.Outcome == "hit" {
					seen = true
				}
				// Length-preserving transform-chain counts resolve through the
				// dedicated ArrayCount* kernels without materializing the chain.
				if strings.HasPrefix(stat.Kernel, "ArrayCount") && stat.Outcome == "hit" {
					seen = true
				}
			}
			if !seen {
				t.Fatalf("missing sequence transform runtime stat: %#v", RuntimeKernelExecutionStats())
			}
		})
	}
}

func TestEvalSequenceCompositeReducerAddChainStats(t *testing.T) {
	ClearRuntimeKernelExecutionStats()
	t.Cleanup(ClearRuntimeKernelExecutionStats)

	assertEvalValue(t, "x:til 1024;r:17 rotate x;y:128 sublist reverse r;(+/y)+first y+last y", int64(108513))

	seenPipeline := false
	seenComposite := false
	seenSum := false
	for _, stat := range RuntimeKernelExecutionStats() {
		if stat.Outcome == "fallback" || stat.Outcome == "error" {
			t.Fatalf("unexpected sequence composite fallback/error: %#v all=%#v", stat, RuntimeKernelExecutionStats())
		}
		if stat.Kernel == "QScriptPipelinePlan" && stat.Shape == "script-pipeline/sequence-edge-reduce/sum-first-last-transform-chain/assignments" && stat.Outcome == "hit" {
			seenPipeline = true
		}
		if stat.Kernel == "SequenceTransformChainEdgeReduce" && stat.Shape == "vector-transform-chain/sum-first-last/rotate.reverse.sublist/i64" && stat.Outcome == "hit" {
			seenComposite = true
		}
		if stat.Kernel == "ArraySum" && stat.Shape == "vector-reduce/sum/i64" && stat.Outcome == "hit" {
			seenSum = true
		}
	}
	if !seenPipeline || !seenComposite {
		t.Fatalf("missing sequence composite runtime stats: pipeline=%v composite=%v sum=%v stats=%#v", seenPipeline, seenComposite, seenSum, RuntimeKernelExecutionStats())
	}
}

func TestEvalXcolsReordersTableAndDictionaryColumns(t *testing.T) {
	got, err := Eval("`price`sym xcols flip `sym`price`size!(`AAPL`MSFT;100 101;10 20)")
	if err != nil {
		t.Fatalf("Eval xcols frame returned error: %v", err)
	}
	frame, ok := got.(data.Frame)
	if !ok {
		t.Fatalf("xcols frame = %#v, want data.Frame", got)
	}
	if names := frame.Schema().Names(); !reflect.DeepEqual(names, []data.Symbol{"price", "sym", "size"}) {
		t.Fatalf("xcols frame columns = %#v", names)
	}

	broadcastValue, err := Eval("flip `sym`price!(`AAPL;100 101)")
	if err != nil {
		t.Fatalf("Eval flip atom broadcast returned error: %v", err)
	}
	broadcastFrame, ok := broadcastValue.(data.Frame)
	if !ok {
		t.Fatalf("flip atom broadcast = %#v, want data.Frame", broadcastValue)
	}
	if broadcastFrame.Len() != 2 {
		t.Fatalf("flip atom broadcast rows = %d, want 2", broadcastFrame.Len())
	}
	assertFrameValue(t, broadcastFrame, "sym", 0, data.Symbol("AAPL"))
	assertFrameValue(t, broadcastFrame, "sym", 1, data.Symbol("AAPL"))

	keyed, err := Eval("`price`sym xcols 1!flip `sym`price`size!(`AAPL`MSFT;100 101;10 20)")
	if err != nil {
		t.Fatalf("Eval xcols keyed returned error: %v", err)
	}
	keyedFrame, ok := keyed.(data.KeyedFrame)
	if !ok {
		t.Fatalf("xcols keyed = %#v, want data.KeyedFrame", keyed)
	}
	if keys := keyedFrame.Keys(); !reflect.DeepEqual(keys, []data.Symbol{"sym"}) {
		t.Fatalf("xcols keyed keys = %#v, want sym", keys)
	}
	if names := keyedFrame.Frame().Schema().Names(); !reflect.DeepEqual(names, []data.Symbol{"price", "sym", "size"}) {
		t.Fatalf("xcols keyed columns = %#v", names)
	}

	dictValue, err := Eval("`ask`bid xcols `bid`ask`last!100 101 100.5")
	if err != nil {
		t.Fatalf("Eval xcols dict returned error: %v", err)
	}
	dict, ok := dictValue.(EvalDict)
	if !ok {
		t.Fatalf("xcols dict = %#v, want EvalDict", dictValue)
	}
	if !reflect.DeepEqual(dict.Keys, []any{data.Symbol("ask"), data.Symbol("bid"), data.Symbol("last")}) {
		t.Fatalf("xcols dict keys = %#v", dict.Keys)
	}

	assertEvalErrorContains(t, "`missing xcols flip `sym`price!(`AAPL;100 101)", "does not exist")
	assertEvalErrorContains(t, "`sym`sym xcols flip `sym`price!(`AAPL;100 101)", "duplicated")
}

func TestEvalXKeyCreatesKeyedTables(t *testing.T) {
	got, err := Eval("`sym`venue xkey flip `sym`venue`price`size!(`AAPL`AAPL`MSFT;`XNYS`XNAS`XNYS;100 101 80;10 11 20)")
	if err != nil {
		t.Fatalf("Eval xkey returned error: %v", err)
	}
	keyed, ok := got.(data.KeyedFrame)
	if !ok {
		t.Fatalf("xkey = %#v, want data.KeyedFrame", got)
	}
	if keys := keyed.Keys(); !reflect.DeepEqual(keys, []data.Symbol{"sym", "venue"}) {
		t.Fatalf("xkey keys = %#v, want sym venue", keys)
	}
	if names := keyed.Frame().Schema().Names(); !reflect.DeepEqual(names, []data.Symbol{"sym", "venue", "price", "size"}) {
		t.Fatalf("xkey frame columns = %#v", names)
	}
	hit, err := keyed.LookupValueByKey(data.Symbol("AAPL"), data.Symbol("XNAS"))
	if err != nil {
		t.Fatalf("xkey lookup returned error: %v", err)
	}
	if hit.Len() != 1 {
		t.Fatalf("xkey lookup rows = %d, want 1", hit.Len())
	}
	if names := hit.Schema().Names(); !reflect.DeepEqual(names, []data.Symbol{"price", "size"}) {
		t.Fatalf("xkey lookup value columns = %#v, want price size", names)
	}
	assertFrameValue(t, hit, "price", 0, int64(101))

	rekeyed, err := Eval("`venue xkey (`sym xkey flip `sym`venue`price!(`AAPL`MSFT;`XNYS`XNAS;100 80))")
	if err != nil {
		t.Fatalf("Eval rekeyed xkey returned error: %v", err)
	}
	rekeyedFrame, ok := rekeyed.(data.KeyedFrame)
	if !ok {
		t.Fatalf("rekeyed xkey = %#v, want data.KeyedFrame", rekeyed)
	}
	if keys := rekeyedFrame.Keys(); !reflect.DeepEqual(keys, []data.Symbol{"venue"}) {
		t.Fatalf("rekeyed xkey keys = %#v, want venue", keys)
	}

	assertEvalErrorContains(t, "`missing xkey flip `sym`price!(`AAPL`MSFT;100 101)", "does not exist")
	assertEvalErrorContains(t, "`sym xkey `a`b!1 2", "expects a table")
}

func TestEvalXGroupAndUngroupTables(t *testing.T) {
	got, err := Eval("`sym xgroup flip `sym`price`size!(`AAPL`AAPL`MSFT;100 101 80;10 11 20)")
	if err != nil {
		t.Fatalf("Eval xgroup returned error: %v", err)
	}
	keyed, ok := got.(data.KeyedFrame)
	if !ok {
		t.Fatalf("xgroup = %#v, want data.KeyedFrame", got)
	}
	if keys := keyed.Keys(); !reflect.DeepEqual(keys, []data.Symbol{"sym"}) {
		t.Fatalf("xgroup keys = %#v, want sym", keys)
	}
	grouped := keyed.Frame()
	if grouped.Len() != 2 {
		t.Fatalf("xgroup rows = %d, want 2", grouped.Len())
	}
	assertFrameValue(t, grouped, "sym", 0, data.Symbol("AAPL"))
	priceCol, ok := grouped.Column("price")
	if !ok {
		t.Fatal("xgroup missing price column")
	}
	firstPricesValue, ok := priceCol.At(0)
	if !ok {
		t.Fatal("xgroup price row 0 out of range")
	}
	firstPrices, ok := firstPricesValue.(data.Array)
	if !ok {
		t.Fatalf("xgroup first prices = %#v, want data.Array", firstPricesValue)
	}
	if !reflect.DeepEqual(firstPrices.Values(), []any{int64(100), int64(101)}) {
		t.Fatalf("xgroup first prices = %#v", firstPrices.Values())
	}

	ungroupedValue, err := Eval("ungroup (`sym xgroup flip `sym`price`size!(`AAPL`AAPL`MSFT;100 101 80;10 11 20))")
	if err != nil {
		t.Fatalf("Eval ungroup xgroup returned error: %v", err)
	}
	ungrouped, ok := ungroupedValue.(data.Frame)
	if !ok {
		t.Fatalf("ungroup xgroup = %#v, want data.Frame", ungroupedValue)
	}
	if names := ungrouped.Schema().Names(); !reflect.DeepEqual(names, []data.Symbol{"sym", "price", "size"}) {
		t.Fatalf("ungroup columns = %#v", names)
	}
	assertFrameValue(t, ungrouped, "sym", 0, data.Symbol("AAPL"))
	assertFrameValue(t, ungrouped, "price", 1, int64(101))
	assertFrameValue(t, ungrouped, "sym", 2, data.Symbol("MSFT"))
	assertFrameValue(t, ungrouped, "size", 2, int64(20))

	multiKeyValue, err := Eval("`sym`venue xgroup flip `sym`venue`price!(`AAPL`AAPL`AAPL;`XNYS`XNYS`XNAS;100 101 102)")
	if err != nil {
		t.Fatalf("Eval multi-key xgroup returned error: %v", err)
	}
	multiKey, ok := multiKeyValue.(data.KeyedFrame)
	if !ok {
		t.Fatalf("multi-key xgroup = %#v, want data.KeyedFrame", multiKeyValue)
	}
	if keys := multiKey.Keys(); !reflect.DeepEqual(keys, []data.Symbol{"sym", "venue"}) {
		t.Fatalf("multi-key xgroup keys = %#v, want sym venue", keys)
	}

	assertEvalErrorContains(t, "ungroup 10 20", "expects a table")
}

func TestEvalCoreTableVerbCompositions(t *testing.T) {
	got, err := Eval("take 2 flip `sym`price`size!(`AAPL`MSFT`NVDA;100 101 102;10 20 30)")
	if err != nil {
		t.Fatalf("Eval(take frame) returned error: %v", err)
	}
	frame, ok := got.(data.Frame)
	if !ok {
		t.Fatalf("take frame = %#v, want data.Frame", got)
	}
	if frame.Len() != 2 {
		t.Fatalf("take frame len = %d, want 2", frame.Len())
	}
	assertFrameValue(t, frame, "sym", 0, data.Symbol("AAPL"))
	assertFrameValue(t, frame, "price", 1, int64(101))

	got, err = Eval("drop 1 flip `sym`price`size!(`AAPL`MSFT`NVDA;100 101 102;10 20 30)")
	if err != nil {
		t.Fatalf("Eval(drop frame word) returned error: %v", err)
	}
	frame, ok = got.(data.Frame)
	if !ok {
		t.Fatalf("drop frame word = %#v, want data.Frame", got)
	}
	if frame.Len() != 2 {
		t.Fatalf("drop frame word len = %d, want 2", frame.Len())
	}
	assertFrameValue(t, frame, "sym", 0, data.Symbol("MSFT"))
	assertFrameValue(t, frame, "size", 1, int64(30))

	assertEvalArray(t, "(10 20 30 40)[where 10 20 30 40>20]", data.KindI64, []any{int64(30), int64(40)})
	assertEvalArray(t, "cols take 1 flip `sym`price`size!(`AAPL`MSFT;100 101;10 20)", data.KindSymbol, []any{
		data.Symbol("sym"),
		data.Symbol("price"),
		data.Symbol("size"),
	})

	metaValue, err := Eval("meta take 1 flip `sym`price!(`AAPL`MSFT;100 101)")
	if err != nil {
		t.Fatalf("Eval(meta take frame) returned error: %v", err)
	}
	metaFrame, ok := metaValue.(data.Frame)
	if !ok {
		t.Fatalf("meta take frame = %#v, want data.Frame", metaValue)
	}
	if metaFrame.Len() != 2 {
		t.Fatalf("meta take frame len = %d, want 2", metaFrame.Len())
	}
	assertFrameValue(t, metaFrame, "c", 0, data.Symbol("sym"))
	assertFrameValue(t, metaFrame, "t", 1, "i64")

	grouped, err := Eval("group take 5 `AAPL`MSFT`AAPL")
	if err != nil {
		t.Fatalf("Eval(group take vector) returned error: %v", err)
	}
	groupDict, ok := grouped.(EvalDict)
	if !ok {
		t.Fatalf("group take vector = %#v, want EvalDict", grouped)
	}
	if !reflect.DeepEqual(groupDict.Keys, []any{data.Symbol("AAPL"), data.Symbol("MSFT")}) {
		t.Fatalf("group take keys = %#v", groupDict.Keys)
	}
	assertDictIndexArray(t, groupDict, 0, []any{int64(0), int64(2), int64(3)})
	assertDictIndexArray(t, groupDict, 1, []any{int64(1), int64(4)})

	got, err = Eval("take 3 ungroup (`sym xgroup flip `sym`price`size!(`AAPL`AAPL`MSFT;100 101 80;10 11 20))")
	if err != nil {
		t.Fatalf("Eval(take ungroup xgroup) returned error: %v", err)
	}
	frame, ok = got.(data.Frame)
	if !ok {
		t.Fatalf("take ungroup xgroup = %#v, want data.Frame", got)
	}
	if names := frame.Schema().Names(); !reflect.DeepEqual(names, []data.Symbol{"sym", "price", "size"}) {
		t.Fatalf("take ungroup columns = %#v", names)
	}
	assertFrameValue(t, frame, "sym", 0, data.Symbol("AAPL"))
	assertFrameValue(t, frame, "price", 1, int64(101))
	assertFrameValue(t, frame, "sym", 2, data.Symbol("MSFT"))
}

func TestEvalTakeDropWhereGroupFlipMetaColsKeyedCompositions(t *testing.T) {
	got, err := Eval("take 4 (`sym xkey flip `sym`price`size!(`AAPL`MSFT`NVDA;100 101 102;10 20 30))")
	if err != nil {
		t.Fatalf("Eval(take keyed frame) returned error: %v", err)
	}
	keyed, ok := got.(data.KeyedFrame)
	if !ok {
		t.Fatalf("take keyed frame = %#v, want data.KeyedFrame", got)
	}
	if keys := keyed.Keys(); !reflect.DeepEqual(keys, []data.Symbol{"sym"}) {
		t.Fatalf("take keyed keys = %#v, want sym", keys)
	}
	frame := keyed.Frame()
	if frame.Len() != 4 {
		t.Fatalf("take keyed len = %d, want 4", frame.Len())
	}
	assertFrameValue(t, frame, "sym", 0, data.Symbol("AAPL"))
	assertFrameValue(t, frame, "sym", 3, data.Symbol("AAPL"))
	assertFrameValue(t, frame, "price", 3, int64(100))

	got, err = Eval("drop -1 (`sym xkey flip `sym`price`size!(`AAPL`MSFT`NVDA;100 101 102;10 20 30))")
	if err != nil {
		t.Fatalf("Eval(drop keyed frame) returned error: %v", err)
	}
	keyed, ok = got.(data.KeyedFrame)
	if !ok {
		t.Fatalf("drop keyed frame = %#v, want data.KeyedFrame", got)
	}
	frame = keyed.Frame()
	if frame.Len() != 2 {
		t.Fatalf("drop keyed len = %d, want 2", frame.Len())
	}
	assertFrameValue(t, frame, "sym", 0, data.Symbol("AAPL"))
	assertFrameValue(t, frame, "sym", 1, data.Symbol("MSFT"))

	assertEvalArray(t, "(10 20 30 40 50)[where 0 2 1 0 2]", data.KindI64, []any{
		int64(20),
		int64(20),
		int64(30),
		int64(50),
		int64(50),
	})
	assertEvalArray(t, "cols flip `z`a`m!(1 2;3 4;5 6)", data.KindSymbol, []any{
		data.Symbol("z"),
		data.Symbol("a"),
		data.Symbol("m"),
	})
	assertEvalArray(t, "cols (`size`sym xcols take 2 flip `sym`price`size!(`AAPL`MSFT`NVDA;100 101 102;10 20 30))", data.KindSymbol, []any{
		data.Symbol("size"),
		data.Symbol("sym"),
		data.Symbol("price"),
	})

	metaValue, err := Eval("meta (`size`sym xcols take 2 flip `sym`price`size!(`AAPL`MSFT`NVDA;100 101 102;10 20 30))")
	if err != nil {
		t.Fatalf("Eval(meta xcols take frame) returned error: %v", err)
	}
	metaFrame, ok := metaValue.(data.Frame)
	if !ok {
		t.Fatalf("meta xcols take frame = %#v, want data.Frame", metaValue)
	}
	assertFrameValue(t, metaFrame, "c", 0, data.Symbol("size"))
	assertFrameValue(t, metaFrame, "t", 0, "i64")
	assertFrameValue(t, metaFrame, "c", 1, data.Symbol("sym"))
	assertFrameValue(t, metaFrame, "t", 1, "symbol")

	grouped, err := Eval("group (where 0 2 1 0 2)")
	if err != nil {
		t.Fatalf("Eval(group where count vector) returned error: %v", err)
	}
	groupDict, ok := grouped.(EvalDict)
	if !ok {
		t.Fatalf("group where count vector = %#v, want EvalDict", grouped)
	}
	if !reflect.DeepEqual(groupDict.Keys, []any{int64(1), int64(2), int64(4)}) {
		t.Fatalf("group where keys = %#v", groupDict.Keys)
	}
	assertDictIndexArray(t, groupDict, 0, []any{int64(0), int64(1)})
	assertDictIndexArray(t, groupDict, 1, []any{int64(2)})
	assertDictIndexArray(t, groupDict, 2, []any{int64(3), int64(4)})

	got, err = Eval("ungroup take 1 (`sym xgroup flip `sym`price`size!(`AAPL`AAPL`MSFT;100 101 80;10 11 20))")
	if err != nil {
		t.Fatalf("Eval(ungroup take keyed group) returned error: %v", err)
	}
	frame, ok = got.(data.Frame)
	if !ok {
		t.Fatalf("ungroup take keyed group = %#v, want data.Frame", got)
	}
	if frame.Len() != 2 {
		t.Fatalf("ungroup take keyed group len = %d, want 2", frame.Len())
	}
	assertFrameValue(t, frame, "sym", 0, data.Symbol("AAPL"))
	assertFrameValue(t, frame, "price", 0, int64(100))
	assertFrameValue(t, frame, "price", 1, int64(101))
}

func TestEvalXascXdescSortTables(t *testing.T) {
	got, err := Eval("`sym`price xasc flip `sym`price`size!(`MSFT`AAPL`AAPL`MSFT;80 101 100 90;20 30 10 40)")
	if err != nil {
		t.Fatalf("Eval xasc returned error: %v", err)
	}
	frame, ok := got.(data.Frame)
	if !ok {
		t.Fatalf("xasc frame = %#v, want data.Frame", got)
	}
	assertFrameValue(t, frame, "sym", 0, data.Symbol("AAPL"))
	assertFrameValue(t, frame, "price", 0, int64(100))
	assertFrameValue(t, frame, "size", 0, int64(10))
	assertFrameValue(t, frame, "price", 1, int64(101))
	assertFrameValue(t, frame, "sym", 3, data.Symbol("MSFT"))
	assertFrameValue(t, frame, "price", 3, int64(90))

	desc, err := Eval("`price xdesc flip `sym`price!(`MSFT`AAPL`NVDA;80 101 210)")
	if err != nil {
		t.Fatalf("Eval xdesc returned error: %v", err)
	}
	descFrame, ok := desc.(data.Frame)
	if !ok {
		t.Fatalf("xdesc frame = %#v, want data.Frame", desc)
	}
	assertFrameValue(t, descFrame, "sym", 0, data.Symbol("NVDA"))
	assertFrameValue(t, descFrame, "price", 2, int64(80))

	keyed, err := Eval("`price xasc 1!flip `sym`price`size!(`MSFT`AAPL`AAPL;80 101 100;20 30 10)")
	if err != nil {
		t.Fatalf("Eval keyed xasc returned error: %v", err)
	}
	keyedFrame, ok := keyed.(data.KeyedFrame)
	if !ok {
		t.Fatalf("keyed xasc = %#v, want data.KeyedFrame", keyed)
	}
	if keys := keyedFrame.Keys(); !reflect.DeepEqual(keys, []data.Symbol{"sym"}) {
		t.Fatalf("keyed xasc keys = %#v, want sym", keys)
	}
	assertFrameValue(t, keyedFrame.Frame(), "price", 0, int64(80))

	assertEvalErrorContains(t, "`missing xasc flip `sym`price!(`AAPL`MSFT;100 101)", "does not exist")
	assertEvalErrorContains(t, "`sym xasc `a`b!1 2", "expects a table")
}

func TestEvalXgroupKeysTables(t *testing.T) {
	got, err := Eval("`sym xgroup flip `sym`price`size!(`AAPL`MSFT`AAPL;100 101 102;10 20 30)")
	if err != nil {
		t.Fatalf("Eval xgroup returned error: %v", err)
	}
	keyed, ok := got.(data.KeyedFrame)
	if !ok {
		t.Fatalf("xgroup = %#v, want data.KeyedFrame", got)
	}
	if keys := keyed.Keys(); !reflect.DeepEqual(keys, []data.Symbol{"sym"}) {
		t.Fatalf("xgroup keys = %#v, want sym", keys)
	}
	hit, err := keyed.LookupByKey(data.Symbol("AAPL"))
	if err != nil {
		t.Fatalf("LookupByKey returned error: %v", err)
	}
	if hit.Len() != 1 {
		t.Fatalf("AAPL hit len = %d, want 1", hit.Len())
	}
	priceCol, ok := hit.Column("price")
	if !ok {
		t.Fatal("AAPL hit missing price column")
	}
	pricesValue, ok := priceCol.At(0)
	if !ok {
		t.Fatal("AAPL price row 0 out of range")
	}
	prices, ok := pricesValue.(data.Array)
	if !ok {
		t.Fatalf("AAPL grouped price = %#v, want data.Array", pricesValue)
	}
	if !reflect.DeepEqual(prices.Values(), []any{int64(100), int64(102)}) {
		t.Fatalf("AAPL grouped prices = %#v", prices.Values())
	}

	rekeyed, err := Eval("`venue xgroup flip `sym`venue`price!(`AAPL`AAPL`MSFT;`XNYS`XNAS`XNYS;100 101 80)")
	if err != nil {
		t.Fatalf("Eval rekeyed xgroup returned error: %v", err)
	}
	rekeyedFrame, ok := rekeyed.(data.KeyedFrame)
	if !ok {
		t.Fatalf("rekeyed xgroup = %#v, want data.KeyedFrame", rekeyed)
	}
	if keys := rekeyedFrame.Keys(); !reflect.DeepEqual(keys, []data.Symbol{"venue"}) {
		t.Fatalf("rekeyed xgroup keys = %#v, want venue", keys)
	}

	assertEvalErrorContains(t, "`missing xgroup flip `sym`price!(`AAPL`MSFT;100 101)", "does not exist")
	assertEvalErrorContains(t, "`sym xgroup `a`b!1 2", "expects a table")
}

func TestEvalTemporalAtomsVectorsAndTypedNulls(t *testing.T) {
	date0, err := parseQTemporal("date", "2026.06.06")
	if err != nil {
		t.Fatal(err)
	}
	date1, err := parseQTemporal("date", "2026.06.07")
	if err != nil {
		t.Fatal(err)
	}
	second0, err := parseQTemporal("second", "09:30:00")
	if err != nil {
		t.Fatal(err)
	}
	time1, err := parseQTemporal("time", "09:31:00.250")
	if err != nil {
		t.Fatal(err)
	}
	ts0, err := parseQTemporal("timestamp", "2026.06.06D09:30:00")
	if err != nil {
		t.Fatal(err)
	}

	assertEvalValue(t, "2026.06.06", date0)
	assertEvalValue(t, "09:30:00", second0)
	assertEvalValue(t, "mins 2026.06.06", date0)
	assertEvalValue(t, "maxs 2026.06.06D09:30:00", ts0)
	assertEvalValue(t, "mins 0Np", data.NullForKind(data.KindTimestamp))
	assertEvalArray(t, "2026.06.06 0Nd 2026.06.07", data.KindDate, []any{date0, data.NullValue, date1})
	assertEvalArray(t, "09:30:00 0Nv", data.KindSecond, []any{second0, data.NullValue})
	assertEvalArray(t, "0Nt 09:31:00.250", data.KindTime, []any{data.NullValue, time1})
	assertEvalArray(t, "2026.06.06D09:30:00 0Np", data.KindTimestamp, []any{ts0, data.NullValue})
	assertEvalArray(t, "mins 2026.06.07 0Nd 2026.06.06", data.KindDate, []any{date1, date1, date0})
	assertEvalArray(t, "maxs 2026.06.06 0Nd 2026.06.07", data.KindDate, []any{date0, date0, date1})
}

func TestEvalMovingCountWindow(t *testing.T) {
	assertEvalArray(t, "3 mcount 10 0N 30 40", data.KindI64, []any{int64(1), int64(1), int64(2), int64(2)})
	assertEvalArray(t, "2 mcount 2026.06.06 0Nd 2026.06.07", data.KindI64, []any{int64(1), int64(1), int64(1)})
	assertEvalArray(t, "2 mcount 2026.06.06D09:30:00 0Np 2026.06.06D09:31:00", data.KindI64, []any{int64(1), int64(1), int64(1)})
	assertEvalValue(t, "3 mcount 0N", int64(0))
	assertEvalValue(t, "3 mcount 42", int64(1))
	assertEvalErrorContains(t, "0 mcount 10 20", "mcount width must be a positive integer")
}

func TestEvalMovingExtremaWindow(t *testing.T) {
	ts0, err := parseQTemporal("timestamp", "2026.06.06D09:30:00")
	if err != nil {
		t.Fatal(err)
	}
	ts1, err := parseQTemporal("timestamp", "2026.06.06D09:31:00")
	if err != nil {
		t.Fatal(err)
	}
	ts2, err := parseQTemporal("timestamp", "2026.06.06D09:29:00")
	if err != nil {
		t.Fatal(err)
	}

	assertEvalArray(t, "3 mmin 30 0N 10 20", data.KindI64, []any{int64(30), int64(30), int64(10), int64(10)})
	assertEvalArray(t, "3 mmax 30 0N 10 20", data.KindI64, []any{int64(30), int64(30), int64(30), int64(20)})
	assertEvalArray(t, "2 mmin 2026.06.06D09:30:00 0Np 2026.06.06D09:29:00", data.KindTimestamp, []any{ts0, ts0, ts2})
	assertEvalArray(t, "2 mmax 2026.06.06D09:30:00 0Np 2026.06.06D09:31:00", data.KindTimestamp, []any{ts0, ts0, ts1})
	assertEvalArray(t, "2 mmax 0Np 0Np", data.KindTimestamp, []any{data.NullValue, data.NullValue})
	assertEvalValue(t, "3 mmin 42", int64(42))
	assertEvalErrorContains(t, "0 mmax 10 20", "mmax width must be a positive integer")
}

func TestEvalMovingWindowOverSum(t *testing.T) {
	assertEvalValue(t, "+/3 msum 10 20 30 40", int64(190))
	assertEvalValue(t, "+/3 mavg 10 20 30 40", 75.0)
	assertEvalValue(t, "+/2 msum 1.5 2.5 3.5", 11.5)
	assertEvalValue(t, "+/3 mcount 10 20 30 40", int64(9))
	assertEvalValue(t, "+/2 mmin 3 1 4 2", int64(7))
	assertEvalValue(t, "+/2 mmax 3 1 4 2", int64(14))
	assertEvalValue(t, "+/3 mcount 10 0N 30 40", int64(6))
	assertEvalValue(t, "+/3 mmin 30 0N 10 20", int64(80))
	assertEvalErrorContains(t, "+/0 msum 10 20", "msum width must be a positive integer")
}

func TestEvalFirstLastDyadicAvoidsWholeVector(t *testing.T) {
	assertEvalValue(t, "x:til 5;first x+last x", int64(4))
	assertEvalValue(t, "x:til 5;last x+first x", int64(4))
	assertEvalValue(t, "first (10 20 30)+1", int64(11))
	assertEvalValue(t, "last 1+10 20 30", int64(31))
	assertEvalValue(t, "x:til 5;(+/x)+first x+last x", int64(14))
	assertEvalValue(t, "x:til 5;(+/x)+((sum x) plus 10)+first x", int64(30))
	assertEvalValue(t, "first (0#10 20)+1", data.NullValue)
	assertEvalErrorContains(t, "first 1 2 3+10 20", "vector length mismatch")
}

// TestEvalSignalAndUntrappedErrors feeds the error-route differential corpus
// (extracted mechanically from assertEvalErrorContains literals): signal
// errors and untrapped trap-adjacent failures must agree across the compiled
// and string routes and stay stable on warm repeats.
func TestEvalSignalAndUntrappedErrors(t *testing.T) {
	assertEvalErrorContains(t, "'\"err\"", "err")
	assertEvalErrorContains(t, "'`oops", "oops")
	assertEvalErrorContains(t, "e:\"bad input\";'e", "bad input")
	assertEvalErrorContains(t, "e:42;'e", "signal expects a string or symbol")
	assertEvalErrorContains(t, "f:{$[x<0;'\"neg\";x*2]};f[-1]", "neg")
	assertEvalErrorContains(t, "@[{'x};\"deep\";{'x}]", "deep")
	assertEvalErrorContains(t, "@[10 20 30;1;2]", "amend function is not callable")
	assertEvalErrorContains(t, "@[{x};1;2;3]", "amend function is not callable")
}

// TestEvalFunctionalFormCorpus seeds the mechanically-extracted corpora
// (parser fuzz seeds, error-route differential) with functional qSQL forms;
// behavioral coverage lives in functional_query_test.go.
func TestEvalFunctionalFormCorpus(t *testing.T) {
	assertEvalValue(t, "t:([] a:1 2 3;b:10 20 30); r:?[t;enlist (>;`a;1);0b;()]; count r", int64(2))
	assertEvalValue(t, "?[([] b:10 20 30);();();(sum;`b)]", int64(60))
	assertEvalValue(t, "t:([] a:1 2 3); ![`t;();0b;(enlist `a)!enlist (*;`a;10)]; last t[`a]", int64(30))
	assertEvalValue(t, "count ![([] a:1 2 3;b:10 20 30);();0b;enlist `b]", int64(3))
	assertEvalErrorContains(t, "?[1;2;3;4]", "expects a table or table name")
	assertEvalErrorContains(t, "![`nosuchtable;();0b;enlist `a]", "is not bound to a table")
	assertEvalErrorContains(t, "?[([] a:1 2);1b;0b;()]", "constraint spec must be a list of parse trees")
	assertEvalErrorContains(t, "![([] a:1 2;b:3 4);enlist (>;`a;1);0b;enlist `b]", "cannot specify both row constraints and columns")
}

func TestFillsPropagatesLastNonNullValue(t *testing.T) {
	got, err := fills(data.NewColumn("x", []any{nil, int64(10), nil, nil, int64(20)}).Data)
	if err != nil {
		t.Fatalf("fills returned error: %v", err)
	}
	array, ok := got.(data.Array)
	if !ok {
		t.Fatalf("fills = %#v, want data.Array", got)
	}
	if array.Kind() != data.KindI64 {
		t.Fatalf("fills kind = %s, want %s", array.Kind(), data.KindI64)
	}
	want := []any{data.NullValue, int64(10), int64(10), int64(10), int64(20)}
	if values := array.Values(); !reflect.DeepEqual(values, want) {
		t.Fatalf("fills values = %#v, want %#v", values, want)
	}
}

func assertEvalValue(t *testing.T, src string, want any) {
	t.Helper()
	got, err := Eval(src)
	if err != nil {
		t.Fatalf("Eval(%q) returned error: %v", src, err)
	}
	if got != want {
		t.Fatalf("Eval(%q) = %#v (%T), want %#v (%T)", src, got, got, want, want)
	}
}

func assertEvalParsedValue(t *testing.T, src string, want any) {
	t.Helper()
	got, ok, err := NewEvalState(nil).evalParsedValueExpr(src)
	if err != nil {
		t.Fatalf("evalParsedValueExpr(%q) returned error: %v", src, err)
	}
	if !ok {
		t.Fatalf("evalParsedValueExpr(%q) did not use parsed value AST", src)
	}
	if got != want {
		t.Fatalf("evalParsedValueExpr(%q) = %#v, want %#v", src, got, want)
	}
}

func assertStateEvalValue(t *testing.T, state *EvalState, src string, want any) {
	t.Helper()
	got, err := state.Eval(src)
	if err != nil {
		t.Fatalf("state.Eval(%q) returned error: %v", src, err)
	}
	if got != want {
		t.Fatalf("state.Eval(%q) = %#v, want %#v", src, got, want)
	}
}

func assertEvalParsedArray(t *testing.T, src string, kind data.Kind, want []any) {
	t.Helper()
	got, ok, err := NewEvalState(nil).evalParsedValueExpr(src)
	if err != nil {
		t.Fatalf("evalParsedValueExpr(%q) returned error: %v", src, err)
	}
	if !ok {
		t.Fatalf("evalParsedValueExpr(%q) did not use parsed value AST", src)
	}
	array, ok := got.(data.Array)
	if !ok {
		t.Fatalf("evalParsedValueExpr(%q) = %#v, want data.Array", src, got)
	}
	if array.Kind() != kind {
		t.Fatalf("evalParsedValueExpr(%q) kind = %s, want %s", src, array.Kind(), kind)
	}
	if values := array.Values(); !reflect.DeepEqual(values, want) {
		t.Fatalf("evalParsedValueExpr(%q) values = %#v, want %#v", src, values, want)
	}
}

func assertStateEvalArray(t *testing.T, state *EvalState, src string, kind data.Kind, want []any) {
	t.Helper()
	got, err := state.Eval(src)
	if err != nil {
		t.Fatalf("state.Eval(%q) returned error: %v", src, err)
	}
	array, ok := got.(data.Array)
	if !ok {
		t.Fatalf("state.Eval(%q) = %#v, want data.Array", src, got)
	}
	if array.Kind() != kind {
		t.Fatalf("state.Eval(%q) kind = %s, want %s", src, array.Kind(), kind)
	}
	if values := array.Values(); !reflect.DeepEqual(values, want) {
		t.Fatalf("state.Eval(%q) values = %#v, want %#v", src, values, want)
	}
}

func assertEvalArray(t *testing.T, src string, kind data.Kind, want []any) {
	t.Helper()
	got, err := Eval(src)
	if err != nil {
		t.Fatalf("Eval(%q) returned error: %v", src, err)
	}
	array, ok := got.(data.Array)
	if !ok {
		t.Fatalf("Eval(%q) = %#v, want data.Array", src, got)
	}
	if array.Kind() != kind {
		t.Fatalf("Eval(%q) kind = %s, want %s", src, array.Kind(), kind)
	}
	if values := normalizeNestedArrayValues(array.Values()); !reflect.DeepEqual(values, normalizeNestedArrayValues(want)) {
		t.Fatalf("Eval(%q) values = %#v, want %#v", src, values, want)
	}
}

func normalizeNestedArrayValues(values []any) []any {
	out := make([]any, len(values))
	for i, value := range values {
		if array, ok := value.(data.Array); ok {
			out[i] = normalizeNestedArrayValues(array.Values())
			continue
		}
		out[i] = value
	}
	return out
}

func assertEvalErrorContains(t *testing.T, src string, want string) {
	t.Helper()
	_, err := Eval(src)
	if err == nil {
		t.Fatalf("Eval(%q) succeeded, want error containing %q", src, want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("Eval(%q) error = %q, want substring %q", src, err.Error(), want)
	}
}

func assertNestedArray(t *testing.T, outer data.Array, row int, kind data.Kind, want []any) {
	t.Helper()
	item, ok := outer.At(row)
	if !ok {
		t.Fatalf("missing nested row %d", row)
	}
	array, ok := item.(data.Array)
	if !ok {
		t.Fatalf("nested row %d = %#v, want data.Array", row, item)
	}
	if array.Kind() != kind {
		t.Fatalf("nested row %d kind = %s, want %s", row, array.Kind(), kind)
	}
	if values := array.Values(); !reflect.DeepEqual(values, want) {
		t.Fatalf("nested row %d values = %#v, want %#v", row, values, want)
	}
}

func assertEvalDict(t *testing.T, src string, keys []data.Symbol, values []any) {
	t.Helper()
	anyKeys := make([]any, len(keys))
	for i, key := range keys {
		anyKeys[i] = key
	}
	assertEvalDictAny(t, src, anyKeys, values)
}

func assertEvalDictAny(t *testing.T, src string, keys []any, values []any) {
	t.Helper()
	got, err := Eval(src)
	if err != nil {
		t.Fatalf("Eval(%q) returned error: %v", src, err)
	}
	dict, ok := got.(EvalDict)
	if !ok {
		t.Fatalf("Eval(%q) = %#v, want EvalDict", src, got)
	}
	if !reflect.DeepEqual(dict.Keys, keys) {
		t.Fatalf("Eval(%q) keys = %#v, want %#v", src, dict.Keys, keys)
	}
	if !reflect.DeepEqual(dict.Values, values) {
		t.Fatalf("Eval(%q) values = %#v, want %#v", src, dict.Values, values)
	}
}

func assertEvalGroupedIndexes(t *testing.T, src string, keys []any, indexes [][]any) {
	t.Helper()
	values := make([]any, len(indexes))
	for i, rowIndexes := range indexes {
		ints := make([]int64, len(rowIndexes))
		for j, value := range rowIndexes {
			ints[j] = value.(int64)
		}
		values[i] = data.NewI64(ints)
	}
	got, err := Eval(src)
	if err != nil {
		t.Fatalf("Eval(%q) returned error: %v", src, err)
	}
	dict, ok := got.(EvalDict)
	if !ok {
		t.Fatalf("Eval(%q) = %#v, want EvalDict", src, got)
	}
	if !reflect.DeepEqual(dict.Keys, keys) {
		t.Fatalf("Eval(%q) keys = %#v, want %#v", src, dict.Keys, keys)
	}
	if len(dict.Values) != len(values) {
		t.Fatalf("Eval(%q) values len = %d, want %d", src, len(dict.Values), len(values))
	}
	for i := range values {
		gotArray, ok := dict.Values[i].(data.Array)
		if !ok {
			t.Fatalf("Eval(%q) value %d = %#v, want data.Array", src, i, dict.Values[i])
		}
		wantArray := values[i].(data.Array)
		if !reflect.DeepEqual(gotArray.Values(), wantArray.Values()) {
			t.Fatalf("Eval(%q) value %d = %#v, want %#v", src, i, gotArray.Values(), wantArray.Values())
		}
	}
}

func assertDictIndexArray(t *testing.T, dict EvalDict, index int, want []any) {
	t.Helper()
	if index < 0 || index >= len(dict.Values) {
		t.Fatalf("dict value index %d out of range", index)
	}
	array, ok := dict.Values[index].(data.Array)
	if !ok {
		t.Fatalf("dict value %d = %#v, want data.Array", index, dict.Values[index])
	}
	if array.Kind() != data.KindI64 {
		t.Fatalf("dict value %d kind = %s, want %s", index, array.Kind(), data.KindI64)
	}
	if values := array.Values(); !reflect.DeepEqual(values, want) {
		t.Fatalf("dict value %d values = %#v, want %#v", index, values, want)
	}
}

func assertFrameValue(t *testing.T, frame data.Frame, name data.Symbol, row int, want any) {
	t.Helper()
	col, ok := frame.Column(name)
	if !ok {
		t.Fatalf("missing column %q", name)
	}
	got, ok := col.At(row)
	if !ok {
		t.Fatalf("missing row %d in column %q", row, name)
	}
	if got != want {
		t.Fatalf("%s[%d] = %#v, want %#v", name, row, got, want)
	}
}
