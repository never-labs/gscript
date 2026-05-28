// Data-oriented hot benchmark: masked SoA aggregate kernels.

N := 65536
REPS := 1200

func make_value(n) {
    t := {}
    for i := 1; i <= n; i++ {
        t[i] = 1.0 + (i % 257) * 0.125
    }
    return array.f64(t)
}

func make_weight(n) {
    t := {}
    for i := 1; i <= n; i++ {
        t[i] = 0.5 + (i % 31) * 0.01
    }
    return array.f64(t)
}

func make_mask(n) {
    t := {}
    for i := 1; i <= n; i++ {
        t[i] = i % 3 != 0
    }
    return array.bool(t)
}

cols := soa.zip({
    value: make_value(N),
    weight: make_weight(N),
    active: make_mask(N),
})
mask := soa.column(cols, "active")

func run_hot(cols, mask, reps) {
    checksum := 0.0
    for r := 1; r <= reps; r++ {
        stats := soa.statsWhere(cols, "value", mask)
        checksum = checksum + stats.sum + stats.mean + stats.min + stats.max + stats.count
    }
    return checksum
}

warm := run_hot(cols, mask, 3)
collectgarbage("collect")
t0 := time.now()
checksum := run_hot(cols, mask, REPS)
elapsed := time.since(t0)

print(string.format("soa_masked_aggregate_hot n=%d reps=%d", N, REPS))
print(string.format("checksum: %.6f", checksum + warm * 0.000001))
print(string.format("Time: %.3fs", elapsed))
