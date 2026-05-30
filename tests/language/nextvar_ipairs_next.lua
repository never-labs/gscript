print("case:nextvar_ipairs_next")

local a = {}
for i = 1, 100 do
  a[i] = true
  assert(#a == i)
end

local x = 0
for k, v in ipairs{10, 20, 30; x = 12} do
  x = x + 1
  assert(k == x and v == x * 10)
end

for _ in ipairs{x = 12, y = 24} do assert(nil) end

x = false
local i = 0
for k, v in ipairs{true, false, true, false} do
  i = i + 1
  x = not x
  assert(x == v)
end
assert(i == 4)

assert(type(ipairs{}) == "function")

local k, v = next({10})
assert(k == 1 and v == 10)
assert(next({}) == nil)

print("ok")
