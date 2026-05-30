print("case:verybig_method_constants_more")

local dummy = {}
for i = 1, 300 do
  dummy[i] = i
end

local t = {foo = function (self, x) return x + self.x end, x = 10}
t.t = t
assert(dummy[1] == 1 and dummy[256] == 256 and dummy[300] == 300)
assert(t:foo(1.5) == 11.5)
assert(t.t:foo(0.5) == 10.5)
assert((function () return t.x + dummy[275] end)() == 285)

print("ok")
