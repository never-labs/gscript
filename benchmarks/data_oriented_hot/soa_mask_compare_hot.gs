// Data-oriented hot benchmark: column comparisons that produce reusable masks.

N := 65536
REPS := 1000

func make_value(n) {
    t := {}
    for i := 1; i <= n; i++ {
        t[i] = 1.0 + (i % 257) * 0.125
    }
    return array.f64(t)
}

func make_limit(n) {
    t := {}
    for i := 1; i <= n; i++ {
        t[i] = 12.0 + (i % 19) * 0.25
    }
    return array.f64(t)
}

cols := soa.zip({
    value: make_value(N),
    limit: make_limit(N),
})

func run_hot(cols, reps) {
    checksum := 0.0
    for r := 1; r <= reps; r++ {
        scalarMask := soa.mask(cols, "value", ">=", 8.5)
        columnMask := soa.mask(cols, "value", "<", "limit")
        checksum = checksum + soa.countWhere(cols, scalarMask)
        checksum = checksum + soa.sumWhere(cols, "value", columnMask)
    }
    return checksum
}

warm := run_hot(cols, 3)
collectgarbage("collect")
t0 := time.now()
checksum := run_hot(cols, REPS)
elapsed := time.since(t0)

print(string.format("soa_mask_compare_hot n=%d reps=%d", N, REPS))
print(string.format("checksum: %.6f", checksum + warm * 0.000001))
print(string.format("Time: %.3fs", elapsed))
