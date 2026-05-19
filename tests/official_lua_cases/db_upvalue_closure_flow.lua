print("case:db_upvalue_closure_flow")

local a, b, c = 1, 2, 3

local function foo1(x)
  b = x
  return c
end

local function foo2(x)
  a = x
  return c + b
end

assert(foo1(10) == 3)
assert(foo2(5) == 13)
assert(a == 5 and b == 10 and c == 3)

local function make_counter(step)
  local total = a + b
  return function(delta)
    total = total + step + delta
    return total
  end
end

local c1 = make_counter(2)
local c2 = make_counter(5)
assert(c1(1) == 18)
assert(c1(1) == 21)
assert(c2(0) == 20)
assert(c1(0) == 23)

print("ok")
