print("case:sort_unpack_ranges")

local a = {}
for i = 1, 20 do
  a[i] = i
end

local x = table.unpack(a)
assert(x == 1)

local y, z
x, y = table.unpack(a, 10, 10)
assert(x == 10 and y == nil)

x, y, z = table.unpack(a, 10, 11)
assert(x == 10 and y == 11 and z == nil)

x, y, z = table.unpack(a, 10, 6)
assert(x == nil and y == nil and z == nil)

x, y, z = table.unpack(a, 11, 10)
assert(x == nil and y == nil and z == nil)

local one = {1}
x, y = table.unpack(one, 1, 1)
assert(x == 1 and y == nil)

local two = {1, 2}
x, y = table.unpack(two, 1, 1)
assert(x == 1 and y == nil)

print("ok")
