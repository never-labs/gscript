print("case:rand_go_host_more")

randMod := require("rand")
assert(randMod == rand)
assert(package.loaded.rand == rand)

rand.seed(12345)
a1 := rand.int()
a2 := rand.int(10)
a3 := rand.int(-3, 3)
a4 := rand.float()
a5 := rand.bool()
a6 := rand.normal()
a7 := rand.normal(10, 0)
a8 := rand.exp(2)

rand.seed(12345)
assert(rand.int() == a1)
assert(rand.int(10) == a2)
assert(rand.int(-3, 3) == a3)
assert(rand.float() == a4)
assert(rand.bool() == a5)
assert(rand.normal() == a6)
assert(rand.normal(10, 0) == a7)
assert(rand.exp(2) == a8)

assert(type(a1) == "number" && math.type(a1) == "integer" && a1 >= 0)
assert(math.type(a2) == "integer" && 0 <= a2 && a2 < 10)
assert(math.type(a3) == "integer" && -3 <= a3 && a3 <= 3)
assert(type(a4) == "number" && 0 <= a4 && a4 < 1)
assert(type(a5) == "boolean")
assert(type(a6) == "number")
assert(a7 == 10)
assert(type(a8) == "number" && a8 >= 0)

assert(rand.int(7, 7) == 7)
assert(rand.choice({}) == nil)
assert(rand.choice({"only"}) == "only")

items := {1, 2, 3, 4}
ret := rand.shuffle(items)
assert(ret == items)
sum := 0
seen := {}
for i := 1; i <= #items; i++ {
	sum = sum + items[i]
	seen[items[i]] = true
}
assert(sum == 10)
assert(seen[1] && seen[2] && seen[3] && seen[4])

sample := rand.sample({"a", "b", "c"}, 10)
assert(#sample == 3)
mark := {}
for i := 1; i <= #sample; i++ {
	assert(sample[i] == "a" || sample[i] == "b" || sample[i] == "c")
	assert(!mark[sample[i]])
	mark[sample[i]] = true
}
assert(#rand.sample({"x", "y"}, 0) == 0)

assert(rand.weighted({"a", "b"}, {0, 5}) == "b")
assert(rand.weighted({}, {}) == nil)

u := rand.uuid()
assert(type(u) == "string")
assert(#u == 36)
assert(string.sub(u, 15, 15) == "4")
variant := string.sub(u, 20, 20)
assert(variant == "8" || variant == "9" || variant == "a" || variant == "b")

bs := rand.bytes(16)
assert(type(bs) == "string" && #bs == 16)
assert(rand.bytes(0) == "")

assert(!pcall(rand.seed))
assert(!pcall(rand.seed, {}))
assert(!pcall(rand.int, 0))
assert(!pcall(rand.int, 5, 4))
assert(!pcall(rand.int, "bad"))
assert(!pcall(rand.choice, 1))
assert(!pcall(rand.shuffle, "x"))
assert(!pcall(rand.sample, {}, -1))
assert(!pcall(rand.sample, {}, "bad"))
assert(!pcall(rand.normal, 0, -1))
assert(!pcall(rand.exp, 0))
assert(!pcall(rand.weighted, {"a"}, {0}))
assert(!pcall(rand.weighted, {"a"}, {-1}))

print("ok")
