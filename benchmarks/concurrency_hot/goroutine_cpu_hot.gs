WORKERS := 4
N := 4000000

func spin(n) {
    s := 0
    for i := 1; i <= n; i++ {
        s = s + (i % 97) * (i % 89)
    }
    return s
}

func run_single(workers, n) {
    total := 0
    for w := 1; w <= workers; w++ {
        total = total + spin(n)
    }
    return total
}

func run_parallel(workers, n) {
    ch := make(chan, workers)
    for w := 1; w <= workers; w++ {
        go func(size) {
            ch <- spin(size)
        }(n)
    }
    total := 0
    for w := 1; w <= workers; w++ {
        total = total + <-ch
    }
    return total
}

warm := run_parallel(WORKERS, N)

collectgarbage("collect")
t0 := time.now()
single := run_single(WORKERS, N)
singleElapsed := time.since(t0)

collectgarbage("collect")
t1 := time.now()
parallel := run_parallel(WORKERS, N)
parallelElapsed := time.since(t1)

if single != parallel {
    error("cpu parallel mismatch")
}

speedup := singleElapsed / parallelElapsed
print(string.format("goroutine_cpu_hot workers=%d n=%d", WORKERS, N))
print(string.format("checksum: %.6f", parallel + warm * 0.000000001))
print(string.format("single: %.6fs", singleElapsed))
print(string.format("parallel: %.6fs", parallelElapsed))
print(string.format("speedup: %.2fx", speedup))
print(string.format("Time: %.3fs", parallelElapsed))
