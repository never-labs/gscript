print("case:sort_unpack_assignment_more")

local unpack = table.unpack
local a = {1,2,3,4,5,6,7,8,9,10}
local x, y, z = unpack(a, 4, 6)
assert(x == 4 and y == 5 and z == 6)
x, y, z = unpack(a, 8, 7)
assert(x == nil and y == nil and z == nil)
x, y = unpack({[20] = "x"}, 20, 20)
assert(x == "x" and y == nil)

print("ok")
