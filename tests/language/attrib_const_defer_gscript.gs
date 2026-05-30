order := ""

func record(s) {
	order = order .. s
}

func work() {
	const cfg = {count: 1}
	cfg.count = 2
	defer record("one")
	defer record("two")
	ok, err := pcall(func() {
		cfg = {}
	})
	if ok {
		record("bad")
	}
	record("body")
	error("boom")
	return ok, cfg.count
}

ok, count := pcall(work)
print(ok, count, order)
