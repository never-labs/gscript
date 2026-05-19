print("case:locals_shadowing_repeat_more")

local function f (a)
  local _1, _2, _3, _4, _5
  local _6, _7, _8, _9, _10
  local x = 3
  local b = a
  local c,d = a,b
  if (d == b) then
    local x = 'q'
    x = b
    assert(x == 2)
  else
    assert(nil)
  end
  assert(x == 3)
  local f = 10
  assert(f == 10 and c == 2)
end

local b=10
local a; repeat local b; a,b=1,2; assert(a+1==b); until a+b==3

local x = 1
assert(x == 1)
f(2)
assert(type(f) == 'function')

print("ok")
