print("case:events_index_newindex_call")

local a, t = {10, 20, 30; x = "10", y = "20"}, {}
assert(setmetatable(a, t) == a)
assert(getmetatable(a) == t)
assert(setmetatable(a, nil) == a)
assert(getmetatable(a) == nil)
assert(setmetatable(a, t) == a)

function f (t, i, e)
  assert(not e)
  local p = rawget(t, "parent")
  return (p and p[i] + 3), "dummy return"
end

t.__index = f
a.parent = {z = 25, x = 12, [4] = 24}
assert(a[1] == 10 and a.z == 28 and a[4] == 27 and a.x == "10")

a = setmetatable({}, t)
function f(t, i, v) rawset(t, i, v - 3) end
t.__newindex = f
a[1] = 30; a.x = 101; a[5] = 200
assert(a[1] == 27 and a.x == 98 and a[5] == 197)

local c = {}
a = setmetatable({}, t)
t.__newindex = c
t.__index = c
a[1] = 10; a[2] = 20; a[3] = 90
for i = 4, 20 do a[i] = i * 10 end
assert(a[1] == 10 and a[2] == 20 and a[3] == 90)
for i = 4, 20 do assert(a[i] == i * 10) end
assert(next(a) == nil)

setmetatable(t, nil)
function f (t, ...) return t, {...} end
t.__call = f

local x, y = a(table.unpack{"a", 1})
assert(x == a and y[1] == "a" and y[2] == 1 and y[3] == nil)
x, y = a()
assert(x == a and y[1] == nil)

print("ok")
