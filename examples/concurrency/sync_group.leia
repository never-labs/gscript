group := sync.group()
out := make(chan, 4)

for i := 1; i <= 4; i++ {
    group.start(func(ctx, v) {
        out <- v * v
    }, i)
}

ok, err, failures := group.wait()
if !ok {
    error(err)
}

sum := 0
for i := 1; i <= 4; i++ {
    sum = sum + <-out
}

print(string.format("sum=%d failures=%d", sum, failures))
