print("case:events_arith_compare")

local mt = {}
mt.__add = function (a, b) return a.x + b.x end
mt.__sub = function (a, b) return a.x - b.x end
mt.__mul = function (a, b) return a.x * b.x end
mt.__div = function (a, b) return a.x / b.x end
mt.__mod = function (a, b) return a.x % b.x end
mt.__pow = function (a, b) return a.x ^ b.x end
mt.__unm = function (a) return -a.x end

local a = setmetatable({x = 8}, mt)
local b = setmetatable({x = 2}, mt)

assert(a + b == 10)
assert(a - b == 6)
assert(a * b == 16)
assert(a / b == 4)
assert(a % b == 0)
assert(b ^ b == 4)
assert(-b == -2)

print("ok")
