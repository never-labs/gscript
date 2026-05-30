print("case:events_eq_invalidate")

local mt = {}
mt.__eq = true

local a = setmetatable({10}, mt)
local b = setmetatable({10}, mt)

mt.__eq = nil
assert(a ~= b)

mt.__eq = function(x, y)
  return x[1] == y[1]
end
assert(a == b)

mt.__eq = function(x, y)
  return x[1] ~= y[1]
end
assert(a ~= b)

print("ok")
