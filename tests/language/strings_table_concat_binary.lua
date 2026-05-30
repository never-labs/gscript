print("case:strings_table_concat_binary")

local z = string.char(0)
local z1 = string.char(0, 1)
local z12 = string.char(0, 1, 2)
assert(table.concat({z, z1, z12}, "." .. z .. ".") ==
       z .. "." .. z .. "." .. z1 .. "." .. z .. "." .. z12)

local a = {}
for i = 1, 300 do
  a[i] = "xuxu"
end
assert(table.concat(a, "123") .. "123" == string.rep("xuxu123", 300))
assert(table.concat(a, "b", 20, 20) == "xuxu")
assert(table.concat(a, "", 20, 21) == "xuxuxuxu")
assert(table.concat(a, "x", 22, 21) == "")
assert(table.concat(a, "3", 299) == "xuxu3xuxu")

print("ok")
