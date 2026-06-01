N := 200
WORKERS := 4

func run(workers, n) {
    wg := sync.waitgroup()
    out := make(chan, workers)
    for w := 1; w <= workers; w++ {
        wg.add(1)
        go func(id) {
            sum := 0
            for i := 1; i <= n; i++ {
                sum = sum + i + id
            }
            out <- sum
            wg.done()
        }(w)
    }
    wg.wait()
    total := 0
    for w := 1; w <= workers; w++ {
        total = total + <-out
    }
    return total
}

warm := run(2, 10)
collectgarbage("collect")
t0 := time.now()
sum := run(WORKERS, N)
elapsed := time.since(t0)

expected := 0
for w := 1; w <= WORKERS; w++ {
    for i := 1; i <= N; i++ {
        expected = expected + i + w
    }
}
if sum != expected {
    error(string.format("waitgroup_hot mismatch got=%d want=%d", sum, expected))
}

print(string.format("waitgroup_hot checksum=%d warm=%d", sum, warm))
print(string.format("Time: %.6fs", elapsed))
