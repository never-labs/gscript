// Data-oriented hot benchmark: mask select into dense temporaries.

N := 65536
REPS := 1200

func make_value(n) {
    t := {}
    for i := 1; i <= n; i++ {
        t[i] = 1.0 + (i % 257) * 0.125
    }
    return array.f64(t)
}

func make_fallback(n) {
    t := {}
    for i := 1; i <= n; i++ {
        t[i] = 0.25 + (i % 17) * 0.5
    }
    return array.f64(t)
}

func make_zero(n) {
    t := {}
    for i := 1; i <= n; i++ {
        t[i] = 0.0
    }
    return array.f64(t)
}

cols := soa.zip({
    value: make_value(N),
    fallback: make_fallback(N),
    selected: make_zero(N),
})
mask := soa.mask(cols, "value", ">=", 8.5)

func run_hot(cols, mask, reps) {
    checksum := 0.0
    for r := 1; r <= reps; r++ {
        checksum = checksum + soa.sumSelect(cols, mask, "value", "fallback")
    }
    return checksum
}

warm := run_hot(cols, mask, 3)
collectgarbage("collect")
t0 := time.now()
checksum := run_hot(cols, mask, REPS)
elapsed := time.since(t0)

print(string.format("soa_select_hot n=%d reps=%d", N, REPS))
print(string.format("checksum: %.6f", checksum + warm * 0.000001))
print(string.format("Time: %.3fs", elapsed))
