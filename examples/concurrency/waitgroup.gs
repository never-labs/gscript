wg := sync.waitgroup()
out := make(chan, 4)

for i := 1; i <= 4; i++ {
    wg.add(1)
    go func(v) {
        out <- v * v
        wg.done()
    }(i)
}

wg.wait()

sum := 0
for i := 1; i <= 4; i++ {
    sum = sum + <-out
}

print(string.format("sum=%d", sum))
