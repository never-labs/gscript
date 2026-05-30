N := 200000

ready := make(chan, 1)
closed := make(chan, 1)
close(closed)

func run(n) {
    total := 0
    for i := 1; i <= n; i++ {
        ready <- i
        select {
        case v, ok := <-ready:
            if ok {
                total = total + v
            }
        default:
            total = total - 1000000
        }
        select {
        case v, ok := <-closed:
            if v == nil && !ok {
                total = total + 1
            }
        default:
            total = total - 1000000
        }
    }
    return total
}

warm := run(1000)
collectgarbage("collect")
t0 := time.now()
sum := run(N)
elapsed := time.since(t0)

expected := (N * (N + 1)) / 2 + N
if sum != expected {
    error(string.format("select_comma_ok_hot mismatch got=%d want=%d", sum, expected))
}

print(string.format("select_comma_ok_hot checksum=%d warm=%d", sum, warm))
print(string.format("Time: %.6fs", elapsed))
