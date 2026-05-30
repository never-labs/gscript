print("case:sort_table_insert_remove_concat")

local a = {1, 2, 3}
table.insert(a, 4)
assert(#a == 4 and a[1] == 1 and a[4] == 4)
table.insert(a, 2, 10)
assert(#a == 5 and a[1] == 1 and a[2] == 10 and a[3] == 2 and a[5] == 4)
assert(table.remove(a, 2) == 10)
assert(#a == 4 and a[1] == 1 and a[2] == 2 and a[3] == 3 and a[4] == 4)
assert(table.remove(a) == 4)
assert(#a == 3 and a[3] == 3)

assert(table.concat({"a", "b", "c"}, ",") == "a,b,c")
assert(table.concat({"a", "b", "c"}, ",", 2, 3) == "b,c")
assert(table.concat({}, ",") == "")

print("ok")
