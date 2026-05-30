print("case:events_compare_ordering")

local mt = {}

mt.__lt = function(a, b, c)
  assert(c == nil)
  if type(a) == "table" then a = a.x end
  if type(b) == "table" then b = b.x end
  return a < b, "ignored"
end

mt.__le = function(a, b, c)
  assert(c == nil)
  if type(a) == "table" then a = a.x end
  if type(b) == "table" then b = b.x end
  return a <= b, "ignored"
end

mt.__eq = function(a, b, c)
  assert(c == nil)
  if type(a) == "table" then a = a.x end
  if type(b) == "table" then b = b.x end
  return a == b, "ignored"
end

local function Op(x)
  local r = {}
  r.x = x
  return setmetatable(r, mt)
end

assert(not (Op(1) < Op(1)))
assert(Op(1) < Op(2))
assert(not (Op(2) < Op(1)))
assert(not (1 < Op(1)))
assert(Op(1) < 2)
assert(not (2 < Op(1)))

assert(Op(1) <= Op(1))
assert(Op(1) <= Op(2))
assert(not (Op(2) <= Op(1)))
assert(Op("a") <= Op("a"))
assert(Op("a") <= Op("b"))
assert(not (Op("b") <= Op("a")))

assert(not (Op(1) > Op(1)))
assert(not (Op(1) > Op(2)))
assert(Op(2) > Op(1))
assert(1 >= Op(1))
assert(not (1 >= Op(2)))
assert(Op(2) >= 1)

assert(Op(10) == Op(10))
assert(not (Op(10) == Op(11)))
assert(Op("x") ~= Op("y"))
assert(not (Op("x") ~= Op("x")))

print("ok")
