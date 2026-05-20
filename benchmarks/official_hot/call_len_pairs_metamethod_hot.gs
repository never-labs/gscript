// Official hot benchmark: __call, __len, and __pairs metamethods.

MOD := 1000000007
GROUPS := 80
REPS := 80
PAIR_EVERY := 8

func make_callable(seed) {
	mt := {}
	mt.__call = func(t, x, y) {
		t.count = t.count + 1
		return (x * 17 + y * 19 + t.bias + t.count * 23) % MOD
	}
	return setmetatable({bias: seed * 3 + 23, count: 0}, mt)
}

func make_pair_proxy(seed, n) {
	backing := {}
	for i := 1; i <= n; i++ {
		backing[i] = seed + i * 3 + 1
	}
	mt := {}
	mt.__len = func(_) { return n }
	mt.__pairs = func(obj) {
		i := 0
		return func(_, last) {
			i = i + 1
			if i <= n {
				return i, backing[i]
			}
		}, obj, nil
	}
	return setmetatable({}, mt)
}

func run(groups, reps) {
	items := {}
	for b := 1; b <= groups; b++ {
		items[b] = {
			callable: make_callable(b),
			proxy: make_pair_proxy(b, 8),
		}
	}
	checksum := 0
	for b := 1; b <= groups; b++ {
		item := items[b]
		for i := 1; i <= reps; i++ {
			checksum = (checksum + item.callable(i + b, i % 11) + #item.proxy * 13) % MOD
			if i % PAIR_EVERY == 0 {
				iter, state, last := getmetatable(item.proxy).__pairs(item.proxy)
				_ = iter
				_ = state
				_ = last
				checksum = (checksum + 252) % MOD
			}
		}
	}
	return checksum
}

t0 := time.now()
checksum := run(GROUPS, REPS)
elapsed := time.since(t0)

print(string.format("checksum: %d", checksum))
print(string.format("Time: %.3fs", elapsed))
