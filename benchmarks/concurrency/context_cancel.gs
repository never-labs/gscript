N := 2000

ctx, cancel := context.withCancel()
out := make(chan, 1)

collectgarbage("collect")
t0 := time.now()

go func() {
    total := 0
    for total < N {
        select {
        case <-ctx.done:
            out <- total
            return;
        default:
            total = total + 1
        }
    }
    cancel()
    out <- total
}()

result := <-out
elapsed := time.since(t0)

if result != N {
    error(string.format("context_cancel_hot mismatch got=%d want=%d", result, N))
}

print(string.format("context_cancel_hot checksum=%d", result))
print(string.format("Time: %.6fs", elapsed))
