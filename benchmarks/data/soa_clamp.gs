// Data-oriented hot benchmark: dense column clamp and clampInto.

N := 65536
REPS := 1600

func make_f64(n, scale, bias) {
    t := {}
    for i := 1; i <= n; i++ {
        t[i] = i * scale + bias
    }
    return array.f64(t)
}

cols := soa.zip({
    value: make_f64(N, 0.01, -200.0),
    clamped: make_f64(N, 0.0, 0.0),
})

func run_hot(cols, reps) {
    checksum := 0.0
    for r := 1; r <= reps; r++ {
        tmp := soa.clamp(cols, "value", -25.0, 125.0)
        checksum = checksum + tmp[1] + tmp[N]
        soa.clampInto(cols, "clamped", "value", -25.0, 125.0)
        checksum = checksum + soa.column(cols, "clamped")[1] + soa.column(cols, "clamped")[N]
    }
    return checksum
}

warm := run_hot(cols, 2)
collectgarbage("collect")
t0 := time.now()
checksum := run_hot(cols, REPS)
elapsed := time.since(t0)

print(string.format("soa_clamp_hot n=%d reps=%d", N, REPS))
print(string.format("checksum: %.6f", checksum + warm * 0.000001))
print(string.format("Time: %.3fs", elapsed))
