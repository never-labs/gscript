N := 100

func run(n) {
    hits := 0
    for i := 1; i <= n; i++ {
        never := make(chan)
        select {
        case <-never:
            hits = hits - 1000000
        case <-time.after(0.000001):
            hits = hits + 1
        }
    }
    return hits
}

warm := run(2)
collectgarbage("collect")
t0 := time.now()
hits := run(N)
elapsed := time.since(t0)

if hits != N {
    error(string.format("select_timeout_hot mismatch got=%d want=%d", hits, N))
}

print(string.format("select_timeout_hot checksum=%d warm=%d", hits, warm))
print(string.format("Time: %.6fs", elapsed))
