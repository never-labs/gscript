print("case:math_random_protocol")

random := math.random

math.randomseed(1007)
raw := random(0)
assert(type(raw) == "number" && raw >= 0 && math.tointeger(raw))

math.randomseed(1007, 0)
f := random()
assert(type(f) == "number" && f >= 0 && f < 1)

a := random(10)
assert(math.tointeger(a) && a >= 1 && a <= 10)

b := random(10, 20)
assert(math.tointeger(b) && b >= 10 && b <= 20)

x, y := math.randomseed()
assert(type(x) == "number" && type(y) == "number")
res := random(0)
x, y = math.randomseed(x, y)
assert(type(x) == "number" && type(y) == "number")
assert(random(0) == res)
math.randomseed(x, y)
assert(random(0) == res)

assert(!pcall(random, 1, 2, 3))
assert(!pcall(random, 10, 1))

print("ok")
