// Conformance hot benchmark: regexp submatches, findAll/split, and deterministic random intervals.

MOD := 1000000007
N := 36000

func mix(h, v) {
	return (h * 131 + v) % MOD
}

func line(i) {
	return string.format("svc=api%d status=%d route=/v1/items/%d trace=t%05d", i % 17, 200 + (i % 5) * 100, i % 997, (i * 37) % 100000)
}

func run(n) {
	re := regexp.mustCompile("([a-z]+)=([a-z0-9/]+)")
	numRe := regexp.mustCompile("[0-9]+")
	checksum := 0
	seed := 17
	seen := {}
	intervalHits := 0
	for i := 1; i <= n; i++ {
		s := line(i)
		first := re.findSubmatch(s)
		checksum = mix(checksum, #first[1] + #first[2] * 3 + #first[3] * 7)
		all := re.findAllSubmatch(s, -1)
		for j := 1; j <= #all; j++ {
			checksum = mix(checksum, #all[j][1] + #all[j][2] + #all[j][3])
		}
		nums := numRe.findAll(s, 3)
		for j := 1; j <= #nums; j++ {
			checksum = mix(checksum, tonumber(nums[j]) % 10007)
		}
		parts := regexp.split("\\s+", s, -1)
		checksum = mix(checksum, #parts)

		seed = (seed * 48271) % 2147483647
		r := (seed % 97) - 48
		seen[r] = true
		checksum = mix(checksum, r + 100)

		width := (i % 31) + 1
		low := (seed % 200) - 100
		high := low + width
		pick := low + (seed % (width + 1))
		if pick >= low && pick <= high {
			intervalHits = intervalHits + 1
		}
		checksum = mix(checksum, (high - low) * 5 + pick + 128)
	}
	count := 0
	for _, _ := range pairs(seen) {
		count = count + 1
	}
	return mix(mix(checksum, count), intervalHits)
}

run(1000)

t0 := time.now()
checksum := run(N)
elapsed := time.since(t0)

print(string.format("checksum: %d", checksum))
print(string.format("Time: %.3fs", elapsed))
