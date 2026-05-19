print("case:events_arith_compare")

mt := {}
mt.__add = func(a, b) { return a.x + b.x }
mt.__sub = func(a, b) { return a.x - b.x }
mt.__mul = func(a, b) { return a.x * b.x }
mt.__div = func(a, b) { return a.x / b.x }
mt.__mod = func(a, b) { return a.x % b.x }
mt.__pow = func(a, b) { return a.x ** b.x }
mt.__unm = func(a) { return -a.x }

a := setmetatable({x: 8}, mt)
b := setmetatable({x: 2}, mt)

assert(a + b == 10)
assert(a - b == 6)
assert(a * b == 16)
assert(a / b == 4)
assert(a % b == 0)
assert(b ** b == 4)
assert(-b == -2)

print("ok")
