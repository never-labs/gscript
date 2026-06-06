package vm

import (
	"testing"

	"github.com/never-labs/leia/internal/runtime"
)

func TestCallableLenPairsDriverRuntimeSpecialization(t *testing.T) {
	stats := runtime.EnableRuntimePathStats()
	defer runtime.DisableRuntimePathStats()

	globals := compileAndRun(t, `
M := 1000003
G := 3
R := 4
EVERY := 2

func maker(seed) {
    mt := {}
    mt.__call = func(t, x, y) {
        t.n = t.n + 1
        return (x * 5 + y * 11 + t.offset + t.n * 13) % M
    }
    return setmetatable({offset: seed * 4 + 9, n: 0}, mt)
}

func proxy_factory(seed, n) {
    backing := {}
    for i := 1; i <= n; i++ {
        backing[i] = seed + i * 3 + 1
    }
    mt := {}
    mt.__len = func(_) { return n }
    mt.__pairs = func(obj) {
        i := 0
        return func(_, last) {
            i = i + 1
            if i <= n {
                return i, backing[i]
            }
        }, obj, nil
    }
    return setmetatable({}, mt)
}

func driver(groups, reps) {
    items := {}
    for b := 1; b <= groups; b++ {
        items[b] = {
            callable: maker(b),
            proxy: proxy_factory(b, 6),
        }
    }
    checksum := 0
    for b := 1; b <= groups; b++ {
        item := items[b]
        for i := 1; i <= reps; i++ {
            checksum = (checksum + item.callable(i + b, i % 7) + #item.proxy * 17) % M
            if i % EVERY == 0 {
                iter, state, last := getmetatable(item.proxy).__pairs(item.proxy)
                _ := iter
                _ := state
                _ := last
                checksum = (checksum + 99) % M
            }
        }
    }
    return checksum
}

result := driver(G, R)
`)
	expectGlobalInt(t, globals, "result", 3012)
	if got := runtimeRuntimeSpecializationHitCount(stats, RuntimeSpecializationRouteCallSiteValue, "callable_len_pairs_driver"); got == 0 {
		t.Fatalf("callable_len_pairs_driver hit count = %d, want > 0", got)
	}
}

func TestCallableLenPairsDriverRejectsNameOnly(t *testing.T) {
	top := compileProto(t, `
func driver(groups, reps) {
    return groups + reps
}
`)
	child := findTestProtoByName(top, "driver")
	if child == nil {
		t.Fatal("missing driver proto")
	}
	if isCallableLenPairsDriverProto(child) {
		t.Fatal("callable_len_pairs_driver should reject name-only matches")
	}
}
