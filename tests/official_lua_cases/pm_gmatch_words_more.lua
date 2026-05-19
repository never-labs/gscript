print("case:pm_gmatch_words_more")

local t = {n = 0}
for w in string.gmatch("first second word", "%w+") do
  t.n = t.n + 1
  t[t.n] = w
end
assert(t[1] == "first" and t[2] == "second" and t[3] == "word" and t.n == 3)

t = {}
for i, j in string.gmatch("13 14 10 = 11, 15= 16, 22=23", "(%d+)%s*=%s*(%d+)") do
  t[tonumber(i)] = tonumber(j)
end

local a = 0
for k, v in pairs(t) do
  assert(k + 1 == v + 0)
  a = a + 1
end
assert(a == 3)

print("ok")
