print("case:calls_anonymous_invocation")

local a = nil
(function (x) a = x end)(23)
assert(a == 23 and (function (x) return x * 2 end)(20) == 40)

local f = function (x, y)
  return x + y, x - y
end

local p, q = f(9, 4)
assert(p == 13 and q == 5)

print("ok")
