// Data-oriented hot benchmark: SoA compact/filter plus gather materialization.

N := 32768
FILTER_REPS := 180
GATHER_REPS := 180

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
        t[i] = i % 4 != 0
    }
    return array.bool(t)
}

func make_indices(n) {
    t := {}
    out := 1
    for i := n; i >= 1; i = i - 3 {
        t[out] = i
        out = out + 1
    }
    return array.i64(t)
}

cols := soa.zip({
    id: make_i64(N),
    x: make_f64(N, 0.001, 1.0),
    y: make_f64(N, 0.002, 2.0),
    value: make_f64(N, 0.125, 0.5),
    weight: make_f64(N, 0.01, 0.25),
})
mask := make_mask(N)
indices := make_indices(N)

func run_hot(cols, mask, indices, filterReps, gatherReps) {
    checksum := 0.0
    for r := 1; r <= filterReps; r++ {
        filtered := soa.filter(cols, mask)
        checksum = checksum + soa.sum(filtered, "value")
    }
    for r := 1; r <= gatherReps; r++ {
        picked := soa.gather(cols, indices)
        checksum = checksum + soa.sum(picked, "x")
    }
    return checksum
}

warm := run_hot(cols, mask, indices, 2, 2)
collectgarbage("collect")
t0 := time.now()
checksum := run_hot(cols, mask, indices, FILTER_REPS, GATHER_REPS)
elapsed := time.since(t0)

print(string.format("soa_filter_gather_hot n=%d filter_reps=%d gather_reps=%d", N, FILTER_REPS, GATHER_REPS))
print(string.format("checksum: %.6f", checksum + warm * 0.000001))
print(string.format("Time: %.3fs", elapsed))
