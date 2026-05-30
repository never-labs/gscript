print("case:sort_table_insert_remove_concat")

a := {1, 2, 3}
table.insert(a, 4)
assert(#a == 4 && a[1] == 1 && a[4] == 4)
table.insert(a, 2, 10)
assert(#a == 5 && a[1] == 1 && a[2] == 10 && a[3] == 2 && a[5] == 4)
assert(table.remove(a, 2) == 10)
assert(#a == 4 && a[1] == 1 && a[2] == 2 && a[3] == 3 && a[4] == 4)
assert(table.remove(a) == 4)
assert(#a == 3 && a[3] == 3)

words := {"a", "b", "c"}
assert(table.concat(words, ",") == "a,b,c")
assert(table.concat(words, ",", 2, 3) == "b,c")
empty := {}
assert(table.concat(empty, ",") == "")

print("ok")
