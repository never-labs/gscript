N := 200000

ready := make(chan, 1)
empty := make(chan, 1)
sum := 0

func run(n) {
    total := 0
    for i := 1; i <= n; i++ {
        ready <- i
        select {
        case v := <-ready:
            total = total + v
        default:
            total = total - 1000000
        }
        select {
        case v := <-empty:
            total = total - v
        default:
            total = total + 1
        }
    }
    return total
}

warm := run(1000)
collectgarbage("collect")
t0 := time.now()
sum = run(N)
elapsed := time.since(t0)

expected := (N * (N + 1)) / 2 + N
if sum != expected {
    error(string.format("select_default_hot mismatch got=%d want=%d", sum, expected))
}

print(string.format("select_default_hot checksum=%d warm=%d", sum, warm))
print(string.format("Time: %.6fs", elapsed))
