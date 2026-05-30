print("case:strings_gmatch_coroutine_more2")

local f = string.gmatch("1 2 3 4 5", "%d+")
assert(f() == "1")
local co = coroutine.wrap(f)
assert(co() == "2")

print("ok")
