func sum_range(start, stop) {
    total := 0
    for i := start; i <= stop; i++ {
        total = total + (i % 97) * (i % 89)
    }
    return total
}

func worker(start, stop, out) {
    out <- sum_range(start, stop)
}

n := 200000
workers := 4
chunk := n / workers
out := make(chan, workers)

for w := 0; w < workers; w++ {
    start := w * chunk + 1
    stop := (w + 1) * chunk
    if w == workers - 1 {
        stop = n
    }
    go worker(start, stop, out)
}

parallel := 0
for w := 1; w <= workers; w++ {
    parallel = parallel + <-out
}

single := sum_range(1, n)

if parallel != single {
    error("parallel sum mismatch")
}
if len(out) != 0 || cap(out) != workers {
    error("channel len/cap mismatch")
}

print(string.format("parallel=%d single=%d workers=%d", parallel, single, workers))
