print("case:nextvar_ipairs_false_more")

local x = 0
for k, v in ipairs{10, 20, 30; x = 12} do
  x = x + 1
  assert(k == x and v == x * 10)
end
assert(x == 3)

local seen = false
for _ in ipairs{x = 12, y = 24} do seen = true end
assert(not seen)

x = false
local i = 0
for k, v in ipairs{true, false, true, false} do
  i = i + 1
  x = not x
  assert(k == i and x == v)
end
assert(i == 4)
assert(type(ipairs{}) == "function" and ipairs{} == ipairs{})

print("ok")
