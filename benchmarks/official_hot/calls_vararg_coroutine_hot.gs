// Hot benchmark: calls, varargs, closures, coroutine resume/yield control flow.

MOD := 1000000007
N_CALLS := 220000
N_CORO := 90000

func triple(x) {
	return x, x + 1, x + 2
}

func adjusted_call(i) {
	a := i
	b := i + 1
	c := i + 2
	d := i + 5
	return (a + b * 3 + c * 5 + d * 7 + i * 11) % MOD
}

func vararg_fold(base, ...) {
	n := select("#", ...)
	u1 := select(1, ...)
	u2 := select(2, ...)
	u3 := select(3, ...)
	u4 := select(4, ...)
	tail := select(4, ...)
	return (base + n * 13 + u1 * 17 + u2 * 19 + u3 * 23 + u4 * 29 + tail * 31) % MOD
}

func make_worker(seed) {
	captured := seed
	return func(i, v1, v2, v3, v4) {
		a := i + captured
		b := a + 1
		c := a + 2
		v := (a + v1 * 17 + v2 * 19 + v3 * 23 + v4 * 29) % MOD
		captured = (captured + b + c + 4) % MOD
		return (v + captured) % MOD
	}
}

func coroutine_pipeline(worker, n) {
	co := coroutine.create(func(start) {
		acc := start
		for i := 1; i <= n; i++ {
			coroutine.yield(acc)
			step := i % 11
			a := i + 1
			b := i + 2
			c := i + 3
			acc = (acc + worker(i + step, a, b, c, step) + step) % MOD
		}
		return acc, n
	})

	ok, pair := coroutine.resume(co, 7)
	if !ok { error(pair) }

	total := 0
	for i := 1; i <= n - 1; i++ {
		ok, pair = coroutine.resume(co)
		if !ok { error(pair) }
		total = (total + i * 17) % MOD
	}
	return total
}

func workload(nCalls, nCoro) {
	worker := make_worker(19)
	checksum := 0
	for i := 1; i <= nCalls; i++ {
		checksum = (checksum + adjusted_call(i)) % MOD
		checksum = (checksum + vararg_fold(i, i + 1, i + 2, i + 3, i + 4)) % MOD
		checksum = (checksum + worker(i, i + 1, i + 2, i + 3, i + 4)) % MOD
	}
	checksum = (checksum + coroutine_pipeline(worker, nCoro)) % MOD
	return checksum
}

workload(2000, 1000)

t0 := time.now()
checksum := workload(N_CALLS, N_CORO)
elapsed := time.since(t0)

print(string.format("calls_vararg_coroutine_hot checksum=%d", checksum))
print(string.format("Time: %.3fs", elapsed))
