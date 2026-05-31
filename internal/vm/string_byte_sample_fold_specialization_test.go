package vm

import (
	"testing"

	"github.com/never-labs/gscript/internal/runtime"
)

func TestStringByteSampleFoldRuntimeSpecialization(t *testing.T) {
	stats := runtime.EnableRuntimePathStats()
	defer runtime.DisableRuntimePathStats()

	globals := compileAndRun(t, `
MOD := 1000000007
func mix(h, v) {
    return (h * 131 + v) % MOD
}
func checksumString(h, s) {
    h = mix(h, #s)
    step := math.floor(#s / 17) + 1
    for i := 1; i <= #s; i += step {
        h = mix(h, string.byte(s, i))
    }
    if #s > 0 {
        h = mix(h, string.byte(s, #s))
    }
    return h
}
result := checksumString(17, "abcdef0123456789abcdef0123456789")
`)
	expectGlobalInt(t, globals, "result", 290590338)
	if got := runtimeRuntimeSpecializationHitCount(stats, RuntimeSpecializationRouteCallSiteValue, "string_byte_sample_fold"); got == 0 {
		t.Fatalf("string_byte_sample_fold hit count = %d, want > 0", got)
	}
}

func TestStringByteSampleFoldRejectsNameOnly(t *testing.T) {
	top := compileProto(t, `
func checksumString(h, s) {
    return h + #s
}
`)
	child := findTestProtoByName(top, "checksumString")
	if child == nil {
		t.Fatal("missing checksumString proto")
	}
	if isStringByteSampleFoldProto(child) {
		t.Fatal("string_byte_sample_fold should reject name-only matches")
	}
}

func TestStringByteSampleFoldUsesRuntimeFoldShape(t *testing.T) {
	stats := runtime.EnableRuntimePathStats()
	defer runtime.DisableRuntimePathStats()

	globals := compileAndRun(t, `
PRIME := 1009
func mix(h, v) {
    return (h * 257 + v) % PRIME
}
func checksumString(h, s) {
    h = mix(h, #s)
    step := math.floor(#s / 3) + 1
    for i := 1; i <= #s; i += step {
        h = mix(h, string.byte(s, i))
    }
    if #s > 0 {
        h = mix(h, string.byte(s, #s))
    }
    return h
}
result := checksumString(5, "abcdef")
`)
	expectGlobalInt(t, globals, "result", 164)
	if got := runtimeRuntimeSpecializationHitCount(stats, RuntimeSpecializationRouteCallSiteValue, "string_byte_sample_fold"); got == 0 {
		t.Fatalf("string_byte_sample_fold hit count = %d, want > 0", got)
	}
}
