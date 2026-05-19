print("case:locals_basic_more")

local function f1(x) x = nil; return x end
assert(f1(10) == nil)
local function f2() local x; return x end
assert(f2(10) == nil)
local function f3(x) x = nil; local y; return x, y end
assert(f3(10) == nil and select(2, f3(20)) == nil)

local function scope()
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
scope()

print("ok")
