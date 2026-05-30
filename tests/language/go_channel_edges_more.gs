print("case:go_channel_edges_more")

ping := make(chan)
pong := make(chan)
done := make(chan)

go func(base) {
	sum := 0
	count := 0
	for i := 1; i <= 5; i++ {
		v := <-ping
		sum = sum + v + base
		count = count + 1
		pong <- v * 2 + base
	}
	done <- {sum: sum, count: count}
}(10)

sentSum := 0
replySum := 0
for i := 1; i <= 5; i++ {
	ping <- i
	reply := <-pong
	sentSum = sentSum + i
	replySum = replySum + reply
}

stats := <-done
assert(sentSum == 15)
assert(replySum == 80)
assert(stats.sum == 65)
assert(stats.count == 5)

closed := make(chan, 1)
close(closed)
ok, err := pcall(func() {
	closed <- "x"
})
assert(ok == false)
assert(string.find(err, "send on closed channel", 1, true) != nil)

print("ok")
