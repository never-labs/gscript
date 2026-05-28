// Data-oriented hot benchmark: dense and masked column dot products.

N := 65536
REPS := 900

func make_f64(n, scale, bias) {
    t := {}
    for i := 1; i <= n; i++ {
        t[i] = i * scale + bias
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
    x: make_f64(N, 0.001, 1.0),
    y: make_f64(N, 0.002, 0.5),
    weight: make_f64(N, 0.01, 0.25),
})
mask := make_mask(N)

func run_hot(cols, mask, reps) {
    checksum := 0.0
    for r := 1; r <= reps; r++ {
        checksum = checksum + soa.dot(cols, "x", "y")
        checksum = checksum + soa.dotWhere(cols, "x", "weight", mask)
    }
    return checksum
}

warm := run_hot(cols, mask, 2)
collectgarbage("collect")
t0 := time.now()
checksum := run_hot(cols, mask, REPS)
elapsed := time.since(t0)

print(string.format("soa_dot_hot n=%d reps=%d", N, REPS))
print(string.format("checksum: %.6f", checksum + warm * 0.000001))
print(string.format("Time: %.3fs", elapsed))
