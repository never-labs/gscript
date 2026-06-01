print("case:nextvar_table_insert_string_more")

a := {"c", "d"}
table.insert(a, 3, "a")
table.insert(a, "b")
assert(table.remove(a, 1) == "c")
assert(table.remove(a, 1) == "d")
assert(table.remove(a, 1) == "a")
assert(table.remove(a, 1) == "b")
assert(table.remove(a, 1) == nil)
assert(#a == 0 && a.n == nil)

print("ok")
