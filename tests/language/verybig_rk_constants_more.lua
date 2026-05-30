print("case:verybig_rk_constants_more")

local function foo()
  local dummy = {
    1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16,
    17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32,
    33, 34, 35, 36, 37, 38, 39, 40, 41, 42, 43, 44, 45, 46, 47, 48,
  }
  for i = 49, 270 do
    dummy[#dummy + 1] = i
  end
  assert(dummy[1] == 1 and dummy[48] == 48 and dummy[256] == 256 and dummy[270] == 270)
  assert(24.5 + 0.6 == 25.1)
  local t = {x = 10}
  t.foo = function(self, x) return x + self.x end
  t.t = t
  assert(t:foo(1.5) == 11.5)
  assert(t.t:foo(0.5) == 10.5)
  assert(24.3 == 24.3)
  assert((function() return t.x end)() == 10)
end

foo()

print("ok")
