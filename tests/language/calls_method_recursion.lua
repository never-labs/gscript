print("case:calls_method_recursion")

local fact = false
do
  local res = 1
  local function fact (n)
    if n == 0 then return res
    else return n * fact(n - 1)
    end
  end
  assert(fact(5) == 120)
end
assert(fact == false)

local a = {i = 10}
local self = 20
function a:x (x) return x + self.i end
function a.y (x) return x + self end
assert(a:x(1) + 10 == a.y(1))

a.t = {i = -100}
a["t"].x = function (self, a, b) return self.i + a + b end
assert(a.t:x(2, 3) == -95)

do
  local a = {x = 0}
  function a:add (x) self.x, a.y = self.x + x, 20; return self end
  assert(a:add(10):add(20):add(30).x == 60 and a.y == 20)
end

print("ok")
