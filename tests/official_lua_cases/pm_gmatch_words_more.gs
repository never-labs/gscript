print("case:pm_gmatch_words_more")

t := {n: 0}
for w := range string.gmatch("first second word", "%w+") {
  t.n = t.n + 1
  t[t.n] = w
}
assert(t[1] == "first" && t[2] == "second" && t[3] == "word" && t.n == 3)

t = {}
for i, j := range string.gmatch("13 14 10 = 11, 15= 16, 22=23", "(%d+)%s*=%s*(%d+)") {
  t[tonumber(i)] = tonumber(j)
}

a := 0
for k, v := range pairs(t) {
  assert(k + 1 == v + 0)
  a = a + 1
}
assert(a == 3)

print("ok")
