N := 2000

func producer(start, step, n, out) {
    for i := 1; i <= n; i++ {
        out <- start + i * step
    }
}

func run(n) {
    left := make(chan, 32)
    right := make(chan, 32)
    go producer(0, 2, n, left)
    go producer(1, 2, n, right)

    sum := 0
    for i := 1; i <= n * 2; i++ {
        select {
        case v := <-left:
            sum = sum + v
        case v := <-right:
            sum = sum + v
        }
    }
    return sum
}

warm := run(100)
collectgarbage("collect")
t0 := time.now()
sum := run(N)
elapsed := time.since(t0)

expected := N * (N + 1) + N * (N + 2)
if sum != expected {
    error(string.format("select_blocking_hot mismatch got=%d want=%d", sum, expected))
}

print(string.format("select_blocking_hot checksum=%d warm=%d", sum, warm))
print(string.format("Time: %.6fs", elapsed))
