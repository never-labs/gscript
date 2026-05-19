print("case:pm_find_nul_strings")

local z = string.char(0)
local s = "a" .. z .. "o a" .. z .. "o a" .. z .. "o"

local a, b = string.find(s, "a", 1)
assert(a == 1 and b == 1)

a, b = string.find(s, "a" .. z .. "o", 2)
assert(a == 5 and b == 7)

a, b = string.find(s, "a" .. z .. "o", 9)
assert(a == 9 and b == 11)

local e = "a" .. z .. "a" .. z .. "a" .. z .. "a" .. z .. z .. "ab"
a, b = string.find(e, z .. "ab", 2)
assert(a == 9 and b == 11)

a, b = string.find(e, "b")
assert(a == 11 and b == 11)
assert(not string.find(e, "b" .. z))
assert(not string.find("", z))

print("ok")
