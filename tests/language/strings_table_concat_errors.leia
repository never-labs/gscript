print("case:strings_table_concat_errors")

assert(!pcall(table.concat, 3))
bad := {}
assert(!pcall(table.concat, {"a", "b", bad}))
assert(!pcall(table.concat, {"a", nil, "c"}, ",", 1, 3))
mixed := {"a", 2, "c"}
assert(table.concat(mixed, ",") == "a,2,c")

print("ok")
