print("case:pm_gmatch_numeric_pairs_more")

t = {}
for i, j in string.gmatch("13 14 10 = 11, 15= 16, 22=23", "(%d+)%s*=%s*(%d+)") do
  t[tonumber(i)] = tonumber(j)
end
a = 0
for k, v in pairs(t) do
  assert(k + 1 == v + 0)
  a = a + 1
end
assert(a == 3)

print("ok")
