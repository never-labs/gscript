print("case:strings_table_concat_ranges_more3")

local a = {"a", "b", "c"}
assert(table.concat(a, ",", 1, 0) == "")
assert(table.concat(a, ",", 1, 1) == "a")
assert(table.concat(a, ",", 1, 2) == "a,b")
assert(table.concat(a, ",", 2) == "b,c")
assert(table.concat(a, ",", 3) == "c")
assert(table.concat(a, ",", 4) == "")

local b = {}
for i = 1, 40 do b[i] = "xuxu" end
assert(table.concat(b, "123") .. "123" == string.rep("xuxu123", 40))
assert(table.concat(b, "b", 20, 20) == "xuxu")
assert(table.concat(b, "", 20, 21) == "xuxuxuxu")
assert(table.concat(b, "x", 22, 21) == "")
assert(table.concat(b, "3", 39) == "xuxu3xuxu")

print("ok")
