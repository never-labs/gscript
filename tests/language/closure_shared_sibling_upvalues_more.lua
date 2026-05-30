print("case:closure_shared_sibling_upvalues_more")

local foo1, foo2, foo3
do
  local a, b, c = 3, 5, 7
  foo1 = function () return a + b end
  foo2 = function () b = b + 1; return b + a end
  do
    local a = 10
    foo3 = function () return a + b end
  end
  assert(c == 7)
end

assert(foo1() == 8)
assert(foo2() == 9)
assert(foo1() == 9)
assert(foo3() == 16)

print("ok")
