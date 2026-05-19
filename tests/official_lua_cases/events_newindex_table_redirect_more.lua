print("case:events_newindex_table_redirect_more")

local mt = {}
local c = {}
local a = setmetatable({}, mt)
mt.__newindex = c
mt.__index = c

a[1] = 10
a[2] = 20
a[3] = 90
for i = 4, 20 do
  a[i] = i * 10
end

assert(a[1] == 10 and a[2] == 20 and a[3] == 90)
for i = 4, 20 do
  assert(a[i] == i * 10)
end
assert(next(a) == nil)

local mt2 = {}
mt2.__newindex = mt2
local t = setmetatable({}, mt2)
t[1] = 10
assert(mt2[1] == 10)

print("ok")
