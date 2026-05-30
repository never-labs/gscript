print("case:sort_unpack_sparse_boundary_more")

local t = {[1000000000] = "tail"}
local ok, err = pcall(table.unpack, t, 1, 1000000000)
assert(ok == false)
assert(string.find(err, "too many", 1, true) ~= nil)

print("ok")
