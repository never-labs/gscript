// Data-oriented hot benchmark: dense prefix scan and scanInto.

N := 65536
REPS := 1400

func make_f64(n, scale, bias) {
    t := {}
    for i := 1; i <= n; i++ {
        t[i] = i * scale + bias
    }
    return array.f64(t)
}

cols := soa.zip({
    value: make_f64(N, 0.001, 1.0),
    prefix: make_f64(N, 0.0, 0.0),
})

func run_hot(cols, reps) {
    checksum := 0.0
    for r := 1; r <= reps; r++ {
        tmp := soa.scan(cols, "value")
        checksum = checksum + tmp[N]
        soa.scanInto(cols, "prefix", "value")
        checksum = checksum + soa.column(cols, "prefix")[N]
    }
    return checksum
}

warm := run_hot(cols, 2)
collectgarbage("collect")
t0 := time.now()
checksum := run_hot(cols, REPS)
elapsed := time.since(t0)

print(string.format("soa_scan_hot n=%d reps=%d", N, REPS))
print(string.format("checksum: %.6f", checksum + warm * 0.000001))
print(string.format("Time: %.3fs", elapsed))
