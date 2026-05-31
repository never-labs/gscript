package vm

import (
	"testing"

	"github.com/never-labs/gscript/internal/runtime"
)

func TestCoroutineYieldSumLoopRuntimeSpecialization(t *testing.T) {
	stats := runtime.EnableRuntimePathStats()
	defer runtime.DisableRuntimePathStats()

	globals := compileAndRun(t, `
func consume_range(n) {
    co := coroutine.create(func() {
        for i := 1; i <= n; i++ {
            coroutine.yield(i)
        }
        return n
    })
    sum := 0
    for i := 1; i <= n; i++ {
        ok, val := coroutine.resume(co)
        sum = sum + val
    }
    return sum
}
result := consume_range(10)
`)
	expectGlobalInt(t, globals, "result", 55)
	if got := runtimeRuntimeSpecializationHitCount(stats, RuntimeSpecializationRouteCallSiteValue, "coroutine_yield_sum_loop"); got == 0 {
		t.Fatalf("coroutine_yield_sum_loop hit count = %d, want > 0", got)
	}
}

func TestCoroutineCreateResumeAffineSumRuntimeSpecialization(t *testing.T) {
	stats := runtime.EnableRuntimePathStats()
	defer runtime.DisableRuntimePathStats()

	globals := compileAndRun(t, `
func create_resume_sum(n) {
    total := 0
    for i := 1; i <= n; i++ {
        co := coroutine.create(func() {
            return i * 2
        })
        ok, val := coroutine.resume(co)
        total = total + val
    }
    return total
}
result := create_resume_sum(10)
`)
	expectGlobalInt(t, globals, "result", 110)
	if got := runtimeRuntimeSpecializationHitCount(stats, RuntimeSpecializationRouteCallSiteValue, "coroutine_create_resume_affine_sum"); got == 0 {
		t.Fatalf("coroutine_create_resume_affine_sum hit count = %d, want > 0", got)
	}
}

func TestCoroutineSumSpecializationsRejectNameOnly(t *testing.T) {
	top := compileProto(t, `
func consume_range(n) {
    return n
}
func create_resume_sum(n) {
    return n
}
`)
	for _, name := range []string{"consume_range", "create_resume_sum"} {
		child := findTestProtoByName(top, name)
		if child == nil {
			t.Fatalf("missing proto %s", name)
		}
		if isCoroutineYieldSumLoopProto(child) || isCoroutineCreateResumeAffineSumProto(child) {
			t.Fatalf("coroutine sum specializations should reject name-only match for %s", name)
		}
	}
}
