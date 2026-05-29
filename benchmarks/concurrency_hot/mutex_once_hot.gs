WORKERS := 4
N := 1000

func run(workers, n) {
    mu := sync.mutex()
    wg := sync.waitgroup()
    state := {value: 0}
    for w := 1; w <= workers; w++ {
        wg.add(1)
        go func(id) {
            local := 0
            for i := 1; i <= n; i++ {
                local = local + id + i
            }
            mu.lock()
            state.value = state.value + local
            mu.unlock()
            wg.done()
        }(w)
    }
    wg.wait()

    once := sync.once()
    initCount := 0
    for i := 1; i <= 16; i++ {
        once.do(func() {
            initCount = initCount + 1
        })
    }
    return state.value + initCount
}

warm := run(2, 10)
collectgarbage("collect")
t0 := time.now()
sum := run(WORKERS, N)
elapsed := time.since(t0)

expected := 1
for w := 1; w <= WORKERS; w++ {
    for i := 1; i <= N; i++ {
        expected = expected + w + i
    }
}
if sum != expected {
    error(string.format("mutex_once_hot mismatch got=%d want=%d", sum, expected))
}

print(string.format("mutex_once_hot checksum=%d warm=%d", sum, warm))
print(string.format("Time: %.6fs", elapsed))
