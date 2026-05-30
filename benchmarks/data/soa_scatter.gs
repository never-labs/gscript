// Data-oriented hot benchmark: bool-mask index extraction plus scatter writes.

N := 32768
HALF := 16384
REPS := 900

func make_f64(n, scale, bias) {
    t := {}
    for i := 1; i <= n; i++ {
        t[i] = i * scale + bias
    }
    return array.f64(t)
}

func make_i64(n) {
    t := {}
    for i := 1; i <= n; i++ {
        t[i] = i
    }
    return array.i64(t)
}

func make_mask(n) {
    t := {}
    for i := 1; i <= n; i++ {
        t[i] = i % 2 == 0
    }
    return array.bool(t)
}

func make_values(n) {
    t := {}
    for i := 1; i <= n; i++ {
        t[i] = i * 0.25
    }
    return array.f64(t)
}

cols := soa.zip({
    id: make_i64(N),
    score: make_f64(N, 0.01, 1.0),
    value: make_f64(N, 0.125, 0.5),
})
mask := make_mask(N)
values := make_values(HALF)

func run_hot(cols, mask, values, reps) {
    checksum := 0.0
    for r := 1; r <= reps; r++ {
        idx := soa.indicesWhere(cols, mask)
        soa.scatterInto(cols, "score", idx, values)
        checksum = checksum + soa.sumWhere(cols, "score", mask)
        soa.scatterInto(cols, "score", idx, 1.5)
        checksum = checksum + soa.sumWhere(cols, "score", mask)
    }
    return checksum
}

warm := run_hot(cols, mask, values, 2)
collectgarbage("collect")
t0 := time.now()
checksum := run_hot(cols, mask, values, REPS)
elapsed := time.since(t0)

print(string.format("soa_scatter_hot n=%d reps=%d", N, REPS))
print(string.format("checksum: %.6f", checksum + warm * 0.000001))
print(string.format("Time: %.3fs", elapsed))
