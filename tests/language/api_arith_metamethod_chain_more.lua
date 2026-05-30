print("case:api_arith_metamethod_chain_more")

local mt = {}
mt.__add = function(a, b)
  return setmetatable({v = a.v + b.v}, mt)
end
mt.__mod = function(a, b)
  return setmetatable({v = a.v % b.v}, mt)
end
mt.__unm = function(a)
  return setmetatable({v = a.v * 2}, mt)
end

local a = setmetatable({v = 4}, mt)
local b = setmetatable({v = 8}, mt)
local c = setmetatable({v = -3}, mt)

assert((a + b).v == 12)
assert((a % c).v == 4 % -3)
assert(((-b) + (a % c)).v == 14)

print("ok")
