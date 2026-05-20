// Official hot benchmark: strings and patterns.
// Covers format, concat, sub/find, match/gmatch/gsub, and long string growth.

MOD := 1000000007

func mix(h, v) {
	return (h * 131 + v) % MOD
}

func checksumString(h, s) {
	h = mix(h, #s)
	step := math.floor(#s / 17) + 1
	for i := 1; i <= #s; i += step {
		h = mix(h, string.byte(s, i))
	}
	if #s > 0 {
		h = mix(h, string.byte(s, #s))
	}
	return h
}

t0 := time.now()
checksum := 17

rows := {}
for i := 1; i <= 14000; i++ {
	name := string.format("item_%05d", (i * 37) % 100000)
	tag := string.format("tag%02d", i % 97)
	line := string.format("%s;%s;value=%06d;hex=%04x", name, tag, i * 13, (i * 17) % 65536)
	rows[i] = line
	checksum = checksumString(checksum, line)
}

blob := table.concat(rows, "\n")
checksum = checksumString(checksum, blob)

subFindTotal := 0
for i := 1; i <= 22000; i++ {
	start := ((i * 29) % (#blob - 120)) + 1
	part := string.sub(blob, start, start + 89)
	subFindTotal = subFindTotal + #part
	a, b := string.find(part, "value=", 1, true)
	if a != nil {
		subFindTotal = subFindTotal + a + b
	}
	a, b = string.find(part, "tag%d%d")
	if a != nil {
		subFindTotal = subFindTotal + a * 3 + b
	}
	if i % 7 == 0 {
		checksum = checksumString(checksum, part)
	}
}
checksum = mix(checksum, subFindTotal)

patternTotal := 0
for pass := 1; pass <= 18; pass++ {
	for item, value := range string.gmatch(blob, "item_(%d+);tag%d%d;value=(%d+)") {
		patternTotal = patternTotal + tonumber(string.sub(item, 4, 5))
		patternTotal = patternTotal + tonumber(string.sub(value, 5, 6))
	}
}
checksum = mix(checksum, patternTotal)

matchTotal := 0
for i := 1; i <= 26000; i++ {
	idx := ((i * 19) % #rows) + 1
	item, value := string.match(rows[idx], "(item_%d+);tag%d%d;value=(%d+)")
	matchTotal = matchTotal + #item + tonumber(string.sub(value, 4, 6))
}
checksum = mix(checksum, matchTotal)

func repl(num, tag) {
	return tag .. ":" .. num
}

rewrite := ""
for pass := 1; pass <= 10; pass++ {
	rewrite = string.gsub(blob, "item_(%d+);(tag%d%d)", repl)
	checksum = checksumString(checksum, rewrite)
}

grow := ""
for i := 1; i <= 9000; i++ {
	grow = grow .. string.sub(rows[(i % #rows) + 1], 1, 8)
	if i % 900 == 0 {
		checksum = checksumString(checksum, grow)
	}
}
checksum = checksumString(checksum, grow)

finalLen := #blob + #rewrite + #grow
checksum = mix(checksum, finalLen)
elapsed := time.since(t0)

print(string.format("checksum: %d", checksum))
print(string.format("Time: %.3fs", elapsed))
