package vm

import (
	"testing"

	"github.com/gscript/gscript/internal/runtime"
)

func TestStringSampleChecksumRuntimeSpecialization(t *testing.T) {
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
	if got := runtimeRuntimeSpecializationHitCount(stats, RuntimeSpecializationRouteCallSiteValue, "string_sample_checksum"); got == 0 {
		t.Fatalf("string_sample_checksum hit count = %d, want > 0", got)
	}
}

func TestStringSampleChecksumRejectsNameOnly(t *testing.T) {
	top := compileProto(t, `
func checksumString(h, s) {
    return h + #s
}
`)
	child := findTestProtoByName(top, "checksumString")
	if child == nil {
		t.Fatal("missing checksumString proto")
	}
	if isStringSampleChecksumProto(child) {
		t.Fatal("string_sample_checksum should reject name-only matches")
	}
}
