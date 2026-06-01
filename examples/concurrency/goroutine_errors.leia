done := make(chan, 1)

debug.setSink(func(e) {
    if e.type == "error" && e.kind == "goroutine" {
        print("goroutine error:", e.name, e.error)
        done <- true
    }
})

go func() {
    error("background failure")
}()

<-done
debug.setSink(nil)
