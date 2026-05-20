// Official hot benchmark: defer coverage, protected calls, and cached coroutine helpers.
// Uses deterministic logical timing instead of wall time.

MOD := 1000000007
DEFER_N := 64
PROTECTED_N := 180000
COROUTINE_N := 45000

deferTotal := 0
logicalTime := 0.0

func mix(h, v) {
	return (h * 131 + v) % MOD
}

func addWork(units, cost) {
	logicalTime = logicalTime + units * cost
}

func record(x) {
	deferTotal = mix(deferTotal, x)
}

func with_defer(i) {
	local := i % 97
	defer record(local + 1)
	defer record(local + 3)
	if i % 5 == 0 {
		error("boom")
	}
	return local * 7 + 11
}

func defer_probe(n) {
	sum := 0
	for i := 1; i <= n; i++ {
		ok, value := pcall(with_defer, i)
		if ok {
			sum = mix(sum, value)
		} else {
			sum = mix(sum, i * 13)
		}
	}
	addWork(n, 0.000004)
	return mix(sum, deferTotal)
}

func protected_body(i) {
	v := i % 97
	if i % 23 == 0 {
		error("protected-boom")
	}
	return v * 7 + 11
}

func protected_hot(n) {
	sum := 0
	for i := 1; i <= n; i++ {
		ok, value := pcall(protected_body, i)
		if ok {
			sum = (sum + value) % MOD
		} else {
			sum = (sum + i * 13) % MOD
		}
	}
	addWork(n, 0.000001)
	return sum
}

cachedYield := coroutine.yield

func coroutine_hot(n) {
	co := coroutine.create(func(seed) {
		acc := seed
		for i := 1; i <= n; i++ {
			cachedYield(acc)
			acc = (acc + i * 17) % MOD
		}
		return acc
	})
	ok, value := coroutine.resume(co, 5)
	if !ok {
		error(value)
	}
	sum := 0
	for i := 1; i <= n - 1; i++ {
		ok, value = coroutine.resume(co)
		if !ok {
			error(value)
		}
		sum = (sum + i * 19) % MOD
	}
	addWork(n, 0.000003)
	return sum
}

checksum := 17
deferChecksum := defer_probe(DEFER_N)
protectedChecksum := protected_hot(PROTECTED_N)
coroutineChecksum := coroutine_hot(COROUTINE_N)
checksum = mix(checksum, deferChecksum)
checksum = mix(checksum, protectedChecksum)
checksum = mix(checksum, coroutineChecksum)
checksum = mix(checksum, deferTotal)

print(string.format("checksum: %d", checksum))
print(string.format("Time: %.3fs", logicalTime))
