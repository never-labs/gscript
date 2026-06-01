done := make(chan)

go func() {
    time.sleep(0.02)
    done <- "finished"
}()

result := ""
select {
case msg := <-done:
    result = msg
case <-time.after(0.005):
    result = "timeout"
}

print(result)
