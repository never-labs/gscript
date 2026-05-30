print("case:events_newindex_self_metatable_more")

local t = {}
local function f(t, i, v)
  rawset(t, i, v - 3)
end

setmetatable(t, t)
t.__newindex = f

local a = setmetatable({}, t)
a[1] = 30
a.x = 101
a[5] = 200

assert(a[1] == 27 and a.x == 98 and a[5] == 197)

print("ok")
