ch := make(chan, 1)
status := "empty"

select {
case v := <-ch:
    status = string.format("received:%d", v)
default:
    status = "default"
}

ch <- 42

select {
case v := <-ch:
    status = string.format("%s received:%d", status, v)
default:
    status = status + " empty"
}

print(status)
