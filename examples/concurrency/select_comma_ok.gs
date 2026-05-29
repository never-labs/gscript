ch := make(chan, 1)
ch <- 21
first, ok1 := <-ch
close(ch)

second := "unset"
ok2 := true

select {
case v, ok := <-ch:
    if v == nil {
        second = "closed"
    }
    ok2 = ok
default:
    second = "default"
}

print(string.format("first=%d ok1=%s second=%s ok2=%s", first, tostring(ok1), second, tostring(ok2)))
