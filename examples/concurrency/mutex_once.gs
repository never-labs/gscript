mu := sync.mutex()
wg := sync.waitgroup()
state := {value: 0}

for worker := 1; worker <= 4; worker++ {
    wg.add(1)
    go func(id) {
        for i := 1; i <= 100; i++ {
            mu.lock()
            state.value = state.value + id
            mu.unlock()
        }
        wg.done()
    }(worker)
}

wg.wait()

once := sync.once()
initialized := 0
for i := 1; i <= 8; i++ {
    once.do(func() {
        initialized = initialized + 1
    })
}

print(string.format("value=%d initialized=%d", state.value, initialized))
