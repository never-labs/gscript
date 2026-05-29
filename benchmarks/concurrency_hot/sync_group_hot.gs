N := 200
WORKERS := 4

func run(workers, n) {
    group := sync.group()
    out := make(chan, workers)
    for w := 1; w <= workers; w++ {
        group.start(func(id) {
            sum := 0
            for i := 1; i <= n; i++ {
                sum = sum + i + id
            }
            out <- sum
        }, w)
    }
    ok, err, failures := group.wait()
    if !ok {
        error(string.format("sync_group_hot failed count=%d err=%s", failures, err))
    }
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
    error(string.format("sync_group_hot mismatch got=%d want=%d", sum, expected))
}

print(string.format("sync_group_hot checksum=%d warm=%d", sum, warm))
print(string.format("Time: %.6fs", elapsed))
