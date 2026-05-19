print("case:db_nested_call_flow_more")

local function f(x, name)
  name = name or "f"
  return x, name
end

local g = {}
f(g).x = f(2) and f(10) + f(9)
assert(g.x == f(19))

local function h(x)
  if not x then return 3 end
  return x("a", "x")
end

assert(h(f) == "a")
assert(h(nil) == 3)

local function make()
  local count = 0
  return function(delta)
    count = count + delta
    return count
  end
end

local a = make()
local b = make()
assert(a(2) == 2 and a(3) == 5)
assert(b(7) == 7 and a(1) == 6)

print("ok")
