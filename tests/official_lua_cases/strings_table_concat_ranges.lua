print("case:strings_table_concat_ranges")

assert(table.concat({}) == "")
assert(table.concat({}, "x") == "")

local a = {"a", "b", "c"}
assert(table.concat(a, ",", 1, 0) == "")
assert(table.concat(a, ",", 1, 1) == "a")
assert(table.concat(a, ",", 1, 2) == "a,b")
assert(table.concat(a, ",", 2) == "b,c")
assert(table.concat(a, ",", 3) == "c")
assert(table.concat(a, ",", 4) == "")

a = {}
for i = 1, 5 do
  a[i] = "x"
end
assert(table.concat(a, "123") .. "123" == string.rep("x123", 5))
assert(table.concat(a, "b", 2, 2) == "x")
assert(table.concat(a, "", 2, 3) == "xx")
assert(table.concat(a, "x", 4, 3) == "")

print("ok")
