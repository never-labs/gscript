print("case:events_compare_metamethods")

local cap = {}

local function lt(a, b)
  cap[#cap + 1] = {"lt", a, b}
  return true
end

local function le(a, b)
  cap[#cap + 1] = {"le", a, b}
  return true
end

local function eq(a, b)
  cap[#cap + 1] = {"eq", a, b}
  return true
end

local mt = {}
mt.__lt = lt
mt.__le = le
mt.__eq = eq

local a = {}
setmetatable(a, mt)
local b = {}
setmetatable(b, mt)

assert(5.0 > a)
local last = cap[#cap]
assert(last[1] == "lt" and last[2] == a and last[3] == 5.0)

assert(a >= 10)
last = cap[#cap]
assert(last[1] == "le" and last[2] == 10 and last[3] == a)

assert(a <= -10.0)
last = cap[#cap]
assert(last[1] == "le" and last[2] == a and last[3] == -10.0)

assert(a < -10)
last = cap[#cap]
assert(last[1] == "lt" and last[2] == a and last[3] == -10)

assert(a == b)
last = cap[#cap]
assert(last[1] == "eq" and last[2] == a and last[3] == b)

assert(not (a ~= b))
last = cap[#cap]
assert(last[1] == "eq" and last[2] == a and last[3] == b)

print("ok")
