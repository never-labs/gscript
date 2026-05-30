// Conformance hot benchmark: table.move, table.sort, proxy __index/__newindex.

MOD := 1000000007
N := 420
PASSES := 1500

func hot_move(src, first, last, target, dst) {
	return table.move(src, first, last, target, dst)
}

func checksum_array(a, n) {
	h := 17
	for i := 1; i <= n; i++ {
		h = (h * 131 + a[i] * (i % 97 + 1)) % MOD
	}
	return h
}

func make_array(n, salt) {
	a := {}
	for i := 1; i <= n; i++ {
		a[i] = (i * 97 + salt * 53 + n * 17) % 100000
	}
	return a
}

func run(n, passes) {
	checksum := 0
	for pass := 1; pass <= passes; pass++ {
		a := make_array(n, pass)
		table.sort(a)
		checksum = (checksum + checksum_array(a, n)) % MOD

		b := {}
		hot_move(a, 1, n, 1, b)
		hot_move(b, 1, n - 3, 4, b)
		checksum = (checksum + checksum_array(b, n)) % MOD

		src := b
		dst := {}
		reads := 0
		writes := 0
		proxySrc := setmetatable({}, {
			__len: func() { return n },
			__index: func(_, k) {
				reads = reads + 1
				return src[k]
			},
		})
		proxyDst := setmetatable({}, {
			__newindex: func(_, k, v) {
				writes = writes + 1
				dst[k] = v
			},
		})
		hot_move(proxySrc, 1, n, 1, proxyDst)
		proxyLen := #proxySrc
		checksum = (checksum + checksum_array(dst, n) + reads + writes + proxyLen) % MOD
	}
	return checksum
}

run(80, 2)

t0 := time.now()
checksum := run(N, PASSES)
elapsed := time.since(t0)

print(string.format("Checksum: %d", checksum))
print(string.format("Time: %.3fs", elapsed))
