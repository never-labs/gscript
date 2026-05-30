print("case:go_channel_host_more")

ch := make(chan, 4)
go func() {
	for i := 1; i <= 4; i++ {
		ch <- i
	}
	close(ch)
}()

sum := 0
count := 0
for v := range ch {
	sum = sum + v
	count = count + 1
}
assert(sum == 10)
assert(count == 4)

closed := <-ch
assert(closed == nil)

ok, err := pcall(func() {
	bad := make(chan, -1)
	return bad
})
assert(ok == false)
assert(string.find(err, "non-negative", 1, true) != nil)

ok, err = pcall(func() {
	bad := make(chan, "bad")
	return bad
})
assert(ok == false)
assert(string.find(err, "integer", 1, true) != nil)

badClose := make(chan, 1)
close(badClose)
ok, err = pcall(close, badClose)
assert(ok == false)
assert(string.find(err, "closed channel", 1, true) != nil)

print("ok")
