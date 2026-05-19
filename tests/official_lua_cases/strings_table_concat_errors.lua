print("case:strings_table_concat_errors")

assert(not pcall(table.concat, 3))
assert(not pcall(table.concat, {"a", "b", {}}))
assert(not pcall(table.concat, {"a", nil, "c"}, ",", 1, 3))
assert(table.concat({"a", 2, "c"}, ",") == "a,2,c")

print("ok")
