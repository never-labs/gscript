print("case:locals_scope")

local function f1 (x) x = nil; return x end
assert(f1(10) == nil)

local function f2 () local x; return x end
assert(f2(10) == nil)

local function f3 (x) x = nil; local y; return x, y end
assert(f3(10) == nil and select(2, f3(20)) == nil)

do
  local i = 10
  do local i = 100; assert(i == 100) end
  do local i = 1000; assert(i == 1000) end
  assert(i == 10)
  if i ~= 10 then
    local i = 20
  else
    local i = 30
    assert(i == 30)
  end
end

local x = 1
function f (a)
  local x = 3
  local b = a
  local c, d = a, b
  if d == b then
    local x = "q"
    x = b
    assert(x == 2)
  else
    assert(nil)
  end
  assert(x == 3)
end

local b = 10
local a
repeat local b; a, b = 1, 2; assert(a + 1 == b); until a + b == 3

assert(x == 1)
f(2)
assert(type(f) == "function")

print("ok")
