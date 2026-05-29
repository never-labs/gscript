group := sync.group()
out := make(chan, 1)

group.start(func(ctx) {
    error("worker failed")
})

group.start(func(ctx) {
    ok, err := time.sleep(ctx, 1.0)
    out <- string.format("sleep_ok=%s err=%s", tostring(ok), tostring(err))
})

ok, err, failures := group.wait()
sleepResult := <-out

print(string.format("ok=%s failures=%d err=%s %s", tostring(ok), failures, err, sleepResult))
