print("case:events_partial_order_more")

local t = {}
local function rawSet(x)
  local y = {}
  for _, k in pairs(x) do y[k] = 1 end
  return y
end
local function Set(x)
  return setmetatable(rawSet(x), t)
end
t.__lt = function (a, b)
  for k in pairs(a) do
    if not b[k] then return false end
    b[k] = nil
  end
  return next(b) ~= nil
end
t.__le = function (a, b)
  for k in pairs(a) do
    if not b[k] then return false end
  end
  return true
end
assert(Set{1,2,3} < Set{1,2,3,4})
assert(not (Set{1,2,3,4} < Set{1,2,3,4}))
assert(Set{1,2,3,4} <= Set{1,2,3,4})
assert(Set{1,2,3,4} >= Set{1,2,3,4})
assert(not (Set{1,3} <= Set{3,5}))
assert(not (Set{1,3} >= Set{3,5}))

print("ok")
