print("case:nextvar_length_nil")

assert(#{} == 0)
assert(#{nil} == 0)
assert(#{nil, nil} == 0)
assert(#{1, 2, 3, nil, nil} == 3)

local a = {}
for i = 1, 100 do
  a[i] = true
  assert(#a == i)
end

for i = 5, 95 do
  a[i] = nil
end

for i = 1, 4 do
  assert(a[i] == true)
end
for i = 5, 95 do
  assert(a[i] == nil)
end
for i = 96, 100 do
  assert(a[i] == true)
end

local x = false
local n = 0
for k, v in ipairs{true, false, true, false} do
  n = n + 1
  x = not x
  assert(k == n and x == v)
end
assert(n == 4)

print("ok")
