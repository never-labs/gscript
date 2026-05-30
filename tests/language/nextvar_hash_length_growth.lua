print("case:nextvar_hash_length_growth")

local a = {}

for i = 1, 100 do
  a[i .. "+"] = true
end
for i = 1, 100 do
  a[i .. "+"] = nil
end

for i = 1, 100 do
  a[i] = true
  assert(#a == i)
end

assert(a["1+"] == nil)
assert(a["100+"] == nil)
assert(a[1] == true)
assert(a[100] == true)

print("ok")
