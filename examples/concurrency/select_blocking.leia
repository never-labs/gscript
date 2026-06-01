left := make(chan)
right := make(chan)
done := make(chan, 2)

go func() {
    time.sleep(0.01)
    left <- 10
}()

go func() {
    time.sleep(0.02)
    right <- 20
}()

for i := 1; i <= 2; i++ {
    select {
    case v := <-left:
        done <- v
    case v := <-right:
        done <- v
    }
}

a := <-done
b := <-done
print(string.format("sum=%d", a + b))
