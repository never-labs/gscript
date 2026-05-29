WORKERS := 4
SLEEP := 0.05

func sleeper(delay, out) {
    time.sleep(delay)
    out <- 1
}

func run_single(workers, delay) {
    total := 0
    for i := 1; i <= workers; i++ {
        time.sleep(delay)
        total = total + 1
    }
    return total
}

func run_parallel(workers, delay) {
    out := make(chan, workers)
    for i := 1; i <= workers; i++ {
        go sleeper(delay, out)
    }
    total := 0
    for i := 1; i <= workers; i++ {
        total = total + <-out
    }
    return total
}

warm := run_parallel(2, 0.001)

collectgarbage("collect")
t0 := time.now()
single := run_single(WORKERS, SLEEP)
singleElapsed := time.since(t0)

collectgarbage("collect")
t1 := time.now()
parallel := run_parallel(WORKERS, SLEEP)
parallelElapsed := time.since(t1)

if single != parallel {
    error("parallel sleep mismatch")
}

speedup := singleElapsed / parallelElapsed
print(string.format("goroutine_sleep_hot workers=%d sleep=%.3f", WORKERS, SLEEP))
print(string.format("checksum: %.6f", parallel + warm * 0.000001))
print(string.format("single: %.6fs", singleElapsed))
print(string.format("parallel: %.6fs", parallelElapsed))
print(string.format("speedup: %.2fx", speedup))
print(string.format("Time: %.3fs", parallelElapsed))
