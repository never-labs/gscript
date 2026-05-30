print("case:calls_multireturn")

assert(type(1 < 2) == "boolean")
assert(type(nil) == "nil")
assert(type(-3) == "number")
assert(type("x") == "string")
assert(type({}) == "table")
assert(type(type) == "function")

local function pack4(a, b, c)
  local d = "sentinel"
  return a, b, c, d
end

local a, b, c, d = pack4(1, 2)
assert(a == 1 and b == 2 and c == nil and d == "sentinel")

a, b, c, d = pack4(1, 2, 3, 4)
assert(a == 1 and b == 2 and c == 3 and d == "sentinel")

local function tail(x)
  if x <= 0 then return 101 end
  return tail(x - 1)
end
assert(tail(200) == 101)

local function chain(x)
  return x, x + 1, x + 2
end
local x, y, z = chain(-2)
assert(x == -2 and y == -1 and z == 0)

print("ok")
