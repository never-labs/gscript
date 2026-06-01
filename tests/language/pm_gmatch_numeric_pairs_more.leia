print("case:pm_gmatch_numeric_pairs_more")

t := {}
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
