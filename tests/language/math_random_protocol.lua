print("case:math_random_protocol")

local random = math.random

math.randomseed(1007)
local raw = random(0)
assert(type(raw) == "number" and raw >= 0 and math.tointeger(raw))

math.randomseed(1007, 0)
local f = random()
assert(type(f) == "number" and f >= 0 and f < 1)

local a = random(10)
assert(math.tointeger(a) and a >= 1 and a <= 10)

local b = random(10, 20)
assert(math.tointeger(b) and b >= 10 and b <= 20)

local x, y = math.randomseed()
assert(type(x) == "number" and type(y) == "number")
local res = random(0)
x, y = math.randomseed(x, y)
assert(type(x) == "number" and type(y) == "number")
assert(random(0) == res)
math.randomseed(x, y)
assert(random(0) == res)

assert(not pcall(random, 1, 2, 3))
assert(not pcall(random, 10, 1))

print("ok")
