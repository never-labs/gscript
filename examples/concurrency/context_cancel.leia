ctx, cancel := context.withCancel()
out := make(chan, 1)

go func() {
    total := 0
    for total < 1000 {
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
print(string.format("cancelled=%s result=%d err=%s", tostring(ctx.cancelled()), result, ctx.err()))
