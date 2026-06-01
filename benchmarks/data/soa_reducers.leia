// Data-oriented hot benchmark: full-column fused stats reducer.

N := 65536
REPS := 2200

func make_f64(n, scale, bias) {
    t := {}
    for i := 1; i <= n; i++ {
        t[i] = i * scale + bias
    }
    return array.f64(t)
}

cols := soa.zip({
    value: make_f64(N, 0.01, -200.0),
})

func run_hot(cols, reps) {
    checksum := 0.0
    for r := 1; r <= reps; r++ {
        stats := soa.stats(cols, "value")
        checksum = checksum + stats.count * 0.000001 + stats.sum * 0.000000001 + stats.min + stats.max + stats.mean
    }
    return checksum
}

warm := run_hot(cols, 2)
collectgarbage("collect")
t0 := time.now()
checksum := run_hot(cols, REPS)
elapsed := time.since(t0)

print(string.format("soa_reducers_hot n=%d reps=%d", N, REPS))
print(string.format("checksum: %.6f", checksum + warm * 0.000001))
print(string.format("Time: %.3fs", elapsed))
