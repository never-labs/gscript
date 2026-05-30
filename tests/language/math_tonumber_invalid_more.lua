print("case:math_tonumber_invalid_more")

local function f (x)
  return x
end

assert(not f(tonumber("fFfa", 15)))
assert(not f(tonumber("099", 8)))
assert(not f(tonumber("", 8)))
assert(not f(tonumber("  ", 9)))
assert(not f(tonumber("0xf", 10)))
assert(not f(tonumber("inf")))
assert(not f(tonumber(" INF ")))
assert(not f(tonumber("Nan")))
assert(not f(tonumber("nan")))
assert(not f(tonumber("  ")))
assert(not f(tonumber("")))
assert(not f(tonumber("1  a")))
assert(not f(tonumber("1  a", 2)))
assert(not f(tonumber("e1")))
assert(not f(tonumber("e  1")))
assert(not f(tonumber(" 3.4.5 ")))

print("ok")
