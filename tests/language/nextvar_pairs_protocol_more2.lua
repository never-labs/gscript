print("case:nextvar_pairs_protocol_more2")

a = {}
local x, y, z = pairs(a)
assert(type(x) == "function" and y == a and z == nil)
local function foo(e, i)
  assert(e == a)
  if i <= 10 then return i + 1, i + 2 end
end
setmetatable(a, {__pairs = function (x) return foo, x, 0 end})
local i = 0
for k, v in pairs(a) do
  i = i + 1
  assert(k == i and v == k + 1)
end

print("ok")
